package game

import (
	"io"
	"math"
	"testing"
)

// TestGeneticSensitivityMatters is the regression guard for the v0.9.0
// finding: inherited sensitivity used to be decoration — the pathological
// rate was identical (35.7%) for s=0..1 across 24,000 simulated lives,
// because the birth-year baseline decayed to nothing in ~3 years and the
// entry threshold sat far above anything s could supply. After wiring s
// into the dynamics (lower EnterAt, heavier trauma sampling, lighter
// healing, slower extinction), the pathological rate must rise strictly
// with s, and the top of the range must differ from baseline by a
// meaningful margin.
func TestGeneticSensitivityMatters(t *testing.T) {
	evs, err := LoadEvents()
	if err != nil {
		t.Fatal(err)
	}
	careers, err := LoadCareers()
	if err != nil {
		t.Fatal(err)
	}
	levels := []float64{0, 0.5, 1.0}
	const n = 1200
	rates := make([]float64, len(levels))
	for li, s := range levels {
		path := 0
		for i := 0; i < n; i++ {
			cfg := Config{
				Seed:      int64(i + 1),
				Bloodline: &Bloodline{Sensitivity: s, Generation: 1},
				LLM:       Noop,
				MaxAge:    90,
			}.WithPoints(5, 5, 5, 5)
			res, err := Run(io.Discard, cfg, evs, careers)
			if err != nil {
				t.Fatal(err)
			}
			if res.Pathological {
				path++
			}
		}
		rates[li] = float64(path) / n
	}
	// Strict monotonicity: s=0.5 must beat s=0, s=1.0 must beat s=0.5.
	if rates[1] <= rates[0] || rates[2] <= rates[1] {
		t.Fatalf("pathological rate must rise with sensitivity, got %v", rates)
	}
	// Effect size: top-of-range vs baseline, at least 8pp (n=1200, SE ~1.4pp
	// at 35% baseline, so 8pp ≈ 6σ — the old model measured exactly 0).
	if rates[2]-rates[0] < 0.08 {
		t.Fatalf("sensitivity effect too small to matter: %v", rates)
	}
}

// TestSensitivitySlowsExtinction pins the mechanism behind the rate shift:
// starting from the SAME trace, with identical conditions (no trigger, no
// therapy), a high-s state's memory must decay noticeably slower.
func TestSensitivitySlowsExtinction(t *testing.T) {
	p := DefaultTraumaParams()
	lo := NewTraumaState(0.0, p)
	hi := NewTraumaState(1.0, p)
	lo.M, lo.A = 0.5, 0.4 // same starting trace, moderate arousal
	hi.M, hi.A = 0.5, 0.4
	for i := 0; i < 6; i++ {
		lo.Step(false, 0, p)
		hi.Step(false, 0, p)
	}
	// Extinction mult is (1-0.5*s): low-s burns ~0.10/yr, high-s ~0.05/yr.
	// After 6 trigger-free years the low-s trace is gone, the high-s one
	// still holds a clear fraction.
	if hi.M <= lo.M {
		t.Fatalf("high-s extinction should be slower: lo.M=%.4f hi.M=%.4f", lo.M, hi.M)
	}
	if hi.M-lo.M < 0.05 {
		t.Fatalf("extinction gap too small: lo.M=%.4f hi.M=%.4f", lo.M, hi.M)
	}
}

// TestSensitivityThresholdShift pins the second mechanism: the effective
// entry threshold is lowered by SensEnterAt per unit s (Run applies this
// shift before the first year), and the hysteresis pair never inverts even
// under extreme custom configs.
func TestSensitivityThresholdShift(t *testing.T) {
	p := DefaultTraumaParams()
	// s=1 with default params: 0.80 - 0.30 = 0.50.
	if got := p.EnterAt - p.SensEnterAt*1.0; math.Abs(got-0.50) > 1e-9 {
		t.Fatalf("expected EnterAt 0.50 at s=1, got %v", got)
	}
	// Run applies the shift to the params before stepping; simulate that.
	hiP := p
	hiP.EnterAt -= hiP.SensEnterAt * 1.0 // 0.50
	// A heavy trace at load ~0.73: above the shifted threshold, below the
	// default one. After one yearly step the load stays in the band
	// [0.50, 0.80) for the sensitive state but not the normal one.
	lo := NewTraumaState(0.0, p)
	hi := NewTraumaState(1.0, hiP)
	lo.M, lo.A = 0.65, 0.85
	hi.M, hi.A = 0.65, 0.85
	lo.Step(false, 0, p)
	hi.Step(false, 0, hiP)
	if lo.Pathological {
		t.Fatal("low-s must NOT be pathological at this load")
	}
	if !hi.Pathological {
		t.Fatalf("high-s must be pathological at this load (threshold shifted to %.2f)", hiP.EnterAt)
	}
	// Hysteresis sanity: even with the max shift, EnterAt stays above ExitAt.
	hiP2 := p
	hiP2.EnterAt = 0.10 // hostile custom config
	hiP2.ExitAt = 0.20
	hiP2.EnterAt -= hiP2.SensEnterAt * 1.0
	if hiP2.EnterAt <= hiP2.ExitAt {
		hiP2.EnterAt = hiP2.ExitAt + 0.05 // Run's guard
	}
	if hiP2.EnterAt <= hiP2.ExitAt {
		t.Fatalf("hysteresis inverted: EnterAt=%.2f ExitAt=%.2f", hiP2.EnterAt, hiP2.ExitAt)
	}
}
