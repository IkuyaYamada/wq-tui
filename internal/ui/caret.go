package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
)

// Caret is where the terminal's own (hidden) cursor should rest while text
// is being typed: at the editor's or the prompt's cursor. The terminal draws
// an input method's unconverted text there, so Japanese being composed
// shows up where it will land instead of in the bottom-left corner.
//
// Bubble Tea v1 leaves the cursor at the bottom of the screen after every
// frame. Each frame starts by moving to the top-left, so moving the cursor
// once a frame is written does not disturb the next one.
type Caret struct {
	pos atomic.Value // string: the escape sequence to move there, "" for none
}

// WithCaret has View report the typing position to c.
func WithCaret(c *Caret) Option { return func(m *Model) { m.caret = c } }

func (c *Caret) set(x, y int, ok bool) {
	if c == nil {
		return
	}
	seq := ""
	if ok {
		seq = fmt.Sprintf("\x1b[%d;%dH", y+1, x+1)
	}
	c.pos.Store(seq)
}

func (c *Caret) seq() string {
	s, _ := c.pos.Load().(string)
	return s
}

// CaretOutput is the terminal output that puts the cursor at c after
// everything written to it. It stays an *os.File so Bubble Tea still sees
// a terminal it can size.
type CaretOutput struct {
	*os.File
	C *Caret
}

func (o CaretOutput) Write(p []byte) (int, error) {
	out := p
	if seq := o.C.seq(); seq != "" {
		out = append(out[:len(out):len(out)], seq...)
	}
	if _, err := o.File.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}

// tty is the terminal behind w, so a program run in its place gets the
// terminal itself rather than a pipe.
func tty(w io.Writer) io.Writer {
	if o, ok := w.(CaretOutput); ok {
		return o.File
	}
	return w
}

// caretStyle marks the editor's cursor in a scratch rendering so it can be
// found: reverse video, emitted whatever the real terminal supports.
var caretStyle = func() lipgloss.Style {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	return r.NewStyle()
}()

// editCaret finds the editor's cursor in a scratch rendering of it: its row
// within the view (the textarea scrolls on its own) and its column.
func (m *Model) editCaret() (x, y int, ok bool) {
	ta := m.edit
	ta.Cursor.Blink = false
	ta.Cursor.Style = caretStyle
	for i, row := range strings.Split(ta.View(), "\n") {
		if j := strings.Index(row, "\x1b[7m"); j >= 0 {
			return ansi.StringWidth(row[:j]), i, true
		}
	}
	return 0, 0, false
}

// placeCaret reports where typing lands on screen, for the input method.
// footerY is the footer's screen line.
func (m Model) placeCaret(footerY int) {
	switch m.mode {
	case modeEdit:
		p, ok := m.editBox()
		if !ok {
			break
		}
		if x, y, ok := m.editCaret(); ok && y < p.h-2 {
			m.caret.set(p.x+2+x, 1+p.y+1+y, true) // 1: the header line
			return
		}
	case modeInput:
		v := []rune(m.input.Value())
		pos := min(m.input.Position(), len(v))
		x := runewidth.StringWidth(m.input.Prompt) + runewidth.StringWidth(string(v[:pos]))
		m.caret.set(x, footerY, true)
		return
	}
	m.caret.set(0, 0, false)
}
