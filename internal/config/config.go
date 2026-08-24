// Package config loads the optional player configuration file
// (~/.config/rebirth/config.json). All fields are optional pointers so the
// loader can tell "not set" from an explicit zero, and merge precedence is
// flags > config file > built-in defaults (main owns the merge).
//
// Unknown keys are rejected (DisallowUnknownFields) so a typo fails
// loudly at startup instead of silently doing nothing — same gate as the
// event data (momus P1 lesson: key drift is a recurring failure mode).
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Config mirrors the JSON file. Every field is a pointer: nil = not set.
type Config struct {
	Provider *string  `json:"provider"`      // LLM provider preset name
	Model    *string  `json:"model"`         // model override
	BaseURL  *string  `json:"llm_url"`       // endpoint override
	MaxCalls *int     `json:"llm_calls"`     // per-life LLM call budget (narrate+fate)
	Narrate  *float64 `json:"narrate_ratio"` // fraction of trauma/good events narrated (0..1)
	MaxAge   *int     `json:"max_age"`       // default max age
	Seed     *int64   `json:"seed"`          // default seed (0 = time)
	Step     *bool    `json:"step"`          // force per-year advance
	Hints    *bool    `json:"hints"`         // transient LLM-thinking indicator
	Trauma   *Trauma  `json:"trauma"`        // dynamics overrides (balance calibration)
}

// Trauma holds the game-scale trauma dynamics overrides. nil fields keep
// the built-in defaults (see game.DefaultTraumaParams).
type Trauma struct {
	EnterAt          *float64 `json:"enter_at"`
	ExitAt           *float64 `json:"exit_at"`
	Drive            *float64 `json:"drive"`
	EventTraumaScale *float64 `json:"event_trauma_scale"`
}

// DefaultPath returns the config location (same directory as the bloodline
// save). os.UserConfigDir failure falls back to the current directory.
func DefaultPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return dir + "/rebirth/config.json"
	}
	return "config.json"
}

// Load reads and parses the config file. A missing file returns an empty
// Config (all nil) with no error — the file is optional. Parse failures
// (bad JSON, unknown keys) return a descriptive error; main warns and
// proceeds with defaults rather than aborting a game.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &c, nil
}
