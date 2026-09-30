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

// editorDoneMsg reports a finished vim session on either a node's strategy
// or one thread entry.
type editorDoneMsg struct {
	id       string // node whose strategy was edited
	strategy string
	entry    string // thread entry that was edited
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

// runVim opens path in vim. Inside this session only, q in normal mode
// saves and returns to wq.
func runVim(path string, insert bool, done func(error) tea.Msg) tea.Cmd {
	vim := os.Getenv("WQ_VIM")
	if vim == "" {
		vim = "vim"
	}
	args := []string{path, "-c", "nnoremap <silent> q :wqa<CR>"}
	if insert {
		args = append(args, "-c", "startinsert")
	}
	return tea.ExecProcess(exec.Command(vim, args...), done)
}

// editStrategy opens strategy.md; its title: header renames the node.
func editStrategy(dir string, n board.Node) tea.Cmd {
	path, err := strategyPath(dir, n)
	if err != nil {
		return func() tea.Msg { return editorDoneMsg{err: err} }
	}
	return runVim(path, false, func(err error) tea.Msg {
		return editorDoneMsg{id: n.ID, strategy: path, err: err}
	})
}

func editEntry(path string, isNew bool) tea.Cmd {
	return runVim(path, isNew, func(err error) tea.Msg {
		return editorDoneMsg{entry: path, isNew: isNew, err: err}
	})
}

// newEntry creates an empty entry and opens it in insert mode; it is
// removed again if nothing was written.
func newEntry(dir string, n board.Node) tea.Cmd {
	e, err := thread.Add(board.NodeDir(dir, n.ID), time.Now(), "")
	if err != nil {
		return func() tea.Msg { return editorDoneMsg{err: err} }
	}
	return editEntry(e.Path, true)
}

// dropIfBlank removes a new entry that was left empty.
func dropIfBlank(path string) {
	data, err := os.ReadFile(path)
	if err == nil && strings.TrimSpace(string(data)) == "" {
		os.Remove(path)
	}
}
