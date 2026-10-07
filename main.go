// Command wq is a keyboard-driven six-column task board for the terminal.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
	// Cards start at the density zc / zo last left them at on this
	// machine, or compact with WQ_COMPACT=1.
	if path := densityFile(); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			opts = append(opts, ui.WithDensity(strings.TrimSpace(string(data))))
		}
		opts = append(opts, ui.WithDensitySaver(func(name string) {
			if os.MkdirAll(filepath.Dir(path), 0o755) == nil {
				_ = os.WriteFile(path, []byte(name+"\n"), 0o644)
			}
		}))
	}
	if os.Getenv("WQ_COMPACT") == "1" {
		opts = append(opts, ui.WithCompact())
	}
	if os.Getenv("WQ_IME") != "off" {
		opts = append(opts, ui.WithIME(ime.System{}))
	}
	caret := &ui.Caret{}
	opts = append(opts, ui.WithCaret(caret))
	out := ui.CaretOutput{File: os.Stdout, C: caret}
	if _, err := tea.NewProgram(ui.New(dir, b, opts...), tea.WithAltScreen(), tea.WithOutput(out), tea.WithReportFocus()).Run(); err != nil {
		fail(err)
	}
}

// densityFile keeps the card density between runs: a view setting of this
// machine, so it stays out of the data directory.
func densityFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "wq", "density")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "wq:", err)
	os.Exit(1)
}
