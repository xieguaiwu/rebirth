package game

import (
	"fmt"
	"io"
	"math/rand"
	"strings"
)

// YearInfo is the structured result of one simulated year. The daemon
// (cmd/mobile) serializes it to the mobile UI; the CLI uses only Lines.
type YearInfo struct {
	Age          int
	Lines        []string // recorded output lines for this year (CLI-identical)
	CareerID     string   // "" = unemployed
	CareerName   string
	CareerChange string // "", "enter", "quit", "retire"
	Event        *Event // fired event (nil when the year had none)
	Narrated     bool   // the event line was rewritten by the LLM narrator
	Stats        Stats
	TraumaM      float64
	TraumaA      float64
	TraumaP      float64
	TraumaLoad   float64
	Pathological bool
	Luck         float64
	LLMNotice    bool // breaker notice printed this year
}

// Session is the resumable per-life stepper extracted from Run. It owns
// every piece of mutable run state; Advance advances exactly one year and
// DeathCheck applies the post-year death accounting. The CLI drives it in
// a thin loop (byte-identical output); the mobile daemon drives it one
// year per protocol round-trip and checkpoints it for replay recovery.
type Session struct {
	Cfg     Config
	Evs     []Event
	Careers []*Career

	Rng     *rand.Rand
	Params  TraumaParams
	Trauma  *TraumaState
	S       Stats
	Sens    float64 // inherited + birth sensitivity (clamped later by trauma)
	Gen     int
	Fortune *Fortune
	Facts   Facts
	Used    map[string]bool

	CareerID   string
	CareerName string

	History     []string
	SprLowYears int
	Age         int // next age to advance (0-based)
	DeathAge    int // -1 while alive
	DeathStatus string
	EpitaphText string
	LLMWarned   bool
	Aborted     bool
	finished    bool

	// Out receives hint/notice output exactly when the CLI would print it
	// (byte-identical ordering). The daemon passes io.Discard and reads
	// YearInfo/LLMNotice instead.
	Out io.Writer

	// Per-year scratch (reset at the start of each Advance).
	yearLines  []string
	yearChange string
	yearNotice bool
}

// NewSession initializes a life exactly like Run did: rng, trauma params
// (including the v0.9.0 sensitivity threshold shift), attribute allocation,
// fortune and facts. All state that used to be local in Run lives on the
// Session so a life can be stepped, checkpointed and replayed.
func NewSession(cfg Config, evs []Event, careers []*Career) *Session {
	s := &Session{
		Cfg:        cfg,
		Evs:        evs,
		Careers:    careers,
		Rng:        rand.New(rand.NewSource(cfg.Seed)),
		CareerID:   UnemployedID,
		CareerName: "无业",
		DeathAge:   -1,
		Out:        io.Discard,
	}
	if cfg.MaxAge <= 0 {
		s.Cfg.MaxAge = 100
	}
	params := DefaultTraumaParams()
	if cfg.Trauma != nil {
		params = *cfg.Trauma
	}
	sens := 0.0
	gen := 1
	if cfg.Bloodline != nil {
		sens = cfg.Bloodline.Sensitivity
		gen = cfg.Bloodline.Generation
	}
	if cfg.Birth != nil {
		sens += cfg.Birth.SensitivityAdd
	}
	// v0.9.0: inherited sensitivity lowers the pathological-entry threshold
	// itself — the heritable trait is a real bias toward the attractor, not
	// only a birth-year baseline. Hysteresis pair is guarded so EnterAt can
	// never drop to/below ExitAt (custom configs may set either).
	params.EnterAt -= params.SensEnterAt * Clamp01(sens)
	if params.EnterAt <= params.ExitAt {
		params.EnterAt = params.ExitAt + 0.05
	}
	s.Params = params
	s.Sens = sens
	s.Gen = gen
	s.Trauma = NewTraumaState(Clamp01(sens), params)

	if len(cfg.points) == 4 {
		s.S.CHR += cfg.points[0]
		s.S.INT += cfg.points[1]
		s.S.STR += cfg.points[2]
		s.S.MNY += cfg.points[3]
	}
	if cfg.Birth != nil {
		s.S.ApplyDelta(&cfg.Birth.Bonus)
	}
	for _, t := range cfg.Talents {
		s.S.ApplyDelta(&t.Bonus)
	}
	if cfg.InheritTal != nil {
		s.S.ApplyDelta(&cfg.InheritTal.Bonus)
	}
	s.S.Clamp()
	// Nobody starts dead: birth/talent negatives may not zero a stat.
	s.S.CHR = maxF(s.S.CHR, 1)
	s.S.INT = maxF(s.S.INT, 1)
	s.S.STR = maxF(s.S.STR, 1)
	s.S.MNY = maxF(s.S.MNY, 0)
	s.S.SPR = maxF(s.S.SPR, 1)

	s.Fortune = NewFortune(s.Rng, 0.7)
	s.Facts = NewFacts(cfg.Birth)
	s.Used = map[string]bool{}
	return s
}

// Points exposes the attribute allocation (the daemon checkpoints it).
func (c Config) Points() []float64 { return c.points }

// record stores one output line exactly like Run's closure did: printed to
// Out immediately (byte-identical ordering) and appended to history.
func (s *Session) record(line string) {
	fmt.Fprintln(s.Out, line)
	s.History = append(s.History, line)
	s.yearLines = append(s.yearLines, line)
}

// warnBroken prints the one-time narrator breaker notice when it trips.
func (s *Session) warnBroken() {
	if !s.LLMWarned && !isNoop(s.Cfg.LLM) && s.Cfg.LLM.Broken() {
		s.LLMWarned = true
		fmt.Fprintln(s.Out, "\n[提示] 叙事通道连续失败（不可用或过慢），本世余下改为纯本地叙事。")
		s.yearNotice = true
	}
}

// Advance simulates exactly one year (the body of Run's loop, minus the
// post-year death accounting which lives in DeathCheck so the CLI can
// pause between the two phases, byte-identical to the original Run).
func (s *Session) Advance() *YearInfo {
	age := s.Age
	info := &YearInfo{Age: age}
	s.yearLines = nil
	s.yearChange = ""
	s.yearNotice = false

	luck := clamp(s.Fortune.Next()+talentLuck(s.Cfg.Talents, s.Cfg.InheritTal), -1, 1)
	info.Luck = luck

	// --- career decision window ---
	cur := findCareer(s.Careers, s.CareerID)
	if cur == nil || cur.ID == UnemployedID {
		if careerWindows[age] {
			load := s.Trauma.Load(s.Params)
			if c := PickCareer(s.Careers, age, s.S, load, s.Rng, luck); c != nil {
				s.CareerID = c.ID
				s.CareerName = c.Name
				cur = c
				s.yearChange = "enter"
				s.record(fmt.Sprintf("[%3d 岁] ★ 入行：%s —— %s", age, c.Name, c.Desc))
			}
		}
	} else {
		// Yearly drift and recurring exposure while employed.
		s.S.ApplyDelta(&cur.YearlyDelta)
		if cur.YearlyShock > 0 {
			s.Trauma.Shock(cur.YearlyShock, s.Params)
		}
		if lost := careerQuitCheck(cur, s.S); lost {
			s.record(fmt.Sprintf("[%3d 岁] 离开「%s」。", age, cur.Name))
			s.CareerID = UnemployedID
			s.CareerName = "无业"
			cur = nil
			s.yearChange = "quit"
		} else if cur.MaxAge > 0 && age >= cur.MaxAge {
			s.record(fmt.Sprintf("[%3d 岁] 从「%s」退休。", age, cur.Name))
			s.CareerID = UnemployedID
			s.CareerName = "退休"
			cur = nil
			s.yearChange = "retire"
		}
	}

	// --- event roll ---
	ev := PickEvent(s.Evs, age, s.S, s.CareerID, s.Facts, s.Used, s.Rng, luck, s.Trauma.Pathological, s.Params, Clamp01(s.Sens))
	trigger := false
	therapyQ := 0.0

	// Once per decade past 20, the model may weave a unique fate event.
	if age >= 20 && age%10 == 0 && !isNoop(s.Cfg.LLM) {
		summary := stateSummary(age, s.S, s.Trauma, s.CareerName, s.Params)
		if s.Cfg.Hints {
			fmt.Fprint(s.Out, hintPending)
		}
		if fate, ok := s.Cfg.LLM.FateEvent(age, summary); ok {
			clearHint(s.Out, s.Cfg.Hints)
			ev = &fate
			s.Used[ev.ID] = true // fate events also never repeat
		} else {
			clearHint(s.Out, s.Cfg.Hints)
		}
		s.warnBroken()
	}

	if ev != nil {
		s.Used[ev.ID] = true
		established := ev.Sets
		if ev.Context != "" {
			established = ev.Context // v0.6.0 shards use the context key
		}
		if established != "" {
			// "!fact" clears a storyline fact (e.g. rescue ends the
			// cult isolation and reopens ordinary social events).
			if name := strings.TrimPrefix(established, "!"); name != established {
				delete(s.Facts, name)
			} else {
				s.Facts[established] = true
			}
		}
		text := ev.Text
		if !isNoop(s.Cfg.LLM) && (ev.TraumaAlpha > 0 || ev.Good) &&
			narrateSample(ev.ID, s.Cfg.NarrateRatio) {
			if s.Cfg.Hints {
				fmt.Fprint(s.Out, hintPending)
			}
			text = s.Cfg.LLM.Narrate(age,
				fmt.Sprintf("事件:%s 职业:%s 属性:%+v 负荷:%.2f", ev.ID, s.CareerName, s.S, s.Trauma.Load(s.Params)),
				text)
			clearHint(s.Out, s.Cfg.Hints)
			s.warnBroken()
		}
		line := fmt.Sprintf("[%3d 岁] %s", age, text)
		s.record(line)
		info.Narrated = text != ev.Text
		s.S.ApplyDelta(ev.Delta)
		if ev.TraumaAlpha > 0 {
			alpha := ev.TraumaAlpha * s.Params.EventScale * talentTraumaMult(s.Cfg.Talents, s.Cfg.InheritTal)
			s.Trauma.Shock(alpha, s.Params)
			trigger = true
		}
		if ev.TherapyQ > 0 {
			therapyQ = ev.TherapyQ * talentTherapyMult(s.Cfg.Talents, s.Cfg.InheritTal)
		}
	}

	s.Trauma.Step(trigger, therapyQ, s.Params)

	// Pathological attractor drains happiness yearly (clinical analogue).
	if s.Trauma.Pathological {
		s.S.SPR -= 0.25 // attractor drains joy, but slower than despair
	}
	s.S.Clamp()

	s.Age++
	info.Lines = s.yearLines
	info.CareerID = s.CareerID
	info.CareerName = s.CareerName
	info.CareerChange = s.yearChange
	info.Event = ev
	info.Stats = s.S
	info.TraumaM = s.Trauma.M
	info.TraumaA = s.Trauma.A
	info.TraumaP = s.Trauma.P
	info.TraumaLoad = s.Trauma.Load(s.Params)
	info.Pathological = s.Trauma.Pathological
	info.LLMNotice = s.yearNotice
	return info
}

// DeathCheck applies the post-year death accounting (the phase that in the
// original Run came after the step-pause): low-SPR streak tracking and the
// body/spirit death checks. It returns true when the life ends this year.
// It is idempotent: once the life has ended it keeps returning true.
func (s *Session) DeathCheck() bool {
	if s.finished {
		return true
	}
	if s.DeathAge >= 0 {
		return true
	}
	curYear := s.Age - 1 // the year Advance just processed
	if s.S.SPR <= 0.01 {
		s.SprLowYears++
	} else {
		s.SprLowYears = 0
	}
	if s.S.STR <= 0.01 || s.SprLowYears >= 5 {
		s.DeathAge = curYear
		return true
	}
	if curYear == s.Cfg.MaxAge {
		s.DeathAge = curYear
		return true
	}
	return false
}

// Finish computes the end-of-life status (idempotent). The CLI calls it
// after the loop; the daemon calls it when DeathCheck reports death. The
// epitaph is computed separately by FinishEpitaph so the CLI can print
// the death line BEFORE the (potentially blocking) narrator call and show
// its hint indicator, exactly like the pre-session Run.
func (s *Session) Finish() {
	if s.finished {
		return
	}
	s.finished = true
	if s.DeathAge < 0 {
		s.DeathAge = s.Age - 1
	}
	status := "安详离世"
	switch {
	case s.DeathAge <= 5:
		status = "幼年夭折"
	case s.S.STR <= 0.01:
		status = "身体耗竭"
	case s.SprLowYears >= 5:
		if s.DeathAge < 18 {
			status = "未成年早逝"
		} else {
			status = "长期抑郁"
		}
	}
	if s.Trauma.Pathological {
		status += "（终生生处于创伤病理态）"
	}
	s.DeathStatus = status
}

// FinishEpitaph computes the epitaph text on demand (idempotent, one
// narrator call per life when a narrator is configured).
func (s *Session) FinishEpitaph() string {
	if s.EpitaphText != "" {
		return s.EpitaphText
	}
	n := s.Cfg.LLM
	if n == nil || isNoop(n) {
		s.EpitaphText = ""
		return ""
	}
	s.EpitaphText = n.Epitaph(stateSummary(s.DeathAge, s.S, s.Trauma, s.CareerName, s.Params))
	return s.EpitaphText
}

// Result summarizes one completed (or aborted) life, mirroring Run's
// Result contract so main's lineage logic works unchanged.
func (s *Session) Result() *Result {
	return &Result{
		Age:          s.DeathAge,
		Career:       s.CareerName,
		Stats:        s.S,
		Pathological: s.Trauma.Pathological,
		Sensitivity:  EndingSensitivity(s.Trauma, s.Params),
		History:      s.History,
		Aborted:      s.Aborted,
	}
}

// Done reports whether the life has ended (death detected or aborted).
func (s *Session) Done() bool { return s.finished || s.Aborted }
