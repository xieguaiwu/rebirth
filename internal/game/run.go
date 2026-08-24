package game

import (
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"strings"
)

// Noop is the disabled-narrator sentinel; main passes it when LLM is off.
var Noop Narrator = noop{}

type noop struct{}

func (noop) Narrate(age int, summary, fallback string) string { return fallback }
func (noop) FateEvent(age int, summary string) (Event, bool)  { return Event{}, false }
func (noop) Epitaph(summary string) string                    { return "一生至此。" }
func (noop) Broken() bool                                     { return false }

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
	// Broken reports whether the narrator's failure breaker has tripped
	// (channel dead or chronically too slow); Run prints one notice and
	// further calls fail soft instantly.
	Broken() bool
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

	// Trauma overrides the built-in dynamics parameters; nil = defaults.
	Trauma *TraumaParams
	// NarrateRatio is the fraction of trauma/good events sent to the LLM
	// narrator (deterministic per event ID, 0..1). 1 = narrate everything
	// (old behavior), 0.5 = every other event, 0 = never (still falls
	// back to the pool text). Default 1.
	NarrateRatio float64

	// Step turns on manual advance: after every yearly line, Pause is
	// called; returning true aborts the life gracefully. Nil-safe.
	Step  bool
	Pause func() bool

	// Hints enables transient terminal indicators (e.g. "LLM thinking")
	// shown while a blocking narrator call is in flight. Set when stdout
	// is an interactive TTY; indicators would otherwise pollute pipes.
	Hints bool
}

const hintPending = "\033[2m…… 命运编织中，稍等片刻 \033[0m"

// clearHint erases the pending-indicator once real content arrives.
func clearHint(w io.Writer, hints bool) {
	if hints {
		fmt.Fprint(w, "\r\033[K")
	}
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

// Run plays one life end-to-end, emitting progress lines to w. It is a
// thin driver over Session (session.go) so the mobile daemon can step and
// checkpoint the exact same simulation; the output is byte-identical to
// the pre-session implementation (the existing tests are the baseline).
func Run(w io.Writer, cfg Config, evs []Event, careers []*Career) (*Result, error) {
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 100
	}
	sess := NewSession(cfg, evs, careers)
	sess.Out = w

	fmt.Fprintf(w, "\n════ 第 %d 代 · 种子 %d ════\n", sess.Gen, sess.Cfg.Seed)
	if sess.Cfg.Birth != nil {
		fmt.Fprintf(w, "[出身] %s —— %s\n", sess.Cfg.Birth.Name, sess.Cfg.Birth.Desc)
	}
	if sess.Sens > 0.05 {
		fmt.Fprintf(w, "[血脉] 应激敏感性基线 %.2f（高于此值更易受创）\n", Clamp01(sess.Sens))
	}
	for _, t := range sess.Cfg.Talents {
		fmt.Fprintf(w, "[天赋] %s —— %s\n", t.Name, t.Desc)
	}
	if sess.Cfg.InheritTal != nil {
		fmt.Fprintf(w, "[血脉天赋] %s —— %s\n", sess.Cfg.InheritTal.Name, sess.Cfg.InheritTal.Desc)
	}

	for !sess.Done() {
		sess.Advance() // prints its own lines to w (record -> Out)

		// Manual advance: one Enter per year of life. Positioned between
		// Advance and DeathCheck exactly as in the original Run (the
		// pause used to sit between trauma.Step and the death accounting).
		if cfg.Step && cfg.Pause != nil && cfg.Pause() {
			sess.Aborted = true
			sess.DeathAge = sess.Age - 1
			fmt.Fprintf(w, "\n──── 玩家中途离开（%d 岁）────\n", sess.DeathAge)
			return sess.Result(), nil
		}

		if sess.DeathCheck() {
			sess.Finish()
			fmt.Fprintf(w, "\n──── 人生结束：%d 岁 · 职业：%s · %s ────\n", sess.DeathAge, sess.CareerName, sess.DeathStatus)
			if !isNoop(sess.Cfg.LLM) {
				// Byte-identical to the pre-session Run: the pending hint is
				// printed before the (blocking) epitaph call and cleared after.
				if cfg.Hints {
					fmt.Fprint(w, hintPending)
				}
				clearHint(w, cfg.Hints)
				fmt.Fprintln(w, "墓志铭："+sess.FinishEpitaph())
			}
			break
		}
	}
	return sess.Result(), nil
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

// narrateSample decides deterministically — by event ID hash, consuming NO
// RNG — whether this event gets LLM narration. Ratio 0 = default (narrate
// everything, old behavior); 1 = everything; 0.5 = half. The budget would
// otherwise be eaten by narrate calls alone before the epitaph (v0.7.4).
func narrateSample(id string, ratio float64) bool {
	if ratio <= 0 {
		return true // unset: keep the original always-narrate behavior
	}
	if ratio >= 1 {
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return float64(h.Sum32())/float64(math.MaxUint32) < ratio
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
