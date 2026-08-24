package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMissingFileIsEmpty(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file must not error: %v", err)
	}
	if c.Provider != nil || c.MaxAge != nil || c.Trauma != nil {
		t.Fatalf("missing file must yield all-nil config: %+v", c)
	}
}

func TestFullParse(t *testing.T) {
	p := writeTemp(t, `{
  "provider": "deepseek",
  "model": "deepseek-v4-flash",
  "llm_url": "https://custom.example/v1",
  "llm_calls": 30,
  "narrate_ratio": 0.4,
  "max_age": 80,
  "seed": 42,
  "step": true,
  "hints": false,
  "trauma": {"enter_at": 0.9, "exit_at": 0.3, "drive": 0.55, "event_trauma_scale": 0.4}
}`)
	c, err := Load(p)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if *c.Provider != "deepseek" || *c.Model != "deepseek-v4-flash" {
		t.Fatalf("provider/model: %+v", c)
	}
	if *c.BaseURL != "https://custom.example/v1" || *c.MaxCalls != 30 {
		t.Fatalf("url/calls: %+v", c)
	}
	if *c.Narrate != 0.4 || *c.MaxAge != 80 || *c.Seed != 42 {
		t.Fatalf("narrate/maxage/seed: %+v", c)
	}
	if !*c.Step || *c.Hints {
		t.Fatalf("step/hints: %+v", c)
	}
	tr := c.Trauma
	if tr == nil || *tr.EnterAt != 0.9 || *tr.ExitAt != 0.3 || *tr.Drive != 0.55 || *tr.EventTraumaScale != 0.4 {
		t.Fatalf("trauma overrides: %+v", tr)
	}
}

func TestUnknownKeyRejected(t *testing.T) {
	p := writeTemp(t, `{"provider": "deepseek", "providor": "typo"}`)
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "providor") {
		t.Fatalf("unknown key must fail loudly, got: %v", err)
	}
}

func TestBadJSONRejected(t *testing.T) {
	p := writeTemp(t, `{"provider": `)
	if _, err := Load(p); err == nil {
		t.Fatal("broken JSON must error")
	}
}

func TestTraumaPartialOverride(t *testing.T) {
	p := writeTemp(t, `{"trauma": {"enter_at": 0.85}}`)
	c, err := Load(p)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Trauma == nil || c.Trauma.EnterAt == nil || c.Trauma.Drive != nil {
		t.Fatalf("partial trauma must keep other fields nil: %+v", c.Trauma)
	}
}
