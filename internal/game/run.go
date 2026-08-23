package game

import (
	"fmt"
	"io"
	"math/rand"
	"strings"
)

// Noop is the disabled-narrator sentinel; main passes it when LLM is off.
var Noop Narrator = noop{}

type noop struct{}

func (noop) Narrate(age int, summary, fallback string) string { return fallback }
func (noop) FateEvent(age int, summary string) (Event, bool)  { return Event{}, false }
func (noop) Epitaph(summary string) string                    { return "一生至此。" }

// Narrator renders optional LLM flavor. Implementations must fail soft:
// every method falls back to deterministic text.
type Narrator interface {
	// Narrate renders one event line; returns fallback text on any failure.
	Narrate(age int, summary, fallback string) string
	// FateEvent asks the model for a unique mid-life event; ok=false means
	// the caller must use its deterministic pool instead.
	FateEvent(age int, summary string) (Event, bool)
	// Epitaph writes the closing summary of a finished life.
	Epitaph(summary string) string
}

func isNoop(n Narrator) bool { return n == nil || n == Noop }

// Config controls one run.
type Config struct {
	Seed       int64
	Birth      *Birth // starting background; nil = plain start
	Bloodline  *Bloodline
	Talents    []Talent // chosen talents (already picked by UI layer)
	InheritTal *Talent  // bloodline-inherited talent, may be nil
	LLM        Narrator
	MaxAge     int       // hard age cap, default 100
	points     []float64 // CHR/INT/STR/MNY allocation, applied at birth

	// Step turns on manual advance: after every yearly line, Pause is
	// called; returning true aborts the life gracefully. Nil-safe.
	Step  bool
	Pause func() bool
}

// WithPoints sets the player's attribute allocation and returns cfg for chaining.
func (c Config) WithPoints(chr, int_, str, mny float64) Config {
	c.points = []float64{chr, int_, str, mny}
	return c
}

// Result summarizes one completed life.
type Result struct {
	Age          int
	Career       string
	Stats        Stats
	Pathological bool
	Sensitivity  float64 // heritable scalar for the next generation
	History      []string
	Aborted      bool // player quit mid-life via the step prompt
}

// careerWindows are the ages at which a new track may be entered.
var careerWindows = map[int]bool{16: true, 19: true, 23: true, 27: true, 32: true, 38: true, 45: true}

// Run plays one life end-to-end, emitting progress lines to w.
func Run(w io.Writer, cfg Config, evs []Event, careers []*Career) (*Result, error) {
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 100
	}
	rng := rand.New(rand.NewSource(cfg.Seed))
	params := DefaultTraumaParams()

	s := Stats{}
	sens := 0.0
	gen := 1
	if cfg.Bloodline != nil {
		sens = cfg.Bloodline.Sensitivity
		gen = cfg.Bloodline.Generation
	}
	if cfg.Birth != nil {
		sens += cfg.Birth.SensitivityAdd
	}
	trauma := NewTraumaState(clamp01(sens), params)

	fmt.Fprintf(w, "\n════ 第 %d 代 · 种子 %d ════\n", gen, cfg.Seed)
	if len(cfg.points) == 4 {
		s.CHR += cfg.points[0]
		s.INT += cfg.points[1]
		s.STR += cfg.points[2]
		s.MNY += cfg.points[3]
	}
	if cfg.Birth != nil {
		s.ApplyDelta(&cfg.Birth.Bonus)
		fmt.Fprintf(w, "[出身] %s —— %s\n", cfg.Birth.Name, cfg.Birth.Desc)
	}
	if sens > 0.05 {
		fmt.Fprintf(w, "[血脉] 应激敏感性基线 %.2f（高于此值更易受创）\n", clamp01(sens))
	}
	for _, t := range cfg.Talents {
		s.ApplyDelta(&t.Bonus)
		fmt.Fprintf(w, "[天赋] %s —— %s\n", t.Name, t.Desc)
	}
	if cfg.InheritTal != nil {
		s.ApplyDelta(&cfg.InheritTal.Bonus)
		fmt.Fprintf(w, "[血脉天赋] %s —— %s\n", cfg.InheritTal.Name, cfg.InheritTal.Desc)
	}
	s.Clamp()
	// Nobody starts dead: birth/talent negatives may not zero a stat
	// (momus P2-4: orphan build used to die at age 0).
	s.CHR = maxF(s.CHR, 1)
	s.INT = maxF(s.INT, 1)
	s.STR = maxF(s.STR, 1)
	s.MNY = maxF(s.MNY, 0)
	s.SPR = maxF(s.SPR, 1)

	fortune := NewFortune(rng, 0.7)
	var history []string
	facts := NewFacts(cfg.Birth) // storyline facts: "cult", "broken_home"...
	used := map[string]bool{}    // lifetime event uniqueness
	deathAge := -1
	sprLowYears := 0
	careerID := UnemployedID
	careerName := "无业"

	record := func(line string) {
		fmt.Fprintln(w, line)
		history = append(history, line)
	}

	for age := 0; age <= cfg.MaxAge; age++ {
		luck := clamp(fortune.Next()+talentLuck(cfg.Talents, cfg.InheritTal), -1, 1)

		// --- career decision window ---
		cur := findCareer(careers, careerID)
		if cur == nil || cur.ID == UnemployedID {
			if careerWindows[age] && age <= 55 {
				load := trauma.Load(params)
				if c := PickCareer(careers, age, s, load, rng, luck); c != nil {
					careerID = c.ID
					careerName = c.Name
					cur = c
					record(fmt.Sprintf("[%3d 岁] ★ 入行：%s —— %s", age, c.Name, c.Desc))
				}
			}
		} else {
			// Yearly drift and recurring exposure while employed.
			s.ApplyDelta(&cur.YearlyDelta)
			if cur.YearlyShock > 0 {
				trauma.Shock(cur.YearlyShock, params)
			}
			if lost := careerQuitCheck(cur, s); lost {
				record(fmt.Sprintf("[%3d 岁] 离开「%s」。", age, cur.Name))
				careerID = UnemployedID
				careerName = "无业"
				cur = nil
			} else if cur.MaxAge > 0 && age >= cur.MaxAge {
				record(fmt.Sprintf("[%3d 岁] 从「%s」退休。", age, cur.Name))
				careerID = UnemployedID
				careerName = "退休"
				cur = nil
			}
		}

		// --- event roll ---
		ev := PickEvent(evs, age, s, careerID, facts, used, rng, luck, trauma.Pathological)
		trigger := false
		therapyQ := 0.0

		// Once per decade past 20, the model may weave a unique fate event.
		if age >= 20 && age%10 == 0 && !isNoop(cfg.LLM) {
			summary := stateSummary(age, s, trauma, careerName, params)
			if fate, ok := cfg.LLM.FateEvent(age, summary); ok {
				ev = &fate
			}
		}

		if ev != nil {
			used[ev.ID] = true
			if ev.Sets != "" {
				// "!fact" clears a storyline fact (e.g. rescue ends the
				// cult isolation and reopens ordinary social events).
				if name := strings.TrimPrefix(ev.Sets, "!"); name != ev.Sets {
					delete(facts, name)
				} else {
					facts[ev.Sets] = true
				}
			}
			text := ev.Text
			if !isNoop(cfg.LLM) && (ev.TraumaAlpha > 0 || ev.Good) {
				text = cfg.LLM.Narrate(age,
					fmt.Sprintf("事件:%s 职业:%s 属性:%+v 负荷:%.2f", ev.ID, careerName, s, trauma.Load(params)),
					text)
			}
			line := fmt.Sprintf("[%3d 岁] %s", age, text)
			record(line)
			s.ApplyDelta(ev.Delta)
			if ev.TraumaAlpha > 0 {
				alpha := ev.TraumaAlpha * talentTraumaMult(cfg.Talents, cfg.InheritTal)
				trauma.Shock(alpha, params)
				trigger = true
			}
			if ev.TherapyQ > 0 {
				therapyQ = ev.TherapyQ * talentTherapyMult(cfg.Talents, cfg.InheritTal)
			}
		}

		trauma.Step(trigger, therapyQ, params)

		// Pathological attractor drains happiness yearly (clinical analogue).
		if trauma.Pathological {
			s.SPR -= 0.25 // attractor drains joy, but slower than despair
		}
		s.Clamp()

		// Manual advance: one Enter per year of life.
		if cfg.Step && cfg.Pause != nil && cfg.Pause() {
			res := &Result{Age: age, Career: careerName, Stats: s,
				Pathological: trauma.Pathological, Aborted: true}
			fmt.Fprintf(w, "\n──── 玩家中途离开（%d 岁）────\n", age)
			return res, nil
		}

		if s.SPR <= 0.01 {
			sprLowYears++
		} else {
			sprLowYears = 0
		}
		// Death checks: body fails at 0 STR; 5 straight years at 0 SPR.
		if s.STR <= 0.01 || sprLowYears >= 5 {
			deathAge = age
			break
		}
		if age == cfg.MaxAge {
			deathAge = age
		}
	}

	res := &Result{
		Age:          deathAge,
		Career:       careerName,
		Stats:        s,
		Pathological: trauma.Pathological,
		Sensitivity:  EndingSensitivity(trauma, params),
		History:      history,
	}

	status := "安详离世"
	switch {
	case res.Age <= 5:
		status = "幼年夭折" // momus P2-4: toddlers are not "chronically depressed"
	case s.STR <= 0.01:
		status = "身体耗竭"
	case sprLowYears >= 5:
		if res.Age < 18 {
			status = "未成年早逝"
		} else {
			status = "长期抑郁"
		}
	}
	if res.Pathological {
		status += "（终生生处于创伤病理态）"
	}
	fmt.Fprintf(w, "\n──── 人生结束：%d 岁 · 职业：%s · %s ────\n", res.Age, res.Career, status)

	if !isNoop(cfg.LLM) {
		fmt.Fprintln(w, "墓志铭："+cfg.LLM.Epitaph(stateSummary(res.Age, s, trauma, careerName, params)))
	}
	return res, nil
}

func findCareer(cs []*Career, id string) *Career {
	for _, c := range cs {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// careerQuitCheck drops the track when a gate stat collapses.
func careerQuitCheck(c *Career, s Stats) bool {
	if c.QuitBelow <= 0 {
		return false
	}
	switch c.QuitIfStat {
	case "str":
		return s.STR < c.QuitBelow
	case "chr":
		return s.CHR < c.QuitBelow
	case "int":
		return s.INT < c.QuitBelow
	case "spr":
		return s.SPR < c.QuitBelow
	case "mny": // momus P1-2: digital nomad's quit rule was silently dead
		return s.MNY < c.QuitBelow
	default:
		return false
	}
}

func talentLuck(ts []Talent, inherit *Talent) float64 {
	v := 0.0
	for _, t := range ts {
		v += t.LuckBonus
	}
	if inherit != nil {
		v += inherit.LuckBonus
	}
	return v
}

func talentTraumaMult(ts []Talent, inherit *Talent) float64 {
	v := 1.0
	for _, t := range ts {
		if t.TraumaMult > 0 {
			v *= t.TraumaMult
		}
	}
	if inherit != nil && inherit.TraumaMult > 0 {
		v *= inherit.TraumaMult
	}
	return v
}

func talentTherapyMult(ts []Talent, inherit *Talent) float64 {
	v := 1.0
	for _, t := range ts {
		if t.TherapyMult > 0 {
			v *= t.TherapyMult
		}
	}
	if inherit != nil && inherit.TherapyMult > 0 {
		v *= inherit.TherapyMult
	}
	return v
}

// stateSummary builds the compact prompt payload for the narrator.
func stateSummary(age int, s Stats, t *TraumaState, career string, p TraumaParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "年龄%d 职业%s 颜值%.1f 智力%.1f 体质%.1f 家境%.1f 快乐%.1f | 创伤记忆%.2f 杏仁核%.2f 前额叶%.2f 负荷%.2f",
		age, career, s.CHR, s.INT, s.STR, s.MNY, s.SPR, t.M, t.A, t.P, t.Load(p))
	if t.Pathological {
		b.WriteString(" [病理态]")
	}
	return b.String()
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
