package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

type editorDoneMsg struct {
	id       string
	strategy string
	thread   string
	appended string
	err      error
}

func nodeFiles(dir string, n board.Node) (strategy, thread string, err error) {
	d := board.NodeDir(dir, n.ID)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", "", err
	}
	strategy = filepath.Join(d, "strategy.md")
	thread = filepath.Join(d, "thread.md")
	if _, err := os.Stat(strategy); os.IsNotExist(err) {
		if err := syncStrategyTitle(strategy, n.Title); err != nil {
			return "", "", err
		}
	}
	if _, err := os.Stat(thread); os.IsNotExist(err) {
		if err := os.WriteFile(thread, nil, 0o644); err != nil {
			return "", "", err
		}
	}
	return strategy, thread, nil
}

// appendThread adds text to the end of thread.md, keeping one blank line
// between entries, and returns exactly what was written.
func appendThread(path, text string) (string, error) {
	cur, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := string(cur)
	sep := ""
	switch {
	case s == "":
	case strings.HasSuffix(s, "\n\n"):
	case strings.HasSuffix(s, "\n"):
		sep = "\n"
	default:
		sep = "\n\n"
	}
	add := sep + text
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.WriteString(add)
	return add, err
}

// dropUnused removes a timestamp heading that was never written under.
func dropUnused(path, appended string) error {
	cur, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if appended == "" || !strings.HasSuffix(string(cur), appended) {
		return nil
	}
	return os.WriteFile(path, cur[:len(cur)-len(appended)], 0o644)
}

func stamp() string { return time.Now().Format("2006-01-02 15:04") }

// openEditor opens strategy (left) and thread (right) side by side in vim,
// with the cursor under a fresh timestamp heading at the end of the thread.
// The strategy header carries the node's title, so editing it renames the
// node. Inside this session only, q in normal mode saves and returns.
func openEditor(dir string, n board.Node) tea.Cmd {
	strategy, thread, err := nodeFiles(dir, n)
	if err == nil {
		err = syncStrategyTitle(strategy, n.Title)
	}
	if err != nil {
		return func() tea.Msg { return editorDoneMsg{err: err} }
	}
	appended, err := appendThread(thread, "## "+stamp()+"\n\n")
	if err != nil {
		return func() tea.Msg { return editorDoneMsg{err: err} }
	}
	vim := os.Getenv("WQ_VIM")
	if vim == "" {
		vim = "vim"
	}
	cmd := exec.Command(vim, "-O", strategy, thread,
		"-c", "nnoremap <silent> q :wqa<CR>",
		"-c", "wincmd l",
		"-c", "normal! G",
	)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{id: n.ID, strategy: strategy, thread: thread, appended: appended, err: err}
	})
}
