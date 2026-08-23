package tui

import (
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
	e.Backward()
	if got := e.String(); got != "你好世" {
		t.Fatalf("backward broke CJK: %q", got)
	}
	for i := 0; i < 5; i++ {
		e.Backward() // over-delete is a no-op
	}
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

func TestForwardDeleteAndEndNoop(t *testing.T) {
	e := &LineEditor{Runes: []rune("删除测试"), Cursor: 1}
	e.Forward()
	if got := e.String(); got != "删测试" {
		t.Fatalf("forward delete broke CJK: %q", got)
	}
	e.Cursor = len(e.Runes)
	for i := 0; i < 10; i++ { // at end: no-op
		e.Forward()
	}
	if len(e.Runes) != 3 || e.String() != "删测试" {
		t.Fatalf("over-forward deleted extra runes: %q", e.String())
	}
}

// feed submits on Enter and keeps post-Enter bytes for the next call (P3-11).
func TestFeedSubmitCarriesRemainder(t *testing.T) {
	e := &LineEditor{}
	rest, act := feed(e, []byte("123\n456"))
	if act != actionSubmit || e.String() != "123" {
		t.Fatalf("submit broken: act=%v line=%q", act, e.String())
	}
	if string(rest) != "456" {
		t.Fatalf("post-Enter bytes lost: %q", rest)
	}
	// Remainder belongs to the NEXT ReadLine call — fresh editor.
	e2 := &LineEditor{}
	rest2, act2 := feed(e2, append(rest, '\r'))
	if act2 != actionSubmit || e2.String() != "456" {
		t.Fatalf("carried remainder misprocessed: act=%v line=%q", act2, e2.String())
	}
	if len(rest2) != 0 {
		t.Fatalf("unexpected trailing rest: %q", rest2)
	}
}

// A CJK rune split across two feeds must survive intact (P2-7).
func TestFeedSplitRuneCarried(t *testing.T) {
	e := &LineEditor{}
	zh := []byte("中") // e4 b8 ad
	rest, act := feed(e, zh[:2])
	if act != actionNone || len(rest) != 2 {
		t.Fatalf("partial rune not carried: rest=%q act=%v", rest, act)
	}
	rest, act = feed(e, append(rest, zh[2], 'x'))
	if act != actionNone || e.String() != "中x" {
		t.Fatalf("split rune lost or reordered: %q (rest=%q)", e.String(), rest)
	}
}

// An escape sequence split across two feeds must parse as one (P2-6).
func TestFeedSplitEscape(t *testing.T) {
	e := &LineEditor{Runes: []rune("abc"), Cursor: 1}
	rest, _ := feed(e, []byte{0x1b})
	if len(rest) != 1 {
		t.Fatalf("lone ESC not carried")
	}
	rest, _ = feed(e, append(rest, "[C"...))
	if len(rest) != 0 || e.Cursor != 2 {
		t.Fatalf("split CSI failed: cursor=%d rest=%q", e.Cursor, rest)
	}
}

// Ctrl+Right moves one rune right — it must NOT jump to Home (P2-5).
func TestFeedModifiedArrowNotHome(t *testing.T) {
	e := &LineEditor{Runes: []rune("abcd"), Cursor: 1}
	_, act := feed(e, []byte("\x1b[1;5C"))
	if act != actionNone || e.Cursor != 2 {
		t.Fatalf("Ctrl+Right wrong: cursor=%d want 2", e.Cursor)
	}
	fe2 := &LineEditor{Runes: []rune("abcd"), Cursor: 1}
	feed(fe2, []byte("\x1b[1;5D"))
	if fe2.Cursor != 0 {
		t.Fatalf("Ctrl+Left wrong: cursor=%d want 0", fe2.Cursor)
	}
}

// Ctrl+D cancels the line explicitly (P2-8).
func TestFeedCtrlDCancels(t *testing.T) {
	e := &LineEditor{Runes: []rune("hi")}
	rest, act := feed(e, []byte{0x04})
	if act != actionCancel {
		t.Fatalf("Ctrl+D should cancel, got act=%v", act)
	}
	if rest != nil {
		t.Fatalf("cancel should drop tail, got %q", rest)
	}
}

func TestFeedDeleteKey(t *testing.T) {
	e := &LineEditor{Runes: []rune("删除"), Cursor: 1}
	if _, act := feed(e, []byte("\x1b[3~")); act != actionNone || e.String() != "删" {
		t.Fatalf("CSI Delete broken: %q", e.String())
	}
}

func TestLeadingDigits(t *testing.T) {
	cases := map[string]int{"": 0, "3": 3, "1;5": 1, "15~": 15, "abc": 0}
	for in, want := range cases {
		if got := leadingDigits([]byte(in)); got != want {
			t.Errorf("leadingDigits(%q)=%d want %d", in, got, want)
		}
	}
}

// chunkFeeder simulates an os.Stdin that delivers bytes in arbitrary chunk
// sizes, driving the same loop structure ReadLineErr uses.
type chunkFeeder struct {
	chunks [][]byte
	i      int
}

func (f *chunkFeeder) read() []byte {
	if f.i >= len(f.chunks) {
		return nil
	}
	c := f.chunks[f.i]
	f.i++
	return c
}

// driveLoop replays the ReadLineErr loop body against split chunks.
func driveLoop(e *LineEditor, f *chunkFeeder) (string, lineAction) {
	var data []byte
	readMore := func() bool {
		c := f.read()
		if c == nil {
			return false
		}
		data = append(data, c...)
		return true
	}
	if !readMore() {
		return "", actionCancel
	}
	for {
		rest, act := feed(e, data)
		data = nil
		switch act {
		case actionSubmit:
			return e.String(), act
		case actionCancel:
			return e.String(), act
		default:
			// Mirror production: carry tail, then block for continuation
			// and PREPEND the tail before the new bytes.
			c := f.read()
			if c == nil {
				return e.String(), actionCancel
			}
			data = append(append([]byte(nil), rest...), c...)
		}
	}
}

// momus P1-3 regression: ESC[1;5C split across chunks must move the cursor,
// never inject "C" as literal text.
func TestLoopSplitEscapeNoInjection(t *testing.T) {
	e := &LineEditor{Runes: []rune("ab"), Cursor: 2}
	f := &chunkFeeder{chunks: [][]byte{{0x1b}, {'[', '1', ';', '5'}, {'C'}, {'a', 'b', '\r'}}}
	line, _ := driveLoop(e, f)
	if line != "abab" {
		t.Fatalf("split escape injected garbage: %q", line)
	}
}

// momus P1-3 regression: a CJK rune straddling chunks survives intact.
func TestLoopSplitRuneCJK(t *testing.T) {
	e := &LineEditor{}
	zh := []byte("中")
	f := &chunkFeeder{chunks: [][]byte{zh[:1], zh[1:2], zh[2:], {'x', '\r'}}}
	line, _ := driveLoop(e, f)
	if line != "中x" {
		t.Fatalf("split CJK rune lost: %q", line)
	}
}

// momus P2 regression: CRLF counts as one Enter, no phantom empty submit.
func TestFeedCRLFSingleSubmit(t *testing.T) {
	e := &LineEditor{}
	rest, act := feed(e, []byte("ab\r\ncd\r"))
	if act != actionSubmit || e.String() != "ab" {
		t.Fatalf("first submit wrong: %q %v", e.String(), act)
	}
	e2 := &LineEditor{}
	rest2, act2 := feed(e2, rest)
	if act2 != actionSubmit || e2.String() != "cd" || len(rest2) != 0 {
		t.Fatalf("second line after CRLF wrong: %q %v rest=%q", e2.String(), act2, rest2)
	}
}
