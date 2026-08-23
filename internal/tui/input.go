// Package tui provides a zero-dependency, rune-safe terminal line editor
// and simple menu helpers. All cursor arithmetic is in runes (Gate 12).
//
// Input parsing is chunk-safe: incomplete escape sequences and partial
// UTF-8 runes are carried across os.Stdin.Read boundaries instead of being
// dropped or injected as literal text (momus P2-6/P2-7/P3-11).
package tui

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"
	"unsafe"
)

// ErrCancelled reports that the user aborted input (Ctrl+C, Ctrl+D, EOF).
var ErrCancelled = errors.New("input cancelled")

// termios mirrors the kernel struct for ioctl TCGETS/TCSETS.
type termios struct {
	Iflag, Oflag, Cflag, Lflag uint32
	Line                       byte
	Cc                         [32]byte
	Ispeed, Ospeed             uint32
}

const (
	ioctlReadTermios  = 0x5401 // TCGETS
	ioctlWriteTermios = 0x5402 // TCSETS
)

func ioctl(fd uintptr, req uintptr, t *termios) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return errno
	}
	return nil
}

func makeRaw(fd uintptr) (*termios, error) {
	old := &termios{}
	if err := ioctl(fd, ioctlReadTermios, old); err != nil {
		return nil, err
	}
	raw := *old
	raw.Iflag &^= syscall.IXON | syscall.ICRNL | syscall.BRKINT
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := ioctl(fd, ioctlWriteTermios, &raw); err != nil {
		return nil, err
	}
	return old, nil
}

// IsTTY reports whether stdin is an interactive terminal.
func IsTTY() bool {
	var t termios
	return ioctl(os.Stdin.Fd(), ioctlReadTermios, &t) == nil
}

// LineEditor holds a rune buffer with a rune-index cursor (skill §1.6).
type LineEditor struct {
	Runes  []rune
	Cursor int // rune index
}

// Insert places runes at the cursor and advances it by the rune count.
func (e *LineEditor) Insert(ins []rune) {
	out := make([]rune, 0, len(e.Runes)+len(ins))
	out = append(out, e.Runes[:e.Cursor]...)
	out = append(out, ins...)
	out = append(out, e.Runes[e.Cursor:]...)
	e.Runes = out
	e.Cursor += len(ins)
}

// Backward deletes the rune before the cursor. Rune-safe for CJK.
func (e *LineEditor) Backward() {
	if e.Cursor == 0 {
		return
	}
	e.Runes = append(e.Runes[:e.Cursor-1], e.Runes[e.Cursor:]...)
	e.Cursor--
}

// Forward deletes the rune at the cursor.
func (e *LineEditor) Forward() {
	if e.Cursor >= len(e.Runes) {
		return
	}
	e.Runes = append(e.Runes[:e.Cursor], e.Runes[e.Cursor+1:]...)
}

// KillBefore deletes to line start (Ctrl+U).
func (e *LineEditor) KillBefore() {
	e.Runes = e.Runes[e.Cursor:]
	e.Cursor = 0
}

// KillAfter deletes to line end (Ctrl+K).
func (e *LineEditor) KillAfter() {
	e.Runes = e.Runes[:e.Cursor]
}

// KillWordBack deletes the word before the cursor (Ctrl+W). A word is a
// maximal run of non-space runes; trailing spaces go first.
func (e *LineEditor) KillWordBack() {
	i := e.Cursor
	for i > 0 && e.Runes[i-1] == ' ' {
		i--
	}
	for i > 0 && e.Runes[i-1] != ' ' {
		i--
	}
	e.Runes = append(e.Runes[:i], e.Runes[e.Cursor:]...)
	e.Cursor = i
}

// String renders the current buffer.
func (e *LineEditor) String() string { return string(e.Runes) }

type lineAction int

const (
	actionNone   lineAction = iota
	actionSubmit            // Enter pressed
	actionCancel            // Ctrl+C or Ctrl+D
)

// rawCarry holds bytes that belong to the NEXT ReadLine call: the tail of
// a paste after an embedded Enter (momus P3-11), never lost silently.
var rawCarry []byte

// feed applies one chunk of raw terminal bytes to the editor. It returns
// the unconsumed tail — an incomplete escape sequence, a partial UTF-8
// rune, or bytes following an embedded Enter — to be processed later.
func feed(e *LineEditor, data []byte) (rest []byte, act lineAction) {
	i := 0
	for i < len(data) {
		b := data[i]
		switch {
		case b == '\r' || b == '\n':
			return cloneBytes(data[i+1:]), actionSubmit
		case b == 0x7f || b == 0x08:
			e.Backward()
			i++
		case b == 0x15: // Ctrl+U
			e.KillBefore()
			i++
		case b == 0x0b: // Ctrl+K
			e.KillAfter()
			i++
		case b == 0x17: // Ctrl+W
			e.KillWordBack()
			i++
		case b == 0x01: // Ctrl+A
			e.Cursor = 0
			i++
		case b == 0x05: // Ctrl+E
			e.Cursor = len(e.Runes)
			i++
		case b == 0x03 || b == 0x04: // Ctrl+C / Ctrl+D cancel (P2-8)
			return nil, actionCancel
		case b == 0x1b:
			consumed, complete := parseEscape(e, data[i:])
			if !complete {
				// Incomplete sequence: carry until more bytes arrive.
				return cloneBytes(data[i:]), actionNone
			}
			i += consumed
		default:
			if !utf8.FullRune(data[i:]) {
				// Rune straddles the chunk boundary: carry, never drop.
				return cloneBytes(data[i:]), actionNone
			}
			r, size := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size <= 1 {
				i++ // invalid byte: skip rather than corrupt the line
				continue
			}
			e.Insert([]rune{r})
			i += size
		}
	}
	return nil, actionNone
}

func cloneBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// parseEscape handles one escape sequence at seq[0]=='\x1b'. It returns
// the consumed byte count, or ok=false when the sequence is incomplete
// and needs more input.
func parseEscape(e *LineEditor, seq []byte) (int, bool) {
	if len(seq) < 2 {
		return 0, false
	}
	switch seq[1] {
	case '[':
		for j := 2; j < len(seq); j++ {
			if seq[j] >= 0x40 && seq[j] <= 0x7e {
				dispatchCSI(e, seq[2:j], seq[j])
				return j + 1, true
			}
		}
		return 0, false // no terminator yet
	case 'O':
		if len(seq) < 3 {
			return 0, false
		}
		applyFinalByte(e, seq[2])
		return 3, true
	default:
		return 1, true // lone Esc: ignore
	}
}

// dispatchCSI interprets one CSI sequence: inner params + final byte.
// Modified-arrow variants (ESC[1;5C etc.) move like their unmodified
// base key instead of wiping the cursor (momus P2-5).
func dispatchCSI(e *LineEditor, inner []byte, final byte) {
	switch final {
	case 'A', 'B': // up/down: no history in single-line mode
	case 'C':
		if e.Cursor < len(e.Runes) {
			e.Cursor++
		}
	case 'D':
		if e.Cursor > 0 {
			e.Cursor--
		}
	case 'H', 'F':
		if final == 'H' {
			e.Cursor = 0
		} else {
			e.Cursor = len(e.Runes)
		}
	case '~':
		n := leadingDigits(inner)
		switch n {
		case 3:
			e.Forward() // Delete
		case 1, 7:
			e.Cursor = 0 // Home variants
		case 4, 8:
			e.Cursor = len(e.Runes) // End variants
		} // 5/6 PageUp/Down: no-op
	default:
		// Unknown final byte: ignore.
	}
}

// applyFinalByte handles SS3 application-cursor-mode keys.
func applyFinalByte(e *LineEditor, c byte) {
	switch c {
	case 'C':
		if e.Cursor < len(e.Runes) {
			e.Cursor++
		}
	case 'D':
		if e.Cursor > 0 {
			e.Cursor--
		}
	case 'H':
		e.Cursor = 0
	case 'F':
		e.Cursor = len(e.Runes)
	}
}

func leadingDigits(b []byte) int {
	n := 0
	for _, c := range b {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// redraw rewrites the edited line in place.
func redraw(s string) {
	fmt.Printf("\r\x1b[K%s", s)
}

// ReadLine reads one line with raw-mode editing on TTYs and falls back to
// buffered scanning otherwise. Returns ErrCancelled on Ctrl+C/Ctrl+D/EOF.
func ReadLine(prompt string) string {
	s, err := ReadLineErr(prompt)
	if err != nil {
		return ""
	}
	return s
}

// ReadLineErr is ReadLine with an explicit cancellation signal so callers
// can distinguish "empty line" from "player aborted" (momus P2-8).
func ReadLineErr(prompt string) (string, error) {
	if !IsTTY() {
		fmt.Print(prompt)
		sc := bufio.NewScanner(os.Stdin)
		if !sc.Scan() {
			return "", ErrCancelled
		}
		return strings.TrimRight(sc.Text(), "\r\n"), nil
	}

	fd := os.Stdin.Fd()
	old, err := makeRaw(fd)
	if err != nil {
		fmt.Print(prompt)
		sc := bufio.NewScanner(os.Stdin)
		if !sc.Scan() {
			return "", ErrCancelled
		}
		return strings.TrimRight(sc.Text(), "\r\n"), nil
	}
	defer ioctl(fd, ioctlWriteTermios, old)

	fmt.Print(prompt)
	e := &LineEditor{}
	pendingIn := rawCarry
	rawCarry = nil
	buf := make([]byte, 256)
	for {
		if len(pendingIn) == 0 {
			n, rerr := os.Stdin.Read(buf)
			if rerr != nil || n == 0 {
				fmt.Println()
				return "", ErrCancelled
			}
			pendingIn = append(pendingIn, buf[:n]...)
		}
		rest, act := feed(e, pendingIn)
		rawCarry = rest
		pendingIn = nil
		redraw(e.String())
		switch act {
		case actionSubmit:
			fmt.Println()
			return e.String(), nil
		case actionCancel:
			fmt.Println("^C")
			return "", ErrCancelled
		}
	}
}

// Choose prints prompt + numbered options and reads a valid choice in
// [1, options]. Parse errors re-prompt; cancellation propagates.
func Choose(prompt string, options int) (int, error) {
	for {
		fmt.Printf("%s [1-%d] > ", prompt, options)
		line, err := ReadLineErr("")
		if err != nil {
			return 0, err
		}
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		v, perr := strconv.Atoi(s)
		if perr != nil || v < 1 || v > options {
			fmt.Println("无效选项，请输入数字。")
			continue
		}
		return v, nil
	}
}

// Pause waits for Enter (or any line) so the player can read output.
func Pause(hint string) {
	fmt.Print(hint)
	ReadLine("")
}
