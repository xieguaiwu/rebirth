package game

import (
	"bytes"
	"math/rand"
	"strings"
	"testing"
)

func TestTraumaShockBounded(t *testing.T) {
	p := DefaultTraumaParams()
	st := NewTraumaState(0, p)
	for i := 0; i < 50; i++ {
		st.Shock(1.0, p)
		if st.M < 0 || st.M > 1 {
			t.Fatalf("memory escaped [0,1]: %f", st.M)
		}
		st.Step(false, 0, p)
	}
}

func TestHysteresisAsymmetry(t *testing.T) {
	p := DefaultTraumaParams()
	// Construct states directly so the test exercises the latch logic,
	// not the full ODE trajectory.
	st := &TraumaState{M: 0.95, A: 0.85, P: 0.3}
	st.Step(true, 0, p) // load ~0.94 > EnterAt -> must latch on
	if !st.Pathological {
		t.Fatalf("latch failed to engage at load %.2f", st.Load(p))
	}

	// Drop into the hysteresis band (ExitAt < load < EnterAt):
	// the attractor must persist even though entry condition no longer holds.
	st.M, st.A = 0.7, 0.25
	if l := st.Load(p); l >= p.EnterAt || l <= p.ExitAt {
		t.Fatalf("band setup wrong: load %.2f", l)
	}
	st.Step(true, 0, p)
	if !st.Pathological {
		t.Fatalf("hysteresis violated: released while load %.2f > ExitAt", st.Load(p))
	}

	// Below ExitAt the system finally releases.
	st.M, st.A = 0.15, 0.1
	st.Step(false, 0, p)
	if st.Pathological && st.Load(p) < p.ExitAt {
		t.Fatalf("latch stuck below exit threshold: load %.2f", st.Load(p))
	}
}

func TestInheritSubAdditive(t *testing.T) {
	got := InheritSensitivity(1.0, 0, 0.7)
	if got >= 1.0 {
		t.Fatalf("sub-additivity broken: child=%f from parent=1.0", got)
	}
	if got < 0.69 || got > 0.71 {
		t.Fatalf("expected ~0.70, got %f", got)
	}
	// Noise is clamped to +-0.2.
	if InheritSensitivity(0, -5, 0.7) < 0 {
		t.Fatal("negative sensitivity leaked through clamp")
	}
}

func TestExtinctionRequiresSafeContext(t *testing.T) {
	p := DefaultTraumaParams()
	safe := NewTraumaState(0, p)
	unsafe := NewTraumaState(0, p)
	safe.Shock(0.8, p)
	unsafe.Shock(0.8, p)
	for i := 0; i < 20; i++ {
		safe.Step(false, 0, p)  // no cue, low arousal -> extinction
		unsafe.Step(true, 0, p) // yearly cues -> reconsolidation
	}
	if unsafe.M <= safe.M {
		t.Fatalf("cue exposure should preserve memory: unsafe=%f safe=%f", unsafe.M, safe.M)
	}
}

func TestPickEventDeterministic(t *testing.T) {
	evs, err := LoadEvents()
	if err != nil {
		t.Skipf("dataset: %v", err)
	}
	rngA := rand.New(rand.NewSource(42))
	rngB := rand.New(rand.NewSource(42))
	s := Stats{CHR: 5, INT: 5, STR: 5, MNY: 5, SPR: 5}
	for age := 0; age < 60; age++ {
		a := PickEvent(evs, age, s, "", Facts{}, nil, rngA, 0, false)
		b := PickEvent(evs, age, s, "", Facts{}, nil, rngB, 0, false)
		if (a == nil) != (b == nil) {
			t.Fatalf("age %d: nil mismatch", age)
		}
		if a != nil && a.ID != b.ID {
			t.Fatalf("age %d: seed determinism broken: %s vs %s", age, a.ID, b.ID)
		}
	}
}

func TestCareerGateOnEvents(t *testing.T) {
	evs, err := LoadEvents()
	if err != nil {
		t.Skipf("dataset: %v", err)
	}
	s := Stats{CHR: 9, INT: 9, STR: 9, MNY: 9, SPR: 9}
	foundUnemployed := false
	for _, e := range evs {
		if e.Career == "star" && e.eligible(25, s, UnemployedID, Facts{}) {
			t.Fatalf("star event %s leaked to unemployed holder", e.ID)
		}
		if e.Career == "" && e.eligible(25, s, UnemployedID, Facts{}) {
			foundUnemployed = true
		}
	}
	if !foundUnemployed {
		t.Fatal("no career-agnostic events eligible — gating too strict")
	}
}

func TestCultLeaderRequiresTrauma(t *testing.T) {
	careers, err := LoadCareers()
	if err != nil {
		t.Skipf("dataset: %v", err)
	}
	var cult *Career
	for _, c := range careers {
		if c.ID == "cult_leader" {
			cult = c
		}
	}
	if cult == nil {
		t.Fatal("cult_leader missing from dataset")
	}
	s := Stats{CHR: 10, INT: 10, STR: 10, MNY: 10, SPR: 10}
	if cult.eligible(30, s, 0.0) {
		t.Fatal("cult leader entered with zero trauma load")
	}
	if !cult.eligible(30, s, 0.6) {
		t.Fatal("cult leader refused at trauma load 0.6")
	}
}

func TestApplyDeltaClamps(t *testing.T) {
	s := Stats{CHR: 9, INT: 9, STR: 9, MNY: 9, SPR: 9}
	s.ApplyDelta(&Effects{CHR: 5, STR: -100})
	if s.CHR != 10 {
		t.Fatalf("CHR clamp upper failed: %f", s.CHR)
	}
	if s.STR != 0 {
		t.Fatalf("STR clamp lower failed: %f", s.STR)
	}
}

func TestFullAutoRun(t *testing.T) {
	evs, err := LoadEvents()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	careers, _ := LoadCareers()
	births, _ := LoadBirths()
	draw := DrawBirths(births, 3, rand.New(rand.NewSource(7)))
	cfg := Config{
		Seed:      12345,
		Birth:     &draw[0],
		Bloodline: &Bloodline{Generation: 3, Sensitivity: 0.4},
		Talents:   []Talent{{Name: "测试", Desc: "x", Bonus: Effects{INT: 2}}},
		LLM:       Noop,
		MaxAge:    100,
	}.WithPoints(5, 5, 5, 5)
	res, err := Run(discard{}, cfg, evs, careers)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Age < 0 || res.Age > 100 {
		t.Fatalf("implausible death age %d", res.Age)
	}
	if res.Sensitivity < 0 || res.Sensitivity > 1 {
		t.Fatalf("ending sensitivity out of range: %f", res.Sensitivity)
	}
	if len(res.History) == 0 {
		t.Fatal("no history recorded")
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func TestNomadQuitsWhenBroke(t *testing.T) {
	// momus P1-2 regression: quit_if_stat "mny" was silently dead.
	c := Career{QuitIfStat: "mny", QuitBelow: 0.5}
	if !careerQuitCheck(&c, Stats{MNY: 0.1}) {
		t.Fatal("nomad should lose the track when MNY < 0.5")
	}
	if careerQuitCheck(&c, Stats{MNY: 1}) {
		t.Fatal("nomad should keep the track when MNY >= 0.5")
	}
}

func TestCultEventsGatedBehindFact(t *testing.T) {
	evs, err := LoadEvents()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	s := Stats{CHR: 5, INT: 5, STR: 5, MNY: 5, SPR: 5}
	rng := rand.New(rand.NewSource(1))
	// Without the cult fact, no cult event may ever fire.
	for age := 0; age <= 100; age++ {
		for roll := 0; roll < 50; roll++ {
			ev := PickEvent(evs, age, s, UnemployedID, Facts{}, nil, rng, 0, false)
			if ev != nil && hasRequires(ev, "cult") {
				t.Fatalf("cult event %s fired without cult fact (age %d)", ev.ID, age)
			}
		}
	}
	// With the fact, cult events become reachable.
	facts := Facts{"cult": true}
	found := false
	for age := 0; age <= 18 && !found; age++ {
		for roll := 0; roll < 200 && !found; roll++ {
			ev := PickEvent(evs, age, s, UnemployedID, facts, nil, rng, 0, false)
			if ev != nil && hasRequires(ev, "cult") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("cult fact set but no cult event reachable")
	}
}

func TestLovingHomeConflictsWithCult(t *testing.T) {
	evs, err := LoadEvents()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	var loved *Event
	for i := range evs {
		if evs[i].ID == "loved_child" {
			loved = &evs[i]
		}
	}
	if loved == nil {
		t.Fatal("loved_child missing")
	}
	s := Stats{SPR: 9}
	if loved.eligible(3, s, "", Facts{"cult": true}) {
		t.Fatal("loved_child fired inside a cult household")
	}
	if !loved.eligible(3, s, "", Facts{}) {
		t.Fatal("loved_child blocked in an ordinary family")
	}
}

func TestLifetimeNoRepeat(t *testing.T) {
	evs, err := LoadEvents()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	s := Stats{CHR: 5, INT: 5, STR: 5, MNY: 5, SPR: 5}
	rng := rand.New(rand.NewSource(7))
	used := map[string]bool{}
	repeat := ""
	for age := 0; age <= 90; age++ {
		ev := PickEvent(evs, age, s, UnemployedID, Facts{}, used, rng, 0, false)
		if ev == nil {
			continue
		}
		if used[ev.ID] && repeat == "" {
			repeat = ev.ID
		}
		used[ev.ID] = true
	}
	if repeat != "" {
		t.Fatalf("event fired twice in one life: %s", repeat)
	}
}

func TestFactsFromBirth(t *testing.T) {
	if f := NewFacts(&Birth{ID: "cult_family"}); !f["cult"] {
		t.Fatal("cult_family birth must preset the cult fact")
	}
	if f := NewFacts(&Birth{ID: "rural"}); len(f) != 0 {
		t.Fatal("ordinary birth must not preset facts")
	}
	if NewFacts(nil) == nil {
		t.Fatal("nil birth should still yield an empty non-nil map")
	}
}

func hasRequires(e *Event, fact string) bool {
	for _, r := range e.Requires {
		if r == fact {
			return true
		}
	}
	return false
}

// TestCultLifeCoherence plays a full life born into the cult family and
// asserts the storyline stays coherent: no loving-home events, no ordinary
// social events while isolated, no duplicated lines.
func TestCultLifeCoherence(t *testing.T) {
	evs, err := LoadEvents()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	careers, _ := LoadCareers()
	for seed := int64(1); seed <= 30; seed++ {
		cfg := Config{
			Seed:      seed,
			Birth:     &Birth{ID: "cult_family", Name: "邪教家庭"},
			Bloodline: &Bloodline{},
			LLM:       Noop,
			MaxAge:    100,
		}.WithPoints(5, 5, 5, 5)
		var out bytes.Buffer
		res, err := Run(&out, cfg, evs, careers)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		text := out.String()
		if strings.Contains(text, "掌上明珠") {
			t.Fatalf("seed %d: loving-home event inside cult household", seed)
		}
		// Count duplicates (same event text twice = incoherent repetition).
		lines := map[string]int{}
		for _, h := range res.History {
			key := strings.TrimLeft(h, "[0123456789 ]岁")
			key = strings.TrimSpace(strings.SplitN(key, "] ", 2)[len(strings.SplitN(key, "] ", 2))-1])
			lines[key]++
		}
		for k, n := range lines {
			if n > 1 && k != "" {
				t.Fatalf("seed %d: repeated line %q (%dx)", seed, k, n)
			}
		}
	}
}

func TestDrawTalentsWeightedAndPity(t *testing.T) {
	talents, err := LoadTalents()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	rng := rand.New(rand.NewSource(99))
	sawLegendary := false
	for draw := 0; draw < 200; draw++ {
		hand := DrawTalents(talents, 10, rng)
		if len(hand) != 10 {
			t.Fatalf("hand size %d", len(hand))
		}
		seen := map[string]bool{}
		hasRare := false
		for _, tal := range hand {
			if seen[tal.Name] {
				t.Fatalf("duplicate talent in hand: %s", tal.Name)
			}
			seen[tal.Name] = true
			if isRareOrBetter(tal) {
				hasRare = true
			}
			if tal.Rarity == "legendary" {
				sawLegendary = true
			}
		}
		if !hasRare {
			t.Fatal("pity broken: hand without any rare+ talent")
		}
	}
	if !sawLegendary {
		t.Fatal("legendary never appeared in 200 draws — weight too low or bug")
	}
	// n > pool must clamp, not panic.
	if got := DrawTalents(talents[:1], 5, rng); len(got) != 1 {
		t.Fatalf("clamp broken: %d", len(got))
	}
}

func TestRarityStars(t *testing.T) {
	cases := map[string]string{"legendary": "***", "epic": "** ", "rare": "*  ", "": "   ", "weird": "   "}
	for r, want := range cases {
		if got := RarityStars(r); got != want {
			t.Errorf("RarityStars(%q)=%q want %q", r, got, want)
		}
	}
}
