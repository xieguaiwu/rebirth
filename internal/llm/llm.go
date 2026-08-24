// Package llm provides an OpenRouter chat-completions client and a
// fail-soft narrator for the rebirth game. The model adds narrative color
// and injects unique "fate events"; the deterministic core keeps all RNG.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"rebirth/internal/game"
)

const defaultBaseURL = "https://openrouter.ai/api/v1"

// Provider is one named LLM endpoint preset: base URL, default model and
// the environment variable holding the API key. Providers are OpenAI-
// compatible chat endpoints; pick with --provider and override pieces
// with --model / --llm-url.
type Provider struct {
	Name         string
	BaseURL      string
	DefaultModel string
	KeyEnv       string
}

// Providers is the preset registry. Unknown names fall back to openrouter
// (ResolveProvider reports ok=false and main warns).
var Providers = map[string]Provider{
	"openrouter": {
		Name:         "openrouter",
		BaseURL:      defaultBaseURL,
		DefaultModel: "stealth/ox-alpha",
		KeyEnv:       "OPENROUTER_API_KEY",
	},
	"deepseek": {
		Name:         "deepseek",
		BaseURL:      "https://api.deepseek.com/v1",
		DefaultModel: "deepseek-v4-flash",
		KeyEnv:       "DEEPSEEK_API_KEY",
	},
}

// ResolveProvider looks up a preset by name.
func ResolveProvider(name string) (Provider, bool) {
	p, ok := Providers[name]
	return p, ok
}

// Client talks to an OpenAI-compatible chat endpoint (OpenRouter).
type Client struct {
	BaseURL string
	Model   string
	APIKey  string
	HTTP    *http.Client
}

// New builds a client. key comes from OPENROUTER_API_KEY in main; empty key
// callers should use game.Noop instead of this client.
func New(key, model string) *Client {
	return &Client{
		BaseURL: defaultBaseURL,
		Model:   model,
		APIKey:  key,
		HTTP:    &http.Client{Timeout: 45 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// complete sends one chat request and returns the assistant text.
// max_tokens keeps headroom for reasoning models (DeepSeek V4 spends
// budget on reasoning_content before content; lesson: nano-omni empty
// content at max_tokens=300).
func (c *Client) complete(ctx context.Context, system, user string, maxTokens int) (string, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return "", errors.New("llm: empty api key")
	}
	body, err := json.Marshal(chatRequest{
		Model:       c.Model,
		Messages:    []chatMessage{{Role: "system", Content: system}, {Role: "user", Content: user}},
		Temperature: 0.9,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := func() (*http.Response, error) {
		if c.HTTP == nil {
			// Never DefaultClient: it has no timeout at all (v0.8.0
			// hardening — a stalled dial/proxy must never outlive
			// the per-call context).
			c.HTTP = &http.Client{Timeout: 45 * time.Second}
		}
		return c.HTTP.Do(req)
	}()
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm status %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("llm decode: %w", err)
	}
	if len(cr.Choices) == 0 || cr.Choices[0].Message.Content == "" {
		return "", errors.New("llm: empty completion")
	}
	return cr.Choices[0].Message.Content, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Narrator implements game.Narrator on top of the client. A per-run call
// budget keeps long lives from turning into API marathons; once exhausted,
// every narration method fails soft back to deterministic text. The
// epitaph is exempt — one call per life, and it must never be starved by
// earlier narration (v0.7.4).
//
// A failure breaker (v0.8.0) complements the budget: after failLimit
// CONSECUTIVE failures the channel is declared broken for the rest of the
// life and every method returns its fallback instantly. Without it, a
// dead-but-slow endpoint (stealth/ox-alpha measured 15–40s > the 12/18s
// timeouts) froze the game for the full timeout before every single
// fallback — once per sampled event, all life long.
type Narrator struct {
	C        *Client
	calls    int
	MaxCalls int
	fails    int  // consecutive failures; any success resets
	dead     bool // breaker open: fail soft without touching the network
}

// failLimit is how many consecutive failures trip the breaker.
const failLimit = 3

// DefaultCallBudget caps LLM narration+fate calls for one simulated life.
// 24 = ~8 fate events (every decade to 90) + ~16 narrated events; the
// narrate path additionally samples by event ID (game.NarrateRatio).
const DefaultCallBudget = 24

// NewNarrator wraps a client; pass game.Noop when key is missing.
func NewNarrator(c *Client) *Narrator { return &Narrator{C: c, MaxCalls: DefaultCallBudget} }

// budget reports whether one more call is allowed and reserves it.
func (n *Narrator) budget() bool {
	if n.C == nil || n.calls >= n.MaxCalls {
		return false
	}
	n.calls++
	return true
}

// fail records one failure and opens the breaker at the limit.
func (n *Narrator) fail() {
	n.fails++
	if n.fails >= failLimit {
		n.dead = true
	}
}

// ok records a success, resetting the breaker streak.
func (n *Narrator) ok() { n.fails = 0 }

// Broken reports whether the failure breaker has tripped; game.Run prints
// a one-time notice so the player knows why narration went silent.
func (n *Narrator) Broken() bool { return n.dead }

// Narrate rewrites one event line vividly. Falls back to raw sanitized text
// when the model skips the JSON envelope, then to fallback on failure.
func (n *Narrator) Narrate(age int, summary, fallback string) string {
	if n.dead {
		return fallback
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if !n.budget() {
		return fallback
	}
	out, err := n.C.complete(ctx,
		"你是人生模拟器的叙事者。把给定事件改写成一句生动中文叙述，不超过40字。只输出JSON：{\"text\":\"...\"}",
		fmt.Sprintf("年龄%d。状态：%s。原文：%s", age, summary, fallback), 600)
	if err != nil {
		n.fail()
		return fallback
	}
	var r struct {
		Text string `json:"text"`
	}
	if extractJSON(out, &r) && len([]rune(strings.TrimSpace(r.Text))) >= 2 {
		n.ok()
		return truncate(sanitizeLine(r.Text), 60)
	}
	// Plain-text models: use the response itself only if it looks like prose
	// (no JSON punctuation anywhere).
	if !strings.ContainsAny(out, "{}") {
		if line := sanitizeLine(out); len([]rune(line)) >= 2 {
			n.ok()
			return truncate(line, 60)
		}
	}
	n.fail()
	return fallback
}

// fatePayload is the schema-constrained shape requested from the model.
type fatePayload struct {
	Text        string  `json:"text"`
	Good        bool    `json:"good"`
	DCHR        float64 `json:"chr"`
	DINT        float64 `json:"int"`
	DSTR        float64 `json:"str"`
	DMNY        float64 `json:"mny"`
	DSPR        float64 `json:"spr"`
	TraumaAlpha float64 `json:"trauma_alpha"`
}

// FateEvent asks the model to invent one unique event fitting current state.
// Every numeric field is clamped; any schema violation returns ok=false so
// the deterministic pool takes over — hallucinated structure never leaks in.
func (n *Narrator) FateEvent(age int, summary string) (game.Event, bool) {
	if n.dead {
		return game.Event{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
	defer cancel()
	if !n.budget() {
		return game.Event{}, false
	}
	out, err := n.C.complete(ctx,
		`你是命运编织者，为人生模拟器发明一个独特事件。贴合给定状态，避免俗套。
只输出JSON，字段：{"text":"事件一句话","good":true/false,"chr":-3..3,"int":-3..3,"str":-3..3,"mny":-3..3,"spr":-3..3,"trauma_alpha":0..0.5}
trauma_alpha 表示该事件的创伤强度（0=无创伤）。`,
		fmt.Sprintf("年龄%d。状态：%s", age, summary), 900)
	if err != nil {
		n.fail()
		return game.Event{}, false
	}
	var p fatePayload
	if !extractJSON(out, &p) {
		n.fail()
		return game.Event{}, false
	}
	text := strings.TrimSpace(stripControl(p.Text))
	if len([]rune(text)) < 4 {
		n.fail()
		return game.Event{}, false
	}
	cl := func(v float64) float64 { return math.Max(-3, math.Min(3, v)) }
	n.ok()
	ev := game.Event{
		ID:           fmt.Sprintf("llm_fate_%d_%d", age, time.Now().UnixMilli()%100000),
		Text:         truncate(text, 80),
		MinAge:       age,
		MaxAge:       age,
		Weight:       1,
		Good:         p.Good,
		Delta:        &game.Effects{CHR: cl(p.DCHR), INT: cl(p.DINT), STR: cl(p.DSTR), MNY: cl(p.DMNY), SPR: cl(p.DSPR)},
		TraumaAlpha:  math.Max(0, math.Min(0.5, p.TraumaAlpha)),
		LLMGenerated: true,
	}
	return ev, true
}

// Epitaph writes the closing line of a finished life. It is exempt from
// the shared budget: one call per life, guaranteed (see Narrator).
func (n *Narrator) Epitaph(summary string) string {
	if n.dead {
		return "一生至此。"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if n.C == nil {
		return "一生至此。"
	}
	n.calls++ // still counted for observability, never blocks
	out, err := n.C.complete(ctx,
		"你是墓志铭作者。为这段人生写一句不超过30字的墓志铭，克制而有余味。只输出JSON：{\"text\":\"...\"}",
		summary, 400)
	if err != nil {
		n.fail()
		return "一生至此。"
	}
	var r struct {
		Text string `json:"text"`
	}
	if extractJSON(out, &r) && len([]rune(strings.TrimSpace(r.Text))) >= 2 {
		return truncate(sanitizeLine(r.Text), 45)
	}
	if !strings.ContainsAny(out, "{}") {
		if line := sanitizeLine(out); len([]rune(line)) >= 2 {
			return truncate(line, 45)
		}
	}
	return "一生至此。"
}

// sanitizeLine strips JSON punctuation, quotes and newlines from a model
// reply so plain-prose responses remain usable.
func sanitizeLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	s = strings.Trim(s, "{\"}`* \t")
	s = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s), "text"), ":")
	return stripControl(s)
}

// stripControl removes all C0/C1 control bytes (ESC, BEL and friends) from
// model output. Without this a prompt-injected reply could execute OSC
// sequences on the player's terminal (momus P1-3).
func stripControl(s string) string {
	if !strings.ContainsFunc(s, func(r rune) bool {
		return r < 0x20 || (r >= 0x7f && r < 0xa1)
	}) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 0x20 && !(r >= 0x7f && r < 0xa1) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// extractJSON pulls the first JSON object out of a possibly chatty response.
func extractJSON(s string, v any) bool {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end <= start {
		return false
	}
	return json.Unmarshal([]byte(s[start:end+1]), v) == nil
}
