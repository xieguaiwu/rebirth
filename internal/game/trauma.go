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
	// Bifurcation thresholds (hysteresis pair).
	EnterAt float64 // Mc2 = 0.70 pathological attractor entry
	ExitAt  float64 // Mc1 = 0.35 exit threshold (< EnterAt)
}

// DefaultTraumaParams returns the tuned default parameter set.
func DefaultTraumaParams() TraumaParams {
	return TraumaParams{
		Beta:       0.06,
		MuA:        0.25,
		AStar:      0.15,
		MuP:        0.20,
		PStar:      0.70,
		NuP:        0.60,
		BetaRe:     0.45,
		Extinction: 0.10,
		LoadM:      0.6,
		LoadA:      0.4,
		EnterAt:    0.70,
		ExitAt:     0.35,
	}
}

// TraumaState is the individual-level coupled state.
type TraumaState struct {
	M            float64 // trauma memory trace, [0,1], leaky integrator
	A            float64 // amygdala reactivity, [0,1]
	P            float64 // prefrontal inhibitory control, [0,1]
	Pathological bool    // inside the pathological attractor (hysteresis latch)
	// LowStreak counts consecutive years without any trauma trigger;
	// used to gate extinction learning (safe-context requirement).
	LowStreak int
}

// NewTraumaState creates an initial state with baseline values shifted by
// inherited HPA sensitivity s in [0,1].
func NewTraumaState(inheritedSensitivity float64, p TraumaParams) *TraumaState {
	m := 0.05 + 0.30*inheritedSensitivity
	a := p.AStar + 0.35*inheritedSensitivity
	return &TraumaState{M: clamp01(m), A: clamp01(a), P: p.PStar}
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
	t.M = clamp01(t.M + alpha*(1-t.M))
	// Mild reconsolidation boost: single shocks wound, only repeated
	// adversity under arousal crosses the bifurcation.
	t.M = clamp01(t.M * (1 + 0.25*p.BetaRe*t.A*(1+alpha)))
	t.LowStreak = 0
}

// Step advances the coupled system by one year. trigger reports whether this
// year contained a cue resembling past trauma (any trauma-tagged event).
// Therapy quality q in [0,1] adds extinction learning proportional to q.
func (t *TraumaState) Step(trigger bool, therapyQ float64, p TraumaParams) {
	// --- memory trace ---
	dm := -p.Beta * t.M
	if !trigger {
		if t.A < 0.55 {
			// Extinction needs a safe-enough context: no cue and moderate
			// arousal. Therapy amplifies it.
			dm -= p.Extinction * (1 + therapyQ)
			t.LowStreak++
		}
	} else {
		// Cue exposure mildly reconsolidates the fear memory, but therapy
		// still does partial work during exposure sessions.
		t.M = clamp01(t.M * (1 + 0.15*p.BetaRe*t.A))
		dm -= p.Extinction * 0.3 * therapyQ
	}
	t.M = clamp01(t.M + dm)

	aPrev := t.A

	// --- amygdala reactivity ---
	drive := 0.9 * t.M // memory-driven excitation
	t.A = clamp01(t.A - p.MuA*(t.A-p.AStar) + drive - 0.8*t.P*t.A)

	// --- prefrontal control: eroded by squared amygdala deviation ---
	dev := t.A - aPrev
	if dev < 0 {
		dev = -dev
	}
	t.P = clamp01(t.P - p.MuP*(t.P-p.PStar) - p.NuP*dev*dev*t.P)

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
	return clamp01(psi*clamp(parentS, 0, 1) + clamp(eta, -0.2, 0.2))
}

// EndingSensitivity maps the final state to the heritable scalar used by the
// next generation. Pathological runs transmit more than the raw trace alone
// (the attractor itself is partially heritable via parenting behavior).
func EndingSensitivity(t *TraumaState, p TraumaParams) float64 {
	s := 0.7*t.Load(p) + 0.3*t.M
	if t.Pathological {
		s += 0.15
	}
	return clamp01(s)
}

func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }
func clamp01(x float64) float64       { return clamp(x, 0, 1) }
