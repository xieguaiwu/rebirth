// Command mobile runs the rebirth simulation core as a JSON-lines daemon
// for the Android app. The daemon is platform-neutral: it builds for any
// GOOS and can be driven from a terminal or CI for golden tests.
//
// Protocol contract: docs/mobile-protocol.md (frozen). Every command is a
// single JSON object per line on stdin; every response is a single JSON
// object per line on stdout. stderr carries logs only — API keys and full
// request bodies never appear in logs.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"rebirth/internal/game"
	"rebirth/internal/llm"
)

const version = "0.10.0"

// ---- protocol wire types ----

type request struct {
	ID              int64               `json:"id"`
	Cmd             string              `json:"cmd"`
	Seed            *int64              `json:"seed"`
	Lang            string              `json:"lang"`
	Count           int                 `json:"count"`
	Birth           *game.Birth         `json:"birth"`
	Talents         []game.Talent       `json:"talents"`
	Points          *pointsReq          `json:"points"`
	MaxAge          int                 `json:"max_age"`
	Narrator        *narratorReq        `json:"narrator"`
	TraumaOverrides *traumaOverridesReq `json:"trauma_overrides"`
}

type pointsReq struct {
	CHR float64 `json:"chr"`
	INT float64 `json:"int"`
	STR float64 `json:"str"`
	MNY float64 `json:"mny"`
}

type narratorReq struct {
	Enabled   bool          `json:"enabled"`
	Providers []providerReq `json:"providers"`
	Budget    int           `json:"budget"`
	Ratio     float64       `json:"ratio"`
}

type providerReq struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	URL      string `json:"url"`
	Key      string `json:"key"`
}

type traumaOverridesReq struct {
	EnterAt          *float64 `json:"enter_at"`
	ExitAt           *float64 `json:"exit_at"`
	Drive            *float64 `json:"drive"`
	EventTraumaScale *float64 `json:"event_trauma_scale"`
}

type response struct {
	ID    int64  `json:"id"`
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// ---- structured year output ----

type statsDTO struct {
	CHR float64 `json:"chr"`
	INT float64 `json:"int"`
	STR float64 `json:"str"`
	MNY float64 `json:"mny"`
	SPR float64 `json:"spr"`
}

type traumaDTO struct {
	M            float64 `json:"m"`
	A            float64 `json:"a"`
	P            float64 `json:"p"`
	Load         float64 `json:"load"`
	Pathological bool    `json:"pathological"`
}

type careerDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type eventDTO struct {
	ID          string        `json:"id"`
	Text        string        `json:"text"`
	Good        bool          `json:"good"`
	LLM         bool          `json:"llm"`
	TraumaAlpha float64       `json:"trauma_alpha"`
	TherapyQ    float64       `json:"therapy_q"`
	Delta       *game.Effects `json:"delta,omitempty"`
}

type yearDTO struct {
	Age             int        `json:"age"`
	Lines           []string   `json:"lines"`
	Career          *careerDTO `json:"career,omitempty"`
	CareerChange    string     `json:"career_change,omitempty"`
	Event           *eventDTO  `json:"event,omitempty"`
	Stats           statsDTO   `json:"stats"`
	Trauma          traumaDTO  `json:"trauma"`
	Luck            float64    `json:"luck"`
	LLMBrokenNotice bool       `json:"llm_broken_notice"`
	Died            bool       `json:"died"`
	DeathStatus     string     `json:"death_status,omitempty"`
	Epitaph         string     `json:"epitaph,omitempty"`
	LineageSaved    bool       `json:"lineage_saved,omitempty"`
	NextGeneration  int        `json:"next_generation,omitempty"`
	NextSensitivity float64    `json:"next_sensitivity,omitempty"`
}

// ---- LLM output cache (checkpoint replay) ----

type llmCache struct {
	Ages map[int]*ageCacheEntry `json:"ages"`
}

type ageCacheEntry struct {
	Fate             *game.Event `json:"fate,omitempty"`
	FateOK           bool        `json:"fate_ok"`
	FateConsulted    bool        `json:"fate_consulted"`
	NarrateText      string      `json:"narrate_text,omitempty"`
	NarrateUsed      bool        `json:"narrate_used"`
	NarrateConsulted bool        `json:"narrate_consulted"`
}

// recordingNarrator wraps the real narrator (or chain) and keeps a
// per-age record of every LLM interaction. On replay the cache serves the
// recorded outputs so a resumed life is bit-identical for already-played
// years; years beyond the checkpoint call the real narrator and extend the
// cache. API keys never enter the cache (the wrapped narrators hold them).
type recordingNarrator struct {
	inner game.Narrator
	cache *llmCache
}

func (r *recordingNarrator) entry(age int) *ageCacheEntry {
	if r.cache.Ages == nil {
		r.cache.Ages = map[int]*ageCacheEntry{}
	}
	e, ok := r.cache.Ages[age]
	if !ok {
		e = &ageCacheEntry{}
		r.cache.Ages[age] = e
	}
	return e
}

func (r *recordingNarrator) Narrate(age int, summary, fallback string) string {
	if e, ok := r.cache.Ages[age]; ok && e.NarrateConsulted {
		if e.NarrateUsed {
			return e.NarrateText
		}
		return fallback
	}
	text := r.inner.Narrate(age, summary, fallback)
	e := r.entry(age)
	e.NarrateConsulted = true
	if text != fallback {
		e.NarrateUsed = true
		e.NarrateText = text
	}
	return text
}

func (r *recordingNarrator) FateEvent(age int, summary string) (game.Event, bool) {
	if e, ok := r.cache.Ages[age]; ok && e.FateConsulted {
		if e.FateOK && e.Fate != nil {
			return *e.Fate, true
		}
		return game.Event{}, false
	}
	ev, ok := r.inner.FateEvent(age, summary)
	e := r.entry(age)
	e.FateConsulted = true
	if ok {
		fate := ev
		e.Fate = &fate
		e.FateOK = true
	}
	return ev, ok
}

func (r *recordingNarrator) Epitaph(summary string) string { return r.inner.Epitaph(summary) }
func (r *recordingNarrator) Broken() bool                  { return r.inner.Broken() }

// ---- checkpoint ----

type checkpoint struct {
	Seed         int64              `json:"seed"`
	Lang         string             `json:"lang"`
	Birth        *game.Birth        `json:"birth,omitempty"`
	Bloodline    *game.Bloodline    `json:"bloodline,omitempty"`
	Talents      []game.Talent      `json:"talents"`
	InheritTal   string             `json:"inherit_tal,omitempty"`
	Points       []float64          `json:"points"`
	MaxAge       int                `json:"max_age"`
	NarrateRatio float64            `json:"narrate_ratio"`
	Trauma       *game.TraumaParams `json:"trauma,omitempty"`
	Age          int                `json:"age"`
	LLMCache     llmCache           `json:"llm_cache"`
}

// ---- daemon ----

type daemon struct {
	dir string

	eventsZh  []game.Event
	careersZh []*game.Career
	birthsZh  []game.Birth
	talentsZh []game.Talent
	eventsEn  []game.Event
	careersEn []*game.Career
	birthsEn  []game.Birth
	talentsEn []game.Talent

	sess *game.Session
	lang string
}

func newDaemon(dir string) (*daemon, error) {
	d := &daemon{dir: dir}
	var err error
	if d.eventsZh, err = game.LoadEventsLang("zh"); err != nil {
		return nil, err
	}
	if d.careersZh, err = game.LoadCareersLang("zh"); err != nil {
		return nil, err
	}
	if d.birthsZh, err = game.LoadBirthsLang("zh"); err != nil {
		return nil, err
	}
	if d.talentsZh, err = game.LoadTalentsLang("zh"); err != nil {
		return nil, err
	}
	if d.eventsEn, err = game.LoadEventsLang("en"); err != nil {
		return nil, err
	}
	if d.careersEn, err = game.LoadCareersLang("en"); err != nil {
		return nil, err
	}
	if d.birthsEn, err = game.LoadBirthsLang("en"); err != nil {
		return nil, err
	}
	if d.talentsEn, err = game.LoadTalentsLang("en"); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *daemon) events(lang string) []game.Event {
	if lang == "en" {
		return d.eventsEn
	}
	return d.eventsZh
}

func (d *daemon) careers(lang string) []*game.Career {
	if lang == "en" {
		return d.careersEn
	}
	return d.careersZh
}

func (d *daemon) births(lang string) []game.Birth {
	if lang == "en" {
		return d.birthsEn
	}
	return d.birthsZh
}

func (d *daemon) talents(lang string) []game.Talent {
	if lang == "en" {
		return d.talentsEn
	}
	return d.talentsZh
}

func (d *daemon) bloodlinePath() string { return filepath.Join(d.dir, "bloodline.json") }
func (d *daemon) sessionPath() string   { return filepath.Join(d.dir, "session.json") }

// buildNarrator turns the protocol narrator config into a game.Narrator.
// Providers are tried in order (failover); empty keys are skipped; an
// empty provider list or disabled narrator yields game.Noop.
func (d *daemon) buildNarrator(n *narratorReq, lang string) game.Narrator {
	if n == nil || !n.Enabled || len(n.Providers) == 0 {
		return game.Noop
	}
	var clients []*llm.Client
	for _, p := range n.Providers {
		if strings.TrimSpace(p.Key) == "" {
			continue
		}
		base, model := p.URL, p.Model
		if preset, ok := llm.ResolveProvider(p.Provider); ok {
			if base == "" {
				base = preset.BaseURL
			}
			if model == "" {
				model = preset.DefaultModel
			}
		} else if base == "" {
			// Unknown provider with no URL cannot be reached.
			continue
		}
		c := llm.New(p.Key, model)
		if base != "" {
			c.BaseURL = base
		}
		clients = append(clients, c)
	}
	if len(clients) == 0 {
		return game.Noop
	}
	chain := llm.NewChain(clients, n.Budget)
	chain.Lang = lang
	return chain
}

func (d *daemon) traumaParams(o *traumaOverridesReq) *game.TraumaParams {
	if o == nil {
		return nil
	}
	tp := game.DefaultTraumaParams()
	if o.EnterAt != nil {
		tp.EnterAt = game.Clamp01(*o.EnterAt)
	}
	if o.ExitAt != nil {
		tp.ExitAt = game.Clamp01(*o.ExitAt)
	}
	if o.Drive != nil {
		tp.Drive = game.Clamp01(*o.Drive)
	}
	if o.EventTraumaScale != nil {
		tp.EventScale = game.Clamp01(*o.EventTraumaScale)
	}
	return &tp
}

func (d *daemon) langOf(reqLang string) string {
	if reqLang == "en" {
		return "en"
	}
	return "zh"
}

// handle dispatches one protocol command.
func (d *daemon) handle(req request) (data any, err error) {
	switch req.Cmd {
	case "hello":
		return map[string]any{"ver": version, "proto": 1}, nil

	case "bloodline_get":
		b, err := game.LoadBloodline(d.bloodlinePath())
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"generation":       b.Generation,
			"sensitivity":      b.Sensitivity,
			"inherited_talent": b.InheritedTal,
		}, nil

	case "draw_births":
		if req.Seed == nil {
			return nil, fmt.Errorf("seed required")
		}
		rng := rand.New(rand.NewSource(*req.Seed))
		return map[string]any{"births": game.DrawBirths(d.births(d.langOf(req.Lang)), 3, rng)}, nil

	case "draw_talents":
		if req.Seed == nil {
			return nil, fmt.Errorf("seed required")
		}
		rng := rand.New(rand.NewSource(*req.Seed))
		return map[string]any{"talents": game.DrawTalents(d.talents(d.langOf(req.Lang)), 10, rng)}, nil

	case "new_session":
		return d.newSession(req)

	case "next":
		return d.next()

	case "checkpoint_get":
		cp, err := d.loadCheckpoint()
		if err != nil {
			return map[string]any{"exists": false}, nil
		}
		gen := 1
		if cp.Bloodline != nil {
			gen = cp.Bloodline.Generation
		}
		return map[string]any{"exists": true, "age": cp.Age, "generation": gen}, nil

	case "resume_session":
		return d.resumeSession(req)

	case "shutdown":
		return map[string]any{"bye": true}, nil

	default:
		return nil, fmt.Errorf("unknown command %q", req.Cmd)
	}
}

func (d *daemon) newSession(req request) (any, error) {
	if req.Seed == nil {
		return nil, fmt.Errorf("seed required")
	}
	if req.Points == nil {
		return nil, fmt.Errorf("points required")
	}
	lang := d.langOf(req.Lang)
	bloodline, err := game.LoadBloodline(d.bloodlinePath())
	if err != nil {
		return nil, fmt.Errorf("bloodline: %w", err)
	}
	curGen := bloodline.Generation + 1

	// Resolve the inherited talent by name in the session language; a
	// cross-language mismatch resolves to nil (no inherited talent).
	var inherit *game.Talent
	if bloodline.InheritedTal != "" {
		for i := range d.talents(lang) {
			if d.talents(lang)[i].Name == bloodline.InheritedTal {
				t := d.talents(lang)[i]
				inherit = &t
				break
			}
		}
	}

	ratio := 0.5
	if req.Narrator != nil && req.Narrator.Ratio > 0 {
		ratio = req.Narrator.Ratio
	}
	maxAge := req.MaxAge
	if maxAge <= 0 {
		maxAge = 100
	}
	narrator := d.buildNarrator(req.Narrator, lang)
	rec := &recordingNarrator{inner: narrator, cache: &llmCache{}}

	cfg := game.Config{
		Seed:         *req.Seed,
		Birth:        req.Birth,
		Bloodline:    &game.Bloodline{Generation: curGen, Sensitivity: bloodline.Sensitivity, InheritedTal: bloodline.InheritedTal},
		Talents:      req.Talents,
		InheritTal:   inherit,
		LLM:          rec,
		MaxAge:       maxAge,
		Trauma:       d.traumaParams(req.TraumaOverrides),
		NarrateRatio: ratio,
	}.WithPoints(req.Points.CHR, req.Points.INT, req.Points.STR, req.Points.MNY)

	d.sess = game.NewSession(cfg, d.events(lang), d.careers(lang))
	d.lang = lang
	// A fresh life invalidates any stale checkpoint.
	_ = os.Remove(d.sessionPath())
	return map[string]any{"generation": curGen}, nil
}

func (d *daemon) next() (any, error) {
	if d.sess == nil {
		return nil, fmt.Errorf("no session")
	}
	info := d.sess.Advance()
	died := d.sess.DeathCheck()

	out := &yearDTO{
		Age:             info.Age,
		Lines:           info.Lines,
		CareerChange:    info.CareerChange,
		Stats:           statsDTO{CHR: info.Stats.CHR, INT: info.Stats.INT, STR: info.Stats.STR, MNY: info.Stats.MNY, SPR: info.Stats.SPR},
		Trauma:          traumaDTO{M: info.TraumaM, A: info.TraumaA, P: info.TraumaP, Load: info.TraumaLoad, Pathological: info.Pathological},
		Luck:            info.Luck,
		LLMBrokenNotice: info.LLMNotice,
	}
	if info.CareerID != "" && info.CareerID != game.UnemployedID {
		out.Career = &careerDTO{ID: info.CareerID, Name: info.CareerName}
	}
	if info.Event != nil {
		out.Event = &eventDTO{
			ID: info.Event.ID, Text: info.Event.Text,
			Good: info.Event.Good, LLM: info.Event.LLMGenerated,
			TraumaAlpha: info.Event.TraumaAlpha, TherapyQ: info.Event.TherapyQ,
			Delta: info.Event.Delta,
		}
	}

	if died {
		d.sess.Finish()
		out.Died = true
		out.DeathStatus = d.sess.DeathStatus
		if isNoopLLM(d.sess.Cfg.LLM) {
			if d.lang == "en" {
				out.Epitaph = "That was a life."
			} else {
				out.Epitaph = "一生至此。"
			}
		} else {
			out.Epitaph = d.sess.EpitaphText
		}
		// Save the lineage exactly like main.go does.
		next := &game.Bloodline{
			Generation:   d.sess.Gen,
			Sensitivity:  game.InheritSensitivity(d.sess.Result().Sensitivity, (d.sess.Rng.Float64()*2-1)*0.1, 0.7),
			InheritedTal: "",
		}
		if b, err := game.LoadBloodline(d.bloodlinePath()); err == nil {
			next.InheritedTal = b.InheritedTal
		}
		for _, t := range d.sess.Cfg.Talents {
			if t.Inheritable {
				next.InheritedTal = t.Name
				break
			}
		}
		if err := next.Save(d.bloodlinePath()); err != nil {
			log.Printf("bloodline save failed: %v", err)
		} else {
			out.LineageSaved = true
			out.NextGeneration = next.Generation
			out.NextSensitivity = next.Sensitivity
		}
		_ = os.Remove(d.sessionPath())
		d.sess = nil
	} else {
		// Checkpoint every alive year for crash recovery.
		if err := d.saveCheckpoint(info.Age); err != nil {
			log.Printf("checkpoint save failed: %v", err)
		}
	}
	return out, nil
}

func isNoopLLM(n game.Narrator) bool { return n == nil || n == game.Noop }

// saveCheckpoint writes the session state for replay recovery. API keys are
// intentionally absent: the Android client re-sends the narrator config on
// resume.
func (d *daemon) saveCheckpoint(age int) error {
	rec, ok := d.sess.Cfg.LLM.(*recordingNarrator)
	if !ok {
		return fmt.Errorf("session narrator is not recording")
	}
	cp := &checkpoint{
		Seed:         d.sess.Cfg.Seed,
		Lang:         d.lang,
		Birth:        d.sess.Cfg.Birth,
		Bloodline:    d.sess.Cfg.Bloodline,
		Talents:      d.sess.Cfg.Talents,
		Points:       d.sess.Cfg.Points(),
		MaxAge:       d.sess.Cfg.MaxAge,
		NarrateRatio: d.sess.Cfg.NarrateRatio,
		Trauma:       d.sess.Cfg.Trauma,
		Age:          age,
		LLMCache:     *rec.cache,
	}
	if d.sess.Cfg.InheritTal != nil {
		cp.InheritTal = d.sess.Cfg.InheritTal.Name
	}
	raw, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		return err
	}
	tmp := d.sessionPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, d.sessionPath())
}

func (d *daemon) loadCheckpoint() (*checkpoint, error) {
	raw, err := os.ReadFile(d.sessionPath())
	if err != nil {
		return nil, err
	}
	var cp checkpoint
	if err := json.Unmarshal(raw, &cp); err != nil {
		return nil, fmt.Errorf("parse checkpoint: %w", err)
	}
	return &cp, nil
}

func (d *daemon) resumeSession(req request) (any, error) {
	cp, err := d.loadCheckpoint()
	if err != nil {
		return nil, fmt.Errorf("no checkpoint")
	}
	lang := cp.Lang
	var inherit *game.Talent
	if cp.InheritTal != "" {
		for i := range d.talents(lang) {
			if d.talents(lang)[i].Name == cp.InheritTal {
				t := d.talents(lang)[i]
				inherit = &t
				break
			}
		}
	}
	narrator := d.buildNarrator(req.Narrator, lang)
	rec := &recordingNarrator{inner: narrator, cache: &cp.LLMCache}
	cfg := game.Config{
		Seed:         cp.Seed,
		Birth:        cp.Birth,
		Bloodline:    cp.Bloodline,
		Talents:      cp.Talents,
		InheritTal:   inherit,
		LLM:          rec,
		MaxAge:       cp.MaxAge,
		Trauma:       cp.Trauma,
		NarrateRatio: cp.NarrateRatio,
	}
	if len(cp.Points) == 4 {
		cfg = cfg.WithPoints(cp.Points[0], cp.Points[1], cp.Points[2], cp.Points[3])
	}
	d.sess = game.NewSession(cfg, d.events(lang), d.careers(lang))
	d.lang = lang

	// Deterministic replay to the checkpoint age: the checkpoint was saved
	// AFTER processing cp.Age, so replay must process through cp.Age
	// inclusive (<=, not <) to land on the same session state.
	for d.sess.Age <= cp.Age {
		d.sess.Advance()
		if d.sess.DeathCheck() {
			return nil, fmt.Errorf("checkpoint age %d beyond life end", cp.Age)
		}
	}
	gen := 1
	if cp.Bloodline != nil {
		gen = cp.Bloodline.Generation
	}
	return map[string]any{"resumed": true, "age": cp.Age, "generation": gen}, nil
}

func main() {
	dir := flag.String("dir", "", "data directory (bloodline.json / session.json live here)")
	flag.Parse()
	if *dir == "" {
		log.Fatal("--dir is required")
	}
	d, err := newDaemon(*dir)
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 1<<20), 4<<20)
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)

	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			log.Printf("bad request line: %v", err)
			_ = enc.Encode(response{ID: req.ID, OK: false, Error: "bad request: " + err.Error()})
			continue
		}
		data, err := d.handle(req)
		if err != nil {
			_ = enc.Encode(response{ID: req.ID, OK: false, Error: err.Error()})
			continue
		}
		if err := enc.Encode(response{ID: req.ID, OK: true, Data: data}); err != nil {
			log.Fatalf("write response: %v", err)
		}
		if req.Cmd == "shutdown" {
			return
		}
	}
	if err := sc.Err(); err != nil && err != io.EOF {
		log.Fatalf("stdin: %v", err)
	}
}
