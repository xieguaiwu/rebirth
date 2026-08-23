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
			c.HTTP = http.DefaultClient
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
// every method fails soft back to deterministic text.
type Narrator struct {
	C        *Client
	calls    int
	MaxCalls int
}

// DefaultCallBudget caps LLM invocations for one simulated life.
const DefaultCallBudget = 10

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

// Narrate rewrites one event line vividly. Falls back to raw sanitized text
// when the model skips the JSON envelope, then to fallback on failure.
func (n *Narrator) Narrate(age int, summary, fallback string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if !n.budget() {
		return fallback
	}
	out, err := n.C.complete(ctx,
		"你是人生模拟器的叙事者。把给定事件改写成一句生动中文叙述，不超过40字。只输出JSON：{\"text\":\"...\"}",
		fmt.Sprintf("年龄%d。状态：%s。原文：%s", age, summary, fallback), 300)
	if err != nil {
		return fallback
	}
	var r struct {
		Text string `json:"text"`
	}
	if extractJSON(out, &r) && len([]rune(strings.TrimSpace(r.Text))) >= 2 {
		return truncate(sanitizeLine(r.Text), 60)
	}
	// Plain-text models: use the response itself only if it looks like prose
	// (no JSON punctuation anywhere).
	if !strings.ContainsAny(out, "{}") {
		if line := sanitizeLine(out); len([]rune(line)) >= 2 {
			return truncate(line, 60)
		}
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
	defer cancel()
	if !n.budget() {
		return game.Event{}, false
	}
	out, err := n.C.complete(ctx,
		`你是命运编织者，为人生模拟器发明一个独特事件。贴合给定状态，避免俗套。
只输出JSON，字段：{"text":"事件一句话","good":true/false,"chr":-3..3,"int":-3..3,"str":-3..3,"mny":-3..3,"spr":-3..3,"trauma_alpha":0..0.5}
trauma_alpha 表示该事件的创伤强度（0=无创伤）。`,
		fmt.Sprintf("年龄%d。状态：%s", age, summary), 500)
	if err != nil {
		return game.Event{}, false
	}
	var p fatePayload
	if !extractJSON(out, &p) {
		return game.Event{}, false
	}
	text := strings.TrimSpace(p.Text)
	if len([]rune(text)) < 4 {
		return game.Event{}, false
	}
	cl := func(v float64) float64 { return math.Max(-3, math.Min(3, v)) }
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

// Epitaph writes the closing line of a finished life.
func (n *Narrator) Epitaph(summary string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if !n.budget() {
		return "一生至此。"
	}
	out, err := n.C.complete(ctx,
		"你是墓志铭作者。为这段人生写一句不超过30字的墓志铭，克制而有余味。只输出JSON：{\"text\":\"...\"}",
		summary, 200)
	if err != nil {
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
	return s
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
