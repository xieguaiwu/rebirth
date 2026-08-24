package game

import (
	"io"
	"regexp"
	"strconv"
	"testing"
)

// ageRe extracts the age from a recorded "[ 12 岁] ..." history line.
var ageRe = regexp.MustCompile(`\[\s*(\d+)\s*岁\]`)

// agesPresent returns the set of ages that produced a visible line.
func agesPresent(r *Result) map[int]bool {
	m := map[int]bool{}
	for _, h := range r.History {
		if sm := ageRe.FindStringSubmatch(h); sm != nil {
			a, _ := strconv.Atoi(sm[1])
			m[a] = true
		}
	}
	return m
}

// TestEarlyChildhoodNeverSilent: ages 1–3 must always produce a line,
// regardless of birth/seed. Regression for the "二岁的内容总是不出来"
// report: the ages-1–2 pool used to hold only loved_child + abandoned
// (abandoned needs 家境<=3), so once loved_child fired at 0/1 the year
// went blank for every ordinary player. v0.8.1 adds unconditional
// toddler events (first_words / first_steps / picture_book) to fill it.
func TestEarlyChildhoodNeverSilent(t *testing.T) {
	births, err := LoadBirths()
	if err != nil {
		t.Fatal(err)
	}
	evs, err := LoadEvents()
	if err != nil {
		t.Fatal(err)
	}
	careers, err := LoadCareers()
	if err != nil {
		t.Fatal(err)
	}
	for bi, b := range births {
		for seed := int64(1); seed <= 40; seed++ {
			cfg := Config{
				Seed:         seed*1000 + int64(bi),
				Birth:        &births[bi],
				LLM:          Noop,
				MaxAge:       40,
				NarrateRatio: 1,
			}.WithPoints(5, 5, 5, 5) // main always allocates 20 points
			res, err := Run(io.Discard, cfg, evs, careers)
			if err != nil {
				t.Fatalf("birth %s seed %d: %v", b.Name, seed, err)
			}
			present := agesPresent(res)
			for _, age := range []int{1, 2, 3} {
				if !present[age] {
					t.Fatalf("birth %s seed %d: no line at age %d (ages present: %v)",
						b.Name, seed, age, present)
				}
			}
		}
	}
}
