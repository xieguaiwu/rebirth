// Package tui provides a zero-dependency, rune-safe terminal line editor
// and simple menu helpers. All cursor arithmetic is in runes (Gate 12).
package tui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

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

// ReadLine reads one line with raw-mode editing on TTYs and falls back to
// buffered scanning otherwise. Supported: ←→↑↓ Home End Delete Backspace,
// Ctrl+A/E/U/K/W/H. History navigation is intentionally omitted (menus only
// need single lines); ↑/↓ are accepted as no-ops.
func ReadLine(prompt string) string {
	if !IsTTY() {
		fmt.Print(prompt)
		sc := bufio.NewScanner(os.Stdin)
		if !sc.Scan() {
			return ""
		}
		return strings.TrimRight(sc.Text(), "\r\n")
	}

	fd := os.Stdin.Fd()
	old, err := makeRaw(fd)
	if err != nil {
		fmt.Print(prompt)
		sc := bufio.NewScanner(os.Stdin)
		sc.Scan()
		return strings.TrimRight(sc.Text(), "\r\n")
	}
	defer ioctl(fd, ioctlWriteTermios, old)

	fmt.Print(prompt)
	e := &LineEditor{}
	buf := make([]byte, 32)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}
		done := false
		for i := 0; i < n && !done; {
			b := buf[i]
			switch {
			case b == '\r' || b == '\n':
				done = true
			case b == 0x7f || b == 0x08: // Backspace / Ctrl+H
				e.Backward()
			case b == 0x15: // Ctrl+U
				e.KillBefore()
			case b == 0x0b: // Ctrl+K
				e.KillAfter()
			case b == 0x17: // Ctrl+W
				e.KillWordBack()
			case b == 0x01: // Ctrl+A
				e.Cursor = 0
			case b == 0x05: // Ctrl+E
				e.Cursor = len(e.Runes)
			case b == 0x03: // Ctrl+C: cancel line
				fmt.Println("^C")
				e.Runes = nil
				done = true
			case b == 0x1b && i+2 < n && buf[i+1] == '[':
				i += handleCSI(e, buf[i+2:n]) + 2
			case b == 0x1b && i+2 < n && buf[i+1] == 'O':
				i += handleSS3(e, buf[i+2]) + 2
			case b == 0x1b:
				// Lone Esc: ignore (no pending sequence within this chunk).
			default:
				// Collect a full UTF-8 rune from possibly-split bytes.
				r, size := decodeRune(buf[i:n])
				if size > 0 {
					e.Insert([]rune{r})
					i += size - 1
				}
			}
			i++
		}
		redraw(e.String())
		if done {
			fmt.Println()
			return e.String()
		}
	}
	return e.String()
}

// decodeRune decodes one UTF-8 rune starting at buf[0]; returns size 0 when
// the bytes are invalid.
func decodeRune(buf []byte) (rune, int) {
	if len(buf) == 0 {
		return 0, 0
	}
	rs := []rune(string(buf))
	if len(rs) == 0 {
		return 0, 0
	}
	size := len(string(rs[0]))
	if size == 0 || size > len(buf) {
		return 0, 0
	}
	return rs[0], size
}

// handleCSI applies a CSI arrow/home/end/delete sequence; returns consumed
// bytes after "\x1b[".
func handleCSI(e *LineEditor, seq []byte) int {
	if len(seq) == 0 {
		return 0
	}
	switch seq[0] {
	case 'A', 'B': // up/down: no history, no-op
	case 'C': // right
		if e.Cursor < len(e.Runes) {
			e.Cursor++
		}
	case 'D': // left
		if e.Cursor > 0 {
			e.Cursor--
		}
	case 'H': // home
		e.Cursor = 0
	case 'F': // end
		e.Cursor = len(e.Runes)
	case '3':
		if len(seq) >= 2 && seq[1] == '~' {
			e.Forward()
			return 1
		}
	case '1', '7':
		if len(seq) >= 2 && (seq[1] == '~' || seq[1] == ';') { // home variants
			e.Cursor = 0
			return skipToTerminator(seq)
		}
	case '4', '8':
		if len(seq) >= 2 && seq[1] == '~' { // end variants
			e.Cursor = len(e.Runes)
			return 1
		}
	case '5', '6': // page up/down: no-op in single-line mode
	}
	return skipToTerminator(seq)
}

// handleSS3 handles application-cursor-mode arrows.
func handleSS3(e *LineEditor, c byte) int {
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
	return 1
}

func skipToTerminator(seq []byte) int {
	for j, b := range seq {
		if b >= 0x40 && b <= 0x7e {
			return j
		}
	}
	return len(seq) - 1
}

// redraw rewrites the edited line in place.
func redraw(s string) {
	fmt.Printf("\r\x1b[K%s", s)
}

// Choose prints prompt + numbered options and reads a valid choice in
// [1, options]. Non-TTY input and parse errors re-prompt; EOF returns 1.
func Choose(prompt string, options int) int {
	for {
		fmt.Printf("%s [1-%d] > ", prompt, options)
		line := ReadLine("")
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		v, err := strconv.Atoi(s)
		if err != nil || v < 1 || v > options {
			fmt.Println("无效选项，请输入数字。")
			continue
		}
		return v
	}
}

// Pause waits for Enter (or any line) so the player can read output.
func Pause(hint string) {
	fmt.Print(hint)
	ReadLine("")
}
