package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// e2eDrive is a running daemon process under test.
type e2eDrive struct {
	t     *testing.T
	cmd   *exec.Cmd
	stdin *bufio.Writer
	sc    *bufio.Scanner
	id    int64
	dir   string
}

func startDaemon(t *testing.T, dir string) *e2eDrive {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "rebirth-mobile")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build daemon: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "--dir", dir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})
	d := &e2eDrive{
		t:     t,
		cmd:   cmd,
		stdin: bufio.NewWriter(stdin),
		sc:    bufio.NewScanner(stdout),
		dir:   dir,
	}
	return d
}

// call sends one request and returns the decoded response data/error.
func (d *e2eDrive) call(cmd string, params map[string]any) (map[string]any, string) {
	d.t.Helper()
	d.id++
	req := map[string]any{"id": d.id, "cmd": cmd}
	for k, v := range params {
		req[k] = v
	}
	raw, err := json.Marshal(req)
	if err != nil {
		d.t.Fatal(err)
	}
	if _, err := d.stdin.Write(append(raw, '\n')); err != nil {
		d.t.Fatalf("write: %v", err)
	}
	if err := d.stdin.Flush(); err != nil {
		d.t.Fatalf("flush: %v", err)
	}
	if !d.sc.Scan() {
		d.t.Fatalf("no response: %v", d.sc.Err())
	}
	var resp struct {
		ID    int64          `json:"id"`
		OK    bool           `json:"ok"`
		Data  map[string]any `json:"data"`
		Error string         `json:"error"`
	}
	if err := json.Unmarshal(d.sc.Bytes(), &resp); err != nil {
		d.t.Fatalf("bad response line %q: %v", d.sc.Text(), err)
	}
	if resp.ID != d.id {
		d.t.Fatalf("response id %d != request id %d", resp.ID, d.id)
	}
	if !resp.OK {
		return nil, resp.Error
	}
	return resp.Data, ""
}

func (d *e2eDrive) mustOK(cmd string, params map[string]any) map[string]any {
	d.t.Helper()
	data, errStr := d.call(cmd, params)
	if errStr != "" {
		d.t.Fatalf("%s failed: %s", cmd, errStr)
	}
	return data
}

func (d *e2eDrive) mustErr(cmd string, params map[string]any) string {
	d.t.Helper()
	_, errStr := d.call(cmd, params)
	if errStr == "" {
		d.t.Fatalf("%s unexpectedly succeeded", cmd)
	}
	return errStr
}

func (d *e2eDrive) shutdown() {
	d.t.Helper()
	d.mustOK("shutdown", nil)
	_ = d.cmd.Wait()
}

// sampleSession returns a standard new_session parameter map.
func sampleSession(seed int64, birth map[string]any, talents []map[string]any) map[string]any {
	return map[string]any{
		"seed":    seed,
		"lang":    "zh",
		"birth":   birth,
		"talents": talents,
		"points":  map[string]any{"chr": 5, "int": 5, "str": 5, "mny": 5},
		"max_age": 100,
		"narrator": map[string]any{
			"enabled":   false,
			"providers": []any{},
		},
	}
}

// TestProtocolFullLife drives a complete life and asserts every protocol
// shape: hello, draws, session, yearly steps, death payload, lineage save.
func TestProtocolFullLife(t *testing.T) {
	dir := t.TempDir()
	d := startDaemon(t, dir)

	hello := d.mustOK("hello", nil)
	if hello["ver"] != version || hello["proto"] != float64(1) {
		t.Fatalf("hello: %v", hello)
	}

	bl := d.mustOK("bloodline_get", nil)
	if bl["generation"] != float64(0) {
		t.Fatalf("fresh bloodline generation: %v", bl)
	}

	births := d.mustOK("draw_births", map[string]any{"seed": 42})
	birthList, ok := births["births"].([]any)
	if !ok || len(birthList) != 3 {
		t.Fatalf("draw_births: %v", births)
	}
	birth := birthList[0].(map[string]any)

	talents := d.mustOK("draw_talents", map[string]any{"seed": 42})
	talentList, ok := talents["talents"].([]any)
	if !ok || len(talentList) != 10 {
		t.Fatalf("draw_talents: %v", talents)
	}

	sess := d.mustOK("new_session", sampleSession(42, birth, []map[string]any{
		talentList[0].(map[string]any),
		talentList[1].(map[string]any),
		talentList[2].(map[string]any),
	}))
	if sess["generation"] != float64(1) {
		t.Fatalf("generation: %v", sess)
	}

	var lastAge float64
	died := false
	steps := 0
	for !died && steps < 120 {
		steps++
		yr := d.mustOK("next", nil)
		if yr["died"].(bool) {
			died = true
			if yr["death_status"] == "" {
				t.Fatal("died year missing death_status")
			}
			if yr["epitaph"] == "" {
				t.Fatal("died year missing epitaph")
			}
			if yr["lineage_saved"] != true {
				t.Fatal("lineage not saved")
			}
			if yr["next_generation"] != float64(1) {
				t.Fatalf("next_generation: %v", yr)
			}
			if _, ok := yr["next_sensitivity"]; !ok {
				t.Fatal("missing next_sensitivity")
			}
		}
		lastAge = yr["age"].(float64)
		if lines, ok := yr["lines"]; ok && lines != nil {
			if _, ok := lines.([]any); !ok {
				t.Fatal("lines is not an array")
			}
		}
		if _, ok := yr["stats"].(map[string]any); !ok {
			t.Fatal("missing stats")
		}
		if _, ok := yr["trauma"].(map[string]any); !ok {
			t.Fatal("missing trauma")
		}
	}
	if !died {
		t.Fatalf("life did not end within 120 steps (last age %v)", lastAge)
	}

	// The death year consumes the session; further next calls error
	// with the protocol §1.6 message.
	if got := d.mustErr("next", nil); got != "session finished" {
		t.Fatalf("expected 'session finished', got %q", got)
	}

	// Bloodline file on disk reflects the saved lineage.
	raw, err := os.ReadFile(filepath.Join(dir, "bloodline.json"))
	if err != nil {
		t.Fatalf("bloodline.json missing: %v", err)
	}
	var bl2 struct {
		Generation  int     `json:"generation"`
		Sensitivity float64 `json:"sensitivity"`
	}
	if err := json.Unmarshal(raw, &bl2); err != nil {
		t.Fatalf("bloodline parse: %v", err)
	}
	if bl2.Generation != 1 || bl2.Sensitivity < 0 || bl2.Sensitivity > 1 {
		t.Fatalf("bloodline content: %+v", bl2)
	}

	d.shutdown()
}

// TestCheckpointResumeDeterministic proves that killing the daemon mid-life
// and resuming from the checkpoint produces a bit-identical life: the
// resumed History equals the uninterrupted run's History.
func TestCheckpointResumeDeterministic(t *testing.T) {
	const seed = 777

	// Each run gets its own data dir so bloodline state cannot leak
	// between the uninterrupted and the resumed run.
	fullDir := t.TempDir()
	resumedDir := t.TempDir()

	runFull := func(dir string, resumeAt int) ([]string, int) {
		d := startDaemon(t, dir)
		births := d.mustOK("draw_births", map[string]any{"seed": seed})
		birth := births["births"].([]any)[0].(map[string]any)
		talents := d.mustOK("draw_talents", map[string]any{"seed": seed})
		tl := talents["talents"].([]any)
		d.mustOK("new_session", sampleSession(seed, birth, []map[string]any{
			tl[0].(map[string]any), tl[1].(map[string]any), tl[2].(map[string]any),
		}))

		var lines []string
		checkpointed := false
		for {
			yr := d.mustOK("next", nil)
			for _, l := range yr["lines"].([]any) {
				lines = append(lines, l.(string))
			}
			age := int(yr["age"].(float64))
			if resumeAt > 0 && age == resumeAt && !checkpointed {
				checkpointed = true
				cp := d.mustOK("checkpoint_get", nil)
				if cp["exists"] != true || int(cp["age"].(float64)) != resumeAt {
					t.Fatalf("checkpoint wrong: %v", cp)
				}
				d.shutdown() // kill the daemon: session state only on disk
				d = startDaemon(t, dir)
				cp2 := d.mustOK("checkpoint_get", nil)
				if cp2["exists"] != true || int(cp2["age"].(float64)) != resumeAt {
					t.Fatalf("checkpoint after restart: %v", cp2)
				}
				rd := d.mustOK("resume_session", map[string]any{"narrator": map[string]any{
					"enabled": false, "providers": []any{},
				}})
				// P1-2: every replayed year must be returned so the UI can
				// rebuild the timeline (the crash lost those YearResults).
				yrs, ok := rd["years"].([]any)
				if !ok || len(yrs) != resumeAt+1 {
					t.Fatalf("resume_session must return replayed years 0..%d: %v", resumeAt, rd)
				}
				lastYr := yrs[len(yrs)-1].(map[string]any)
				if int(lastYr["age"].(float64)) != resumeAt {
					t.Fatalf("last replayed year must be %d: %v", resumeAt, lastYr)
				}
			}
			if yr["died"].(bool) {
				// The resumed run dies with the same epitaph.
				d.shutdown()
				return lines, int(yr["age"].(float64))
			}
		}
	}

	uninterrupted, age1 := runFull(fullDir, 0)
	resumed, age2 := runFull(resumedDir, 30)
	if age1 != age2 {
		t.Fatalf("death age differs: %d vs %d", age1, age2)
	}
	if len(uninterrupted) != len(resumed) {
		t.Fatalf("history length differs: %d vs %d", len(uninterrupted), len(resumed))
	}
	for i := range uninterrupted {
		if uninterrupted[i] != resumed[i] {
			t.Fatalf("history[%d] differs:\n  full:   %q\n  resumed: %q", i, uninterrupted[i], resumed[i])
		}
	}
}

// TestEnglishSessionRuns asserts the en dataset drives a full life through
// the daemon.
func TestEnglishSessionRuns(t *testing.T) {
	dir := t.TempDir()
	d := startDaemon(t, dir)
	births := d.mustOK("draw_births", map[string]any{"seed": 1, "lang": "en"})
	birth := births["births"].([]any)[0].(map[string]any)
	name, _ := birth["name"].(string)
	if !strings.ContainsAny(name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		t.Fatalf("en birth name not English: %q", name)
	}
	sess := sampleSession(1, birth, []map[string]any{})
	sess["lang"] = "en"
	d.mustOK("new_session", sess)

	died := false
	for i := 0; i < 120 && !died; i++ {
		yr := d.mustOK("next", nil)
		died = yr["died"].(bool)
		if died && yr["epitaph"] == "" {
			t.Fatal("en death missing epitaph")
		}
	}
	if !died {
		t.Fatal("en life did not complete")
	}
	d.shutdown()
}

// TestNarratorChainViaProtocol wires two providers (first dead) through the
// protocol and asserts the chain fails over. Uses httptest servers.
func TestNarratorChainViaProtocol(t *testing.T) {
	dir := t.TempDir()
	d := startDaemon(t, dir)

	dead := fakeServer(t, 500, `{"error":"down"}`)
	alive := fakeServer(t, 200, `{"text":"活着的那家讲了故事"}`)

	births := d.mustOK("draw_births", map[string]any{"seed": 9})
	birth := births["births"].([]any)[0].(map[string]any)
	sess := sampleSession(9, birth, []map[string]any{})
	sess["narrator"] = map[string]any{
		"enabled": true,
		"providers": []any{
			map[string]any{"provider": "deepseek", "model": "m", "url": dead.URL, "key": "k1"},
			map[string]any{"provider": "deepseek", "model": "m", "url": alive.URL, "key": "k2"},
		},
		"budget": 24,
		"ratio":  0.5,
	}
	d.mustOK("new_session", sess)

	// Run until a year with an LLM-narrated line (narrated text differs
	// from the pool text). Narrated text appears in the year's lines.
	narrated := false
	for i := 0; i < 120 && !narrated; i++ {
		yr := d.mustOK("next", nil)
		if lines, ok := yr["lines"].([]any); ok {
			for _, l := range lines {
				if strings.Contains(l.(string), "活着的那家讲了故事") {
					narrated = true
				}
			}
		}
		if yr["died"].(bool) {
			break
		}
	}
	if !narrated {
		t.Fatal("no LLM-narrated year observed (chain failover did not engage)")
	}
	d.shutdown()
}

// fakeServer is a minimal OpenAI-compatible LLM endpoint for protocol-level
// chain tests. content is the assistant text (a JSON envelope for narrate).
func fakeServer(t *testing.T, status int, content string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		body := `{"choices":[{"message":{"role":"assistant","content":` + jsonEscape(content) + `}}]}`
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// jsonEscape embeds a string into a JSON string literal.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
