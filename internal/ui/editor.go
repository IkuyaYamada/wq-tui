package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/thread"
)

// editorDoneMsg reports a finished vim session on a node's strategy and,
// optionally, one of its thread entries.
type editorDoneMsg struct {
	id       string // node whose strategy was open
	strategy string
	entry    string // thread entry that was open beside it, if any
	isNew    bool   // the entry was created for this session
	err      error
}

// strategyPath makes sure the node's directory exists and that strategy.md
// carries the node's current title in its header.
func strategyPath(dir string, n board.Node) (string, error) {
	d := board.NodeDir(dir, n.ID)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(d, "strategy.md")
	return path, syncStrategyTitle(path, n.Title)
}

// strategyBody is strategy.md without its header, for display.
func strategyBody(dir string, n board.Node) string {
	data, err := os.ReadFile(filepath.Join(board.NodeDir(dir, n.ID), "strategy.md"))
	if err != nil {
		return ""
	}
	_, body, _ := splitFrontmatter(string(data))
	return strings.Trim(body, "\n")
}

// openSplit opens the node in vim the way the detail screen shows it:
// strategy on the left and, when given, a thread entry on the right. The
// cursor starts in the entry when focusEntry is set (in insert mode for a
// new one), otherwise in the strategy; C-w w moves between them. Inside this
// session only, q in normal mode saves both and returns to wq.
func openSplit(dir string, n board.Node, entry string, isNew, focusEntry bool) tea.Cmd {
	strategy, err := strategyPath(dir, n)
	if err != nil {
		return func() tea.Msg { return editorDoneMsg{err: err} }
	}
	vim := os.Getenv("WQ_VIM")
	if vim == "" {
		vim = "vim"
	}
	args := []string{strategy}
	if entry != "" {
		args = []string{"-O", strategy, entry}
	}
	args = append(args, "-c", "nnoremap <silent> q :wqa<CR>")
	if entry != "" && focusEntry {
		args = append(args, "-c", "wincmd l")
		if isNew {
			args = append(args, "-c", "startinsert")
		}
	}
	return tea.ExecProcess(exec.Command(vim, args...), func(err error) tea.Msg {
		return editorDoneMsg{id: n.ID, strategy: strategy, entry: entry, isNew: isNew, err: err}
	})
}

// newEntry creates an empty entry and opens it beside the strategy in
// insert mode; it is removed again if nothing was written.
func newEntry(dir string, n board.Node) tea.Cmd {
	e, err := thread.Add(board.NodeDir(dir, n.ID), time.Now(), "")
	if err != nil {
		return func() tea.Msg { return editorDoneMsg{err: err} }
	}
	return openSplit(dir, n, e.Path, true, true)
}

// dropIfBlank removes a new entry that was left empty.
func dropIfBlank(path string) {
	data, err := os.ReadFile(path)
	if err == nil && strings.TrimSpace(string(data)) == "" {
		os.Remove(path)
	}
}
