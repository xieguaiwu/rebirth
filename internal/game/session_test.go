package game

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// runViaSession drives the same configuration through a Session instead of
// Run and returns the output + history. Output must be byte-identical.
func runViaSession(cfg Config, evs []Event, careers []*Career) (string, []string, *Result) {
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 100
	}
	sess := NewSession(cfg, evs, careers)
	var buf bytes.Buffer
	sess.Out = &buf

	fmt.Fprintf(&buf, "\n════ 第 %d 代 · 种子 %d ════\n", sess.Gen, sess.Cfg.Seed)
	if sess.Cfg.Birth != nil {
		fmt.Fprintf(&buf, "[出身] %s —— %s\n", sess.Cfg.Birth.Name, sess.Cfg.Birth.Desc)
	}
	if sess.Sens > 0.05 {
		fmt.Fprintf(&buf, "[血脉] 应激敏感性基线 %.2f（高于此值更易受创）\n", Clamp01(sess.Sens))
	}
	for _, t := range sess.Cfg.Talents {
		fmt.Fprintf(&buf, "[天赋] %s —— %s\n", t.Name, t.Desc)
	}
	if sess.Cfg.InheritTal != nil {
		fmt.Fprintf(&buf, "[血脉天赋] %s —— %s\n", sess.Cfg.InheritTal.Name, sess.Cfg.InheritTal.Desc)
	}

	for !sess.Done() {
		sess.Advance()
		if sess.DeathCheck() {
			sess.Finish()
			fmt.Fprintf(&buf, "\n──── 人生结束：%d 岁 · 职业：%s · %s ────\n", sess.DeathAge, sess.CareerName, sess.DeathStatus)
			if !isNoop(sess.Cfg.LLM) {
				fmt.Fprintln(&buf, "墓志铭："+sess.EpitaphText)
			}
			break
		}
	}
	return buf.String(), sess.History, sess.Result()
}

// TestRunSessionByteIdentical proves the session refactor did not change
// CLI output: Run and a Session-driven loop must emit identical bytes for
// the same configuration, across several seeds.
func TestRunSessionByteIdentical(t *testing.T) {
	evs, err := LoadEvents()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	careers, err := LoadCareers()
	if err != nil {
		t.Fatalf("careers: %v", err)
	}
	births, err := LoadBirths()
	if err != nil {
		t.Fatalf("births: %v", err)
	}
	for _, seed := range []int64{1, 42, 12345, 999983} {
		draw := DrawBirths(births, 3, rand.New(rand.NewSource(seed)))
		talents := DrawTalents(mustTalents(t), 3, rand.New(rand.NewSource(seed+1)))
		cfg := Config{
			Seed:         seed,
			Birth:        &draw[0],
			Bloodline:    &Bloodline{Generation: 2, Sensitivity: float64(seed%10) / 10},
			Talents:      talents,
			LLM:          Noop,
			MaxAge:       100,
			NarrateRatio: 0.5,
		}.WithPoints(5, 5, 5, 5)

		var w1 bytes.Buffer
		res1, err := Run(&w1, cfg, evs, careers)
		if err != nil {
			t.Fatalf("seed %d run: %v", seed, err)
		}
		out2, hist2, res2 := runViaSession(cfg, evs, careers)
		if w1.String() != out2 {
			t.Fatalf("seed %d: Run output != Session output\n--- Run ---\n%s\n--- Session ---\n%s", seed, w1.String(), out2)
		}
		if len(res1.History) != len(hist2) {
			t.Fatalf("seed %d: history length %d != %d", seed, len(res1.History), len(hist2))
		}
		for i := range res1.History {
			if res1.History[i] != hist2[i] {
				t.Fatalf("seed %d: history[%d] mismatch: %q != %q", seed, i, res1.History[i], hist2[i])
			}
		}
		if res1.Age != res2.Age || res1.Career != res2.Career || res1.Pathological != res2.Pathological {
			t.Fatalf("seed %d: result mismatch: Run=%+v Session=%+v", seed, res1, res2)
		}
		if fmt.Sprintf("%.6f", res1.Sensitivity) != fmt.Sprintf("%.6f", res2.Sensitivity) {
			t.Fatalf("seed %d: sensitivity %.6f != %.6f", seed, res1.Sensitivity, res2.Sensitivity)
		}
	}
}

// TestSessionDeterministicGolden locks the full-life history of a fixed
// seed to a hash. Any change to the deterministic core (sampling order,
// dynamics, data) breaks this test — the mobile daemon relies on exact
// reproducibility for checkpoint replay and cross-platform parity.
func TestSessionDeterministicGolden(t *testing.T) {
	evs, err := LoadEvents()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	careers, err := LoadCareers()
	if err != nil {
		t.Fatalf("careers: %v", err)
	}
	births, err := LoadBirths()
	if err != nil {
		t.Fatalf("births: %v", err)
	}
	draw := DrawBirths(births, 3, rand.New(rand.NewSource(20260824)))
	talents := DrawTalents(mustTalents(t), 3, rand.New(rand.NewSource(20260825)))
	cfg := Config{
		Seed:         20260824,
		Birth:        &draw[0],
		Bloodline:    &Bloodline{Generation: 1, Sensitivity: 0.2},
		Talents:      talents,
		LLM:          Noop,
		MaxAge:       100,
		NarrateRatio: 0.5,
	}.WithPoints(7, 6, 5, 2)

	_, hist, res := runViaSession(cfg, evs, careers)
	if res.Age <= 0 {
		t.Fatalf("life did not complete: %+v", res)
	}
	h := sha256.Sum256([]byte(joinLines(hist)))
	got := hex.EncodeToString(h[:])
	// Lock: recompute with the same seed must always match this value.
	const want = "643b6a749084ef2503244e59999493e1207d34fc951dee2daf53532f19d64792"
	if got != want {
		t.Fatalf("history hash changed: got %s want %s (deterministic core or data changed)", got, want)
	}

	// Same seed twice => identical history.
	_, hist2, _ := runViaSession(cfg, evs, careers)
	if joinLines(hist) != joinLines(hist2) {
		t.Fatal("same seed produced different histories")
	}
}

// TestEnglishDatasetValidates loads the English dataset and applies the
// same integrity gates as the Chinese one: fact reachability and career
// reference resolution.
func TestEnglishDatasetValidates(t *testing.T) {
	evs, err := LoadEventsLang("en")
	if err != nil {
		t.Fatalf("en events: %v", err)
	}
	careers, err := LoadCareersLang("en")
	if err != nil {
		t.Fatalf("en careers: %v", err)
	}
	births, err := LoadBirthsLang("en")
	if err != nil {
		t.Fatalf("en births: %v", err)
	}
	talents, err := LoadTalentsLang("en")
	if err != nil {
		t.Fatalf("en talents: %v", err)
	}
	if len(evs) == 0 || len(careers) == 0 || len(births) == 0 || len(talents) == 0 {
		t.Fatal("en dataset is empty")
	}
	// Same event count as the Chinese dataset (identity check).
	zhEvs, _ := LoadEvents()
	if len(evs) != len(zhEvs) {
		t.Fatalf("en event count %d != zh %d", len(evs), len(zhEvs))
	}

	// Fact reachability: every referenced fact can be produced.
	facts := map[string]bool{}
	for _, b := range births {
		for k := range NewFacts(&b) {
			facts[k] = true
		}
	}
	for _, e := range evs {
		if e.Sets != "" {
			facts[strings.TrimPrefix(e.Sets, "!")] = true
		}
		if e.Context != "" {
			facts[strings.TrimPrefix(e.Context, "!")] = true
		}
	}
	for _, e := range evs {
		for _, r := range e.Requires {
			if !facts[r] {
				t.Fatalf("en event %s requires unreachable fact %q", e.ID, r)
			}
		}
	}

	// Career references resolve.
	careerIDs := map[string]bool{UnemployedID: true}
	for _, c := range careers {
		careerIDs[c.ID] = true
	}
	for _, e := range evs {
		if e.Career != "" && !careerIDs[e.Career] {
			t.Fatalf("en event %s references unknown career %q", e.ID, e.Career)
		}
	}
}

func mustTalents(t *testing.T) []Talent {
	t.Helper()
	ts, err := LoadTalents()
	if err != nil {
		t.Fatalf("talents: %v", err)
	}
	return ts
}

func joinLines(lines []string) string {
	var b bytes.Buffer
	for _, l := range lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}
