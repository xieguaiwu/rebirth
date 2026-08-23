package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rebirth/internal/game"
)

// mockServer returns a server that replies with one chat choice containing
// the given content.
func mockServer(t *testing.T, content string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing/incorrect auth header")
		}
		w.WriteHeader(status)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": content}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestFateEventValidJSONAccepted(t *testing.T) {
	srv := mockServer(t, `前言都行 {"text":"你在天台捡到一只受伤的鸽子。","good":true,"chr":0.5,"int":0,"str":0,"mny":9,"spr":2,"trauma_alpha":0.8} 尾声也行`, 200)
	n := NewNarrator(&Client{BaseURL: srv.URL, Model: "test", APIKey: "test-key"})
	defer srv.Close()

	ev, ok := n.FateEvent(30, "summary")
	if !ok {
		t.Fatal("valid payload rejected")
	}
	if ev.Delta.MNY > 3 {
		t.Fatalf("MNY clamp failed: %f", ev.Delta.MNY)
	}
	if ev.TraumaAlpha > 0.5 {
		t.Fatalf("trauma alpha clamp failed: %f", ev.TraumaAlpha)
	}
	if !ev.LLMGenerated {
		t.Fatal("LLMGenerated flag not set")
	}
}

func TestFateEventInvalidRejected(t *testing.T) {
	for name, content := range map[string]string{
		"no json":     "抱歉我做不到",
		"empty text":  `{"text":"好","good":true}`,
		"broken json": `{"text":"测试", "mny": }`,
	} {
		srv := mockServer(t, content, 200)
		n := NewNarrator(&Client{BaseURL: srv.URL, Model: "test", APIKey: "test-key"})
		if _, ok := n.FateEvent(30, "s"); ok {
			t.Fatalf("%s: invalid payload accepted", name)
		}
		srv.Close()
	}
}

func TestNarrateFallsBackOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	n := NewNarrator(&Client{BaseURL: srv.URL, Model: "test", APIKey: "test-key"})
	got := n.Narrate(30, "s", "原始文本")
	if got != "原始文本" {
		t.Fatalf("fallback broken: %q", got)
	}
}

func TestEpitaphFallback(t *testing.T) {
	srv := mockServer(t, `{"text":""}`, 200)
	defer srv.Close()
	n := NewNarrator(&Client{BaseURL: srv.URL, Model: "test", APIKey: "test-key"})
	if got := n.Epitaph("s"); !strings.Contains(got, "一生至此") {
		t.Fatalf("expected default epitaph, got %q", got)
	}
}

// Compile-time interface check.
var _ game.Narrator = (*Narrator)(nil)
