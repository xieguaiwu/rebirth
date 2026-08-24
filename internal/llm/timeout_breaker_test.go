package llm

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// neverServer accepts connections and reads the request but never answers —
// the nastiest stall case (dead upstream behind a live proxy).
func neverServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				buf := make([]byte, 4096)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
				}
			}(c)
		}
	}()
	return "http://" + ln.Addr().String()
}

// TestTimeoutsEnforced: a stalled endpoint must fail at the per-call context
// deadline (12s narrate / 18s fate), never hang forever. Regression guard
// for the "命运编织中 stuck 44 minutes" report: proves each call is bounded.
func TestTimeoutsEnforced(t *testing.T) {
	if testing.Short() {
		t.Skip("30s wall time")
	}
	c := New("k", "m")
	c.BaseURL = neverServer(t)
	n := NewNarrator(c)

	start := time.Now()
	if out := n.Narrate(30, "状态", "回退文本"); out != "回退文本" {
		t.Fatalf("Narrate fallback broken: %q", out)
	}
	if el := time.Since(start); el > 20*time.Second {
		t.Fatalf("Narrate took %v — timeout not enforced", el)
	}

	start = time.Now()
	if _, ok := n.FateEvent(30, "状态"); ok {
		t.Fatal("FateEvent should fail against a stalled server")
	}
	if el := time.Since(start); el > 26*time.Second {
		t.Fatalf("FateEvent took %v — timeout not enforced", el)
	}
}

// TestBreakerTripsAfterConsecutiveFailures: after failLimit consecutive
// failures every method must return its fallback INSTANTLY (no network),
// and Broken() must report true. This is the fix for crawl-mode: a
// dead-but-slow channel used to burn the full timeout before every
// fallback, once per sampled event, for the whole life.
func TestBreakerTripsAfterConsecutiveFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New("k", "m")
	c.BaseURL = srv.URL
	c.HTTP = &http.Client{Timeout: 2 * time.Second} // fast failures
	n := NewNarrator(c)

	for i := 1; i <= failLimit; i++ {
		if out := n.Narrate(10, "s", "fb"); out != "fb" {
			t.Fatalf("call %d: expected fallback, got %q", i, out)
		}
		if n.Broken() != (i == failLimit) {
			t.Fatalf("call %d: Broken=%v, want %v", i, n.Broken(), i == failLimit)
		}
	}
	// Tripped: instant fail-soft, budget untouched.
	before := n.calls
	start := time.Now()
	if out := n.Narrate(10, "s", "fb"); out != "fb" {
		t.Fatalf("post-breaker Narrate: %q", out)
	}
	if _, ok := n.FateEvent(10, "s"); ok {
		t.Fatal("post-breaker FateEvent must not succeed")
	}
	if got := n.Epitaph("s"); got != "一生至此。" {
		t.Fatalf("post-breaker Epitaph: %q", got)
	}
	if el := time.Since(start); el > 500*time.Millisecond {
		t.Fatalf("post-breaker calls took %v — should be instant", el)
	}
	if n.calls != before {
		t.Fatalf("breaker must not consume budget: %d -> %d", before, n.calls)
	}
}

// TestBreakerResetsOnSuccess: an intermittent channel that recovers must
// close the breaker again (streak resets on any success).
func TestBreakerResetsOnSuccess(t *testing.T) {
	fail := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"text\":\"外婆的灶台冒着热气。\"}"}}]}`))
	}))
	defer srv.Close()
	c := New("k", "m")
	c.BaseURL = srv.URL
	c.HTTP = &http.Client{Timeout: 2 * time.Second}
	n := NewNarrator(c)

	n.Narrate(1, "s", "fb") // fail 1
	n.Narrate(1, "s", "fb") // fail 2
	fail = false
	if out := n.Narrate(1, "s", "fb"); out != "外婆的灶台冒着热气。" {
		t.Fatalf("success path broken: %q", out)
	}
	if n.Broken() {
		t.Fatal("breaker must reset on success")
	}
	fail = true
	n.Narrate(1, "s", "fb") // fail 1 again, not trip
	if n.Broken() {
		t.Fatal("streak must have restarted after success")
	}
}
