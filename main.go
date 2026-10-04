// Command wq is a keyboard-driven six-column task board for the terminal.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/ime"
	"github.com/IkuyaYamada/wq-tui/internal/ui"
)

// Keep the main goroutine, which runs Bubble Tea's update loop, on the main
// OS thread: macOS only allows input-source switching from there.
func init() { runtime.LockOSThread() }

func main() {
	dir := os.Getenv("WQ_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fail(err)
		}
		dir = filepath.Join(home, "wq")
	}
	b, err := board.Load(dir)
	if err != nil {
		fail(err)
	}
	var opts []ui.Option
	if os.Getenv("WQ_IME") != "off" {
		opts = append(opts, ui.WithIME(ime.System{}))
	}
	caret := &ui.Caret{}
	opts = append(opts, ui.WithCaret(caret))
	out := ui.CaretOutput{File: os.Stdout, C: caret}
	if _, err := tea.NewProgram(ui.New(dir, b, opts...), tea.WithAltScreen(), tea.WithOutput(out)).Run(); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "wq:", err)
	os.Exit(1)
}
