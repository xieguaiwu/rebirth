package game

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
)

//go:embed data/*.json
var eventsFS embed.FS

// Stats are the five player attributes, each clamped to [0,10].
type Stats struct {
	CHR float64 // 颜值
	INT float64 // 智力
	STR float64 // 体质
	MNY float64 // 家境
	SPR float64 // 快乐
}

// Clamp forces every attribute into [0,10] (Gate 7: value-range contract).
func (s *Stats) Clamp() {
	s.CHR = clamp(s.CHR, 0, 10)
	s.INT = clamp(s.INT, 0, 10)
	s.STR = clamp(s.STR, 0, 10)
	s.MNY = clamp(s.MNY, 0, 10)
	s.SPR = clamp(s.SPR, 0, 10)
}

// Effects is a sparse attribute delta.
type Effects struct {
	CHR, INT, STR, MNY, SPR float64
}

// Cond gates event eligibility by attribute ranges (inclusive).
// Zero-valued bounds mean unbounded on that side.
type Cond struct {
	MinCHR float64 `json:"min_chr"`
	MaxCHR float64 `json:"max_chr"`
	MinINT float64 `json:"min_int"`
	MaxINT float64 `json:"max_int"`
	MinSTR float64 `json:"min_str"`
	MaxSTR float64 `json:"max_str"`
	MinMNY float64 `json:"min_mny"`
	MaxMNY float64 `json:"max_mny"`
}

// Event is one life event.
type Event struct {
	ID     string   `json:"id"`
	Text   string   `json:"text"`
	MinAge int      `json:"min_age"`
	MaxAge int      `json:"max_age"`
	Weight float64  `json:"weight"` // base weight
	Good   bool     `json:"good"`   // positive event? boosted by good luck
	Cond   *Cond    `json:"cond,omitempty"`
	Delta  *Effects `json:"delta,omitempty"`
	// Trauma intensity alpha in [0,1]; 0 = not a trauma trigger.
	TraumaAlpha float64 `json:"trauma_alpha,omitempty"`
	// Therapy quality q in [0,1]; >0 marks a healing event.
	TherapyQ float64 `json:"therapy_q,omitempty"`
	// Career restricts the event to holders of one career track.
	// Empty = any track.
	Career string `json:"career,omitempty"`
	// LLMGenerated marks events injected at runtime by the model.
	LLMGenerated bool `json:"-"`
}

func (e Event) eligible(age int, s Stats, career string) bool {
	if age < e.MinAge || age > e.MaxAge {
		return false
	}
	if e.Career != "" && e.Career != career {
		return false
	}
	return condMet(e.Cond, s)
}

func inRange(v, lo, hi float64) bool {
	if lo != 0 && v < lo {
		return false
	}
	if hi != 0 && v > hi {
		return false
	}
	return true
}

// LoadEvents parses every embedded data/events_*.json shard and concatenates
// them. Shards keep the growing dataset reviewable file by file.
func LoadEvents() ([]Event, error) {
	matches, err := fs.Glob(eventsFS, "data/events_*.json")
	if err != nil {
		return nil, fmt.Errorf("glob events: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no events_*.json shards embedded")
	}
	sort.Strings(matches)
	var evs []Event
	for _, m := range matches {
		raw, err := eventsFS.ReadFile(m)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", m, err)
		}
		var shard []Event
		if err := json.Unmarshal(raw, &shard); err != nil {
			return nil, fmt.Errorf("parse %s: %w", m, err)
		}
		evs = append(evs, shard...)
	}
	if len(evs) == 0 {
		return nil, fmt.Errorf("event shards contain no events")
	}
	return evs, nil
}

// Fortune is an AR(1) luck process: luck_t = rho*luck_{t-1} + eps_t,
// clipped to [-1,1]. It autocorrelates fate across years — good years
// cluster, disasters cluster — replacing i.i.d. sampling.
type Fortune struct {
	Luck float64
	Rho  float64
	Rng  *rand.Rand
}

// NewFortune seeds the AR(1) process.
func NewFortune(rnd *rand.Rand, rho float64) *Fortune {
	return &Fortune{Luck: rnd.Float64()*2 - 1, Rho: rho, Rng: rnd}
}

// Next advances one year and returns the new luck value.
func (f *Fortune) Next() float64 {
	eps := (f.Rng.Float64()*2 - 1) * 0.5
	f.Luck = clamp(f.Rho*f.Luck+eps, -1, 1)
	return f.Luck
}

// PickEvent performs weighted sampling over eligible events. Good-luck years
// multiply positive-event weights by (1+luck), bad years suppress them;
// trauma load multiplies negative-event weights when pathological.
func PickEvent(evs []Event, age int, s Stats, career string, rng *rand.Rand, luck float64, pathological bool) *Event {
	type cand struct {
		idx    int
		weight float64
	}
	var cands []cand
	total := 0.0
	for i := range evs {
		e := &evs[i]
		if !e.eligible(age, s, career) {
			continue
		}
		w := e.Weight
		if e.Good {
			w *= 1 + 0.6*luck
		} else {
			w *= 1 - 0.3*luck
		}
		if pathological && !e.Good {
			w *= 1.8 // pathological attractor biases toward dark events
		}
		if w <= 0 {
			continue
		}
		cands = append(cands, cand{idx: i, weight: w})
		total += w
	}
	if total <= 0 || len(cands) == 0 {
		return nil
	}
	r := rng.Float64() * total
	for _, c := range cands {
		r -= c.weight
		if r <= 0 {
			return &evs[c.idx]
		}
	}
	// Float rounding guard: fall back to the last candidate.
	last := cands[len(cands)-1]
	return &evs[last.idx]
}

// ApplyDelta mutates stats by the event delta then clamps.
func (s *Stats) ApplyDelta(d *Effects) {
	if d == nil {
		return
	}
	s.CHR += d.CHR
	s.INT += d.INT
	s.STR += d.STR
	s.MNY += d.MNY
	s.SPR += d.SPR
	s.Clamp()
}

// Talent is an inherited-or-drawn perk applied at birth.
type Talent struct {
	Name        string  `json:"name"`
	Desc        string  `json:"desc"`
	Bonus       Effects `json:"bonus"`
	TraumaMult  float64 `json:"trauma_mult"` // multiplies shock alpha (1 = normal)
	LuckBonus   float64 `json:"luck_bonus"`  // added to AR(1) luck each year
	TherapyMult float64 `json:"therapy_mult"`
	Inheritable bool    `json:"inheritable"`
}

// LoadTalents reads talents.json from the same embedded directory.
func LoadTalents() ([]Talent, error) {
	raw, err := eventsFS.ReadFile("data/talents.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded talents: %w", err)
	}
	var ts []Talent
	if err := json.Unmarshal(raw, &ts); err != nil {
		return nil, fmt.Errorf("parse talents.json: %w", err)
	}
	return ts, nil
}

// DrawTalents samples n distinct talents.
func DrawTalents(ts []Talent, n int, rng *rand.Rand) []Talent {
	if n > len(ts) {
		n = len(ts)
	}
	idx := rng.Perm(len(ts))[:n]
	out := make([]Talent, 0, n)
	for _, i := range idx {
		out = append(out, ts[i])
	}
	return out
}

// Bloodline carries heritable state across runs.
type Bloodline struct {
	Generation   int     `json:"generation"`
	Sensitivity  float64 `json:"sensitivity"` // inherited HPA baseline
	InheritedTal string  `json:"inherited_talent"`
}

// LoadBloodline reads the save file; missing file returns generation 0
// (no ancestors). The counter is NOT incremented here — main owns that.
func LoadBloodline(path string) (*Bloodline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return &Bloodline{Generation: 0}, nil
	}
	var b Bloodline
	if err := json.Unmarshal(raw, &b); err != nil {
		return &Bloodline{Generation: 0}, fmt.Errorf("parse bloodline save: %w", err)
	}
	return &b, nil
}

// Save writes the bloodline atomically enough for a single-user CLI,
// creating the parent directory when missing.
func (b *Bloodline) Save(path string) error {
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, raw, 0o600)
}
