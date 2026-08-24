package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// chatReply is the minimal OpenAI-compatible response the client parses.
func chatReply(text string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"role": "assistant", "content": text}},
		},
	})
	return string(b)
}

// fakeLLM returns an httptest server that answers chat requests with the
// given JSON body and status.
func fakeLLM(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestChainFailsOverToSecondProvider(t *testing.T) {
	dead := fakeLLM(t, http.StatusInternalServerError, `{"error":"boom"}`)
	alive := fakeLLM(t, http.StatusOK, chatReply(`{"text":"命运的低语在耳畔响起"}`))

	chain := NewChain([]*Client{
		{BaseURL: dead.URL, Model: "m1", APIKey: "k1", HTTP: dead.Client()},
		{BaseURL: alive.URL, Model: "m2", APIKey: "k2", HTTP: alive.Client()},
	}, 24)
	chain.Lang = "zh"

	out := chain.Narrate(30, "状态", "原文")
	if out == "原文" {
		t.Fatal("chain should have produced the second provider's text")
	}
	if out != "命运的低语在耳畔响起" {
		t.Fatalf("unexpected chain output: %q", out)
	}
	if chain.Broken() {
		t.Fatal("chain must not be broken while any provider is alive")
	}
}

func TestChainTripsDeadProviderAndFallsBack(t *testing.T) {
	dead1 := fakeLLM(t, http.StatusInternalServerError, `{"error":"nope"}`)
	dead2 := fakeLLM(t, http.StatusInternalServerError, `{"error":"nope"}`)

	chain := NewChain([]*Client{
		{BaseURL: dead1.URL, Model: "m1", APIKey: "k1", HTTP: dead1.Client()},
		{BaseURL: dead2.URL, Model: "m2", APIKey: "k2", HTTP: dead2.Client()},
	}, 24)
	chain.Lang = "zh"

	// Three consecutive failures per provider trip both breakers.
	for i := 0; i < 3; i++ {
		if out := chain.Narrate(30, "状态", "原文"); out != "原文" {
			t.Fatalf("iteration %d: expected fallback, got %q", i, out)
		}
	}
	if !chain.Broken() {
		t.Fatal("chain should report broken when every provider is dead")
	}
	// Once broken, calls return instantly without network.
	if out := chain.Narrate(31, "状态", "原文"); out != "原文" {
		t.Fatalf("broken chain returned %q", out)
	}
}

func TestChainSkipsKeylessProviders(t *testing.T) {
	alive := fakeLLM(t, http.StatusOK, chatReply(`{"text":"only keyed provider speaks"}`))
	chain := NewChain([]*Client{
		{BaseURL: alive.URL, Model: "m1", APIKey: "", HTTP: alive.Client()},
		{BaseURL: alive.URL, Model: "m2", APIKey: "real", HTTP: alive.Client()},
	}, 24)
	chain.Lang = "zh"
	if len(chain.narrators) != 1 {
		t.Fatalf("expected 1 usable narrator, got %d", len(chain.narrators))
	}
}

func TestChainSharedBudget(t *testing.T) {
	alive := fakeLLM(t, http.StatusOK, chatReply(`{"text":"ok"}`))
	chain := NewChain([]*Client{
		{BaseURL: alive.URL, Model: "m1", APIKey: "k1", HTTP: alive.Client()},
	}, 2)
	chain.Lang = "zh"
	if out := chain.Narrate(1, "s", "fb"); out == "fb" {
		t.Fatal("first call should succeed")
	}
	if out := chain.Narrate(2, "s", "fb"); out == "fb" {
		t.Fatal("second call should succeed")
	}
	if out := chain.Narrate(3, "s", "fb"); out != "fb" {
		t.Fatalf("budget exhausted: expected fallback, got %q", out)
	}
}

func TestChainFateEventFailover(t *testing.T) {
	dead := fakeLLM(t, http.StatusBadRequest, `{"error":"bad"}`)
	alive := fakeLLM(t, http.StatusOK, chatReply(`{"text":"被一场夜雨改变了航向","good":true,"chr":1,"int":0,"str":0,"mny":0,"spr":2,"trauma_alpha":0.1}`))
	chain := NewChain([]*Client{
		{BaseURL: dead.URL, Model: "m1", APIKey: "k1", HTTP: dead.Client()},
		{BaseURL: alive.URL, Model: "m2", APIKey: "k2", HTTP: alive.Client()},
	}, 24)
	chain.Lang = "zh"

	ev, ok := chain.FateEvent(40, "状态")
	if !ok {
		t.Fatal("fate event should fail over to the live provider")
	}
	if ev.Text == "" {
		t.Fatal("fate event has no text")
	}
	if ev.LLMGenerated != true {
		t.Fatal("fate events must be marked LLMGenerated")
	}
}

func TestChainEpitaphExemptFromBudget(t *testing.T) {
	alive := fakeLLM(t, http.StatusOK, chatReply(`{"text":"此身虽逝，灯火长明"}`))
	chain := NewChain([]*Client{
		{BaseURL: alive.URL, Model: "m1", APIKey: "k1", HTTP: alive.Client()},
	}, 1)
	chain.Lang = "zh"
	// Consume the only budget unit.
	if out := chain.Narrate(1, "s", "fb"); out == "fb" {
		t.Fatal("narrate should consume the budget")
	}
	// Epitaph still works despite an exhausted budget.
	if ep := chain.Epitaph("总结"); ep == "一生至此。" {
		t.Fatal("epitaph must not be starved by the shared budget")
	}
}
