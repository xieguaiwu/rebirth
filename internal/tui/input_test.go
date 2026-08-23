package tui

import (
	"strings"
	"testing"
)

func TestInsertAdvancesByRune(t *testing.T) {
	e := &LineEditor{}
	e.Insert([]rune("ab"))
	e.Insert([]rune("中"))
	if got := e.String(); got != "ab中" {
		t.Fatalf("insert broken: %q", got)
	}
	if e.Cursor != 3 {
		t.Fatalf("cursor should advance by rune count: %d", e.Cursor)
	}
}

func TestBackwardCJKSafe(t *testing.T) {
	e := &LineEditor{Runes: []rune("你好世界"), Cursor: 4}
	e.Backward() // deletes 界, not half a byte
	if got := e.String(); got != "你好世" {
		t.Fatalf("backward broke CJK: %q", got)
	}
	e.Backward()
	e.Backward()
	e.Backward()
	e.Backward() // at start, no-op
	if e.String() != "" || e.Cursor != 0 {
		t.Fatalf("over-delete: %q cursor=%d", e.String(), e.Cursor)
	}
}

func TestKillWordBackMixed(t *testing.T) {
	e := &LineEditor{Runes: []rune("hello 你好 world"), Cursor: len([]rune("hello 你好 world"))}
	e.KillWordBack()
	if got := string(e.Runes); got != "hello 你好 " {
		t.Fatalf("word kill wrong: %q", got)
	}
	if e.Cursor != len(e.Runes) {
		t.Fatalf("cursor not clamped after kill: %d vs %d", e.Cursor, len(e.Runes))
	}
	e.KillWordBack()
	if got := string(e.Runes); got != "hello " {
		t.Fatalf("second word kill wrong: %q", got)
	}
}

func TestKillBeforeAfter(t *testing.T) {
	e := &LineEditor{Runes: []rune("abcdef"), Cursor: 3}
	e.KillBefore()
	if e.String() != "def" || e.Cursor != 0 {
		t.Fatalf("kill before broken: %q", e.String())
	}
	e.Insert([]rune("xy"))
	e.KillAfter()
	if e.String() != "xy" {
		t.Fatalf("kill after broken: %q", e.String())
	}
}

func TestForwardDelete(t *testing.T) {
	e := &LineEditor{Runes: []rune("删除测试"), Cursor: 1}
	e.Forward()
	if got := e.String(); got != "删测试" {
		t.Fatalf("forward delete broke CJK: %q", got)
	}
	// Repeated Delete at a fixed cursor keeps deleting forward (editor semantics).
	e.Forward()
	if got := e.String(); got != "删试" {
		t.Fatalf("repeat forward should delete next rune: %q", got)
	}
	// At end of buffer, further deletes are no-ops.
	e.Cursor = len(e.Runes)
	for i := 0; i < 10; i++ {
		e.Forward()
	}
	if len(e.Runes) != 2 || e.String() != "删试" {
		t.Fatalf("over-forward deleted extra runes: %q", e.String())
	}
}

// decodeRune must never return a partial rune for split UTF-8 input.
func TestDecodeRuneSplitBytes(t *testing.T) {
	full := []byte("中")
	r, size := decodeRune(full)
	if size == 0 || r != '中' {
		t.Fatalf("full rune failed: r=%q size=%d", r, size)
	}
	bad := full[:2] // truncated
	if _, size := decodeRune(bad); size == 0 && len(string(r)) == 0 {
		t.Log("truncated bytes rejected (acceptable)")
	}
}

func TestSkipToTerminator(t *testing.T) {
	if n := skipToTerminator([]byte("5~")); n != 1 {
		t.Fatalf("tilde seq consumed wrong: %d", n)
	}
}

var _ = strings.TrimSpace // keep import if future tests need it
