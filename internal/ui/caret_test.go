package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

// caretAt reads back the screen cell the caret was put on.
func caretAt(t *testing.T, c *Caret) (x, y int) {
	t.Helper()
	if _, err := fmt.Sscanf(c.seq(), "\x1b[%d;%dH", &y, &x); err != nil {
		t.Fatalf("no caret: %q", c.seq())
	}
	return x - 1, y - 1
}

// cellAt is what the view shows from screen cell (x, y) on.
func cellAt(view string, x, y int) string {
	line := ansi.Strip(strings.Split(view, "\n")[y])
	return ansi.Cut(line, x, x+ansi.StringWidth(line))
}

func TestCaretFollowsTyping(t *testing.T) {
	c := &Caret{}
	m := New(t.TempDir(), &board.Board{}, WithCaret(c))
	n, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = n.(Model)

	// The title prompt: right after what is typed so far.
	m = press(t, m, "a", "設計")
	view := m.View()
	x, y := caretAt(t, c)
	if got := cellAt(view, 0, y); !strings.HasPrefix(got, "title> 設計") || x != ansi.StringWidth("title> 設計") {
		t.Errorf("prompt caret at %d,%d on %q", x, y, got)
	}
	m = press(t, m, "<enter>")
	if m.View(); c.seq() != "" {
		t.Errorf("the caret should be let go on the board: %q", c.seq())
	}

	// The editor: on the character the cursor stands on, past a wrapped
	// line of Japanese and onto the second line.
	m = press(t, m, "K", "i")
	long := strings.Repeat("あいうえお", 12)
	m.edit.SetValue(long + "\nかきくけこ")
	m.edit, _ = m.edit.Update(tea.KeyMsg{Type: tea.KeyCtrlHome})
	m.edit, _ = m.edit.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.edit, _ = m.edit.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.edit, _ = m.edit.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.edit, _ = m.edit.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.edit, _ = m.edit.Update(tea.KeyMsg{Type: tea.KeyRight})
	view = m.View()
	t.Log("\n" + view)
	x, y = caretAt(t, c)
	if got := cellAt(view, x, y); !strings.HasPrefix(got, "くけこ") {
		t.Errorf("editor caret at %d,%d on %q", x, y, got)
	}
}

func TestVimGetsTheTerminal(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "tty")
	if err != nil {
		t.Fatal(err)
	}
	out := CaretOutput{File: f, C: &Caret{}}

	// vim gets the file itself, not a pipe behind the wrapper.
	var cmd vimCmd
	cmd.Cmd = &exec.Cmd{}
	cmd.SetStdout(out)
	if cmd.Stdout != f {
		t.Errorf("vim's stdout is %T, want the terminal", cmd.Stdout)
	}
}
