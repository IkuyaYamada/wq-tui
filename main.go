// Command wq is a keyboard-driven six-column task board for the terminal.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/ui"
)

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
	if _, err := tea.NewProgram(ui.New(dir, b), tea.WithAltScreen()).Run(); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "wq:", err)
	os.Exit(1)
}
