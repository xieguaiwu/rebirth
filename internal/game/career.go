package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
)

// Career is a life track entered at a decision window and kept while its
// conditions hold. Both mundane tracks and extreme tracks (star, model,
// sex worker, CEO, genius scientist, cult leader) live in this table.
type Career struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Desc   string  `json:"desc"`
	MinAge int     `json:"min_age"`
	MaxAge int     `json:"max_age"` // forced retirement beyond this age (0 = none)
	Weight float64 `json:"weight"`  // base entry weight among eligible tracks
	Cond   *Cond   `json:"cond,omitempty"`
	// ExtraCond gates extreme tracks on hidden state.
	RequiresTraumaMin float64 `json:"requires_trauma_min,omitempty"` // e.g. cult leader
	// Yearly drift while employed.
	YearlyDelta Effects `json:"yearly_delta"`
	// YearlyShock is a small recurring trauma exposure (exploitation,
	// burnout, stage fear). 0 = clean track.
	YearlyShock float64 `json:"yearly_shock"`
	// QuitIfBelow makes the holder lose this track when the attribute
	// falls under the value ("str" / "chr" / "int" / "spr").
	QuitIfStat string  `json:"quit_if_stat,omitempty"`
	QuitBelow  float64 `json:"quit_below,omitempty"`
}

func (c *Career) eligible(age int, s Stats, traumaLoad float64) bool {
	if age < c.MinAge {
		return false
	}
	if c.MaxAge > 0 && age > c.MaxAge {
		return false
	}
	if c.RequiresTraumaMin > 0 && traumaLoad < c.RequiresTraumaMin {
		return false
	}
	return c.Cond == nil || condMet(c.Cond, s)
}

func condMet(c *Cond, s Stats) bool {
	if c == nil {
		return true
	}
	return inRange(s.CHR, c.MinCHR, c.MaxCHR) &&
		inRange(s.INT, c.MinINT, c.MaxINT) &&
		inRange(s.STR, c.MinSTR, c.MaxSTR) &&
		inRange(s.MNY, c.MinMNY, c.MaxMNY)
}

// UnemployedID marks the fallback track.
const UnemployedID = "none"

// LoadCareers parses the embedded career table.
func LoadCareers() ([]*Career, error) {
	raw, err := eventsFS.ReadFile("data/careers.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded careers: %w", err)
	}
	var cs []*Career
	if err := json.Unmarshal(raw, &cs); err != nil {
		return nil, fmt.Errorf("parse careers.json: %w", err)
	}
	if len(cs) == 0 {
		return nil, fmt.Errorf("careers.json contains no careers")
	}
	return cs, nil
}

// PickCareer samples one eligible track at a decision window. Falls back to
// unemployment. luck shifts weights toward glamorous tracks (Good flag is
// reused via positive YearlyDelta.MNY).
func PickCareer(cs []*Career, age int, s Stats, traumaLoad float64, rng *rand.Rand, luck float64) *Career {
	total := 0.0
	for _, c := range cs {
		if !c.eligible(age, s, traumaLoad) {
			continue
		}
		w := c.Weight * (1 + 0.5*luck)
		if w > 0 {
			total += w
		}
	}
	if total <= 0 {
		return nil
	}
	r := rng.Float64() * total
	for _, c := range cs {
		if !c.eligible(age, s, traumaLoad) {
			continue
		}
		w := c.Weight * (1 + 0.5*luck)
		if w <= 0 {
			continue
		}
		r -= w
		if r <= 0 {
			return c
		}
	}
	last := cs[len(cs)-1]
	if last.eligible(age, s, traumaLoad) {
		return last
	}
	return nil
}

// Birth is a starting background drawn at character creation. It shapes the
// initial attributes and shifts the inherited trauma baseline — being born
// into war or slums already loads the HPA axis before any personal event.
type Birth struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Desc   string  `json:"desc"`
	Weight float64 `json:"weight"`
	Bonus  Effects `json:"bonus"`
	// SensitivityAdd shifts the heritable HPA baseline upward at birth.
	SensitivityAdd float64 `json:"sensitivity_add"`
}

// LoadBirths parses the embedded birth table.
func LoadBirths() ([]Birth, error) {
	raw, err := eventsFS.ReadFile("data/births.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded births: %w", err)
	}
	var bs []Birth
	if err := json.Unmarshal(raw, &bs); err != nil {
		return nil, fmt.Errorf("parse births.json: %w", err)
	}
	if len(bs) == 0 {
		return nil, fmt.Errorf("births.json contains no entries")
	}
	return bs, nil
}

// DrawBirths samples n distinct weighted births for the choice menu.
func DrawBirths(bs []Birth, n int, rng *rand.Rand) []Birth {
	pool := make([]Birth, len(bs))
	copy(pool, bs)
	out := make([]Birth, 0, n)
	for len(out) < n && len(pool) > 0 {
		total := 0.0
		for _, b := range pool {
			total += b.Weight
		}
		r := rng.Float64() * total
		pick := 0
		for i, b := range pool {
			r -= b.Weight
			if r <= 0 {
				pick = i
				break
			}
		}
		out = append(out, pool[pick])
		pool = append(pool[:pick], pool[pick+1:]...)
	}
	return out
}
