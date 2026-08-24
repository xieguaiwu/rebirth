// Package game implements the life-restart simulation core.
//
// The trauma subsystem is a game-scale adaptation of a computational
// psychiatry model of traumatic memory dynamics (leaky-integrator memory
// trace, amygdala/prefrontal coupling, reconsolidation positive feedback,
// saddle-node bifurcation with hysteresis).
package game

import "math"

// TraumaParams holds the coupled ODE parameters, discretized per year (dt=1).
// Ranges are chosen so yearly updates stay within [0,1] without extra clamps
// except where noted.
type TraumaParams struct {
	// Memory decay rate (per year) of the trauma trace m.
	Beta float64 // 0.06
	// Amygdala reactivity relaxation: da = -MuA*(a - AStar) + drive.
	MuA   float64 // 0.25
	AStar float64 // 0.15 baseline reactivity
	// Prefrontal control relaxation: dp = -MuP*(p - PStar) - NuP*(da)^2*p.
	MuP   float64 // 0.20
	PStar float64 // 0.70 baseline control
	NuP   float64 // 0.60 prefrontal erosion by amygdala deviation
	// Reconsolidation gain: high arousal at recall strengthens the trace.
	BetaRe float64 // 0.45
	// Extinction-learning bonus applied when a year passes with no trigger
	// and low arousal.
	Extinction float64 // 0.10
	// Weights composing the scalar trauma load from m and a.
	LoadM float64 // 0.6
	LoadA float64 // 0.4
	// Drive is the memory-driven amygdala excitation gain per year.
	// v0.7.2 calibration: 0.9 → 0.65 (was the dominant latch lever).
	Drive float64 // 0.65
	// EventScale is the game-scale calibration of event trauma intensity
	// (authored alpha range 0.05-0.65 is clinical-scale; at game scale it
	// saturated the trace — v0.7.2 balance fix). Applied to EVENT shocks
	// only; career yearly exposure and LLM fate events keep their own
	// smaller ranges.
	EventScale float64 // 0.5
	// Bifurcation thresholds (hysteresis pair).
	EnterAt float64 // Mc2 = 0.80 pathological attractor entry
	ExitAt  float64 // Mc1 = 0.35 exit threshold (< EnterAt)

	// Inherited-sensitivity couplings (v0.9.0). Previously sensitivity only
	// shifted the birth-year baseline, which decayed to nothing within ~3
	// years (extinction term dominates) — measured: pathological rate was
	// identical for s=0..1 across 24k lives. These four couplings make the
	// heritable trait a real bias on the dynamics instead of decoration.
	// All four are per-unit-s (s is clamped to [0,1]).
	SensEnterAt float64 // 0.30: s lowers the pathological-entry threshold
	SensTraumaW float64 // 0.40: s raises trauma-event sampling weight
	SensHealW   float64 // 0.40: s lowers healing/therapy event weight
	SensExtinct float64 // 0.50: s slows extinction learning (harder to heal)
}

// DefaultTraumaParams returns the tuned default parameter set.
func DefaultTraumaParams() TraumaParams {
	return TraumaParams{
		Beta:        0.06,
		MuA:         0.25,
		AStar:       0.15,
		MuP:         0.20,
		PStar:       0.70,
		NuP:         0.60,
		BetaRe:      0.45,
		Extinction:  0.10,
		LoadM:       0.6,
		LoadA:       0.4,
		Drive:       0.65,
		EventScale:  0.5,
		EnterAt:     0.80,
		ExitAt:      0.35,
		SensEnterAt: 0.30,
		SensTraumaW: 0.40,
		SensHealW:   0.40,
		SensExtinct: 0.50,
	}
}

// TraumaState is the individual-level coupled state.
type TraumaState struct {
	M            float64 // trauma memory trace, [0,1], leaky integrator
	A            float64 // amygdala reactivity, [0,1]
	P            float64 // prefrontal inhibitory control, [0,1]
	S            float64 // inherited HPA sensitivity, [0,1] (kept for the v0.9.0 couplings)
	Pathological bool    // inside the pathological attractor (hysteresis latch)
}

// NewTraumaState creates an initial state with baseline values shifted by
// inherited HPA sensitivity s in [0,1]. s is also retained on the state so
// the yearly dynamics (extinction, thresholds) can couple to it.
func NewTraumaState(inheritedSensitivity float64, p TraumaParams) *TraumaState {
	s := Clamp01(inheritedSensitivity)
	m := 0.05 + 0.30*s
	a := p.AStar + 0.35*s
	return &TraumaState{M: Clamp01(m), A: Clamp01(a), P: p.PStar, S: s}
}

// Load computes the scalar trauma-load order parameter M = wM*m + wA*a.
func (t *TraumaState) Load(p TraumaParams) float64 {
	return p.LoadM*t.M + p.LoadA*t.A
}

// Shock applies a traumatic event of intensity alpha in [0,1].
//
// Encoding jump: m <- m + alpha*(1-m) (bounded, never exceeds 1).
// Reconsolidation feedback: recall under high arousal multiplies the trace
// by (1 + BetaRe*a), clamped to [0,1]. High arousal here means the current
// amygdala reactivity a.
func (t *TraumaState) Shock(alpha float64, p TraumaParams) {
	alpha = clamp(alpha, 0, 1)
	t.M = Clamp01(t.M + alpha*(1-t.M))
	// Mild reconsolidation boost: single shocks wound, only repeated
	// adversity under arousal crosses the bifurcation.
	t.M = Clamp01(t.M * (1 + 0.25*p.BetaRe*t.A*(1+alpha)))
}

// Step advances the coupled system by one year. trigger reports whether this
// year contained a cue resembling past trauma (any trauma-tagged event).
// Therapy quality q in [0,1] adds extinction learning proportional to q.
func (t *TraumaState) Step(trigger bool, therapyQ float64, p TraumaParams) {
	// --- memory trace ---
	dm := -p.Beta * t.M
	extinctMult := 1 - p.SensExtinct*Clamp01(t.S) // v0.9.0: sensitive lineages heal slower
	if !trigger {
		if t.A < 0.55 {
			// Extinction needs a safe-enough context: no cue and moderate
			// arousal. Therapy amplifies it.
			dm -= p.Extinction * (1 + therapyQ) * extinctMult
		}
	} else {
		// Cue exposure mildly reconsolidates the fear memory, but therapy
		// still does partial work during exposure sessions.
		t.M = Clamp01(t.M * (1 + 0.15*p.BetaRe*t.A))
		dm -= p.Extinction * 0.3 * therapyQ * extinctMult
	}
	t.M = Clamp01(t.M + dm)

	aPrev := t.A

	// --- amygdala reactivity ---
	drive := p.Drive * t.M // memory-driven excitation
	t.A = Clamp01(t.A - p.MuA*(t.A-p.AStar) + drive - 0.8*t.P*t.A)

	// --- prefrontal control: eroded by squared amygdala deviation ---
	dev := t.A - aPrev
	if dev < 0 {
		dev = -dev
	}
	t.P = Clamp01(t.P - p.MuP*(t.P-p.PStar) - p.NuP*dev*dev*t.P)

	// --- bifurcation latch (saddle-node pair with hysteresis) ---
	load := t.Load(p)
	if !t.Pathological && load >= p.EnterAt {
		t.Pathological = true
	}
	if t.Pathological && load < p.ExitAt {
		t.Pathological = false
	}
}

// InheritSensitivity computes the child's inherited HPA sensitivity from the
// parent's ending sensitivity using the sub-additive epigenetic kernel
// s_child = clip(psi*s_parent + eta, 0, smax). psi < 1 guarantees decay
// across generations unless trauma recurs; eta is zero-mean noise.
func InheritSensitivity(parentS, eta, psi float64) float64 {
	return Clamp01(psi*clamp(parentS, 0, 1) + clamp(eta, -0.2, 0.2))
}

// EndingSensitivity maps the final state to the heritable scalar used by the
// next generation. Pathological runs transmit more than the raw trace alone
// (the attractor itself is partially heritable via parenting behavior).
func EndingSensitivity(t *TraumaState, p TraumaParams) float64 {
	s := 0.7*t.Load(p) + 0.3*t.M
	if t.Pathological {
		s += 0.15
	}
	return Clamp01(s)
}

func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }
func Clamp01(x float64) float64       { return clamp(x, 0, 1) }
