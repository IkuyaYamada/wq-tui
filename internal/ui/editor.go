package ui

import (
	_ "embed"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/thread"
)

//go:embed wq.vim
var nodeVimScript []byte

// editorDoneMsg reports that the vim session on a node has ended. step is
// 1 or -1 when vim was left with C-j / C-k to open the next or previous node.
type editorDoneMsg struct {
	id       string
	strategy string
	step     int
	err      error
}

// Exit codes wq.vim leaves with on C-j / C-k (after saving everything).
const (
	exitNextNode = 3
	exitPrevNode = 4
)

// exitStep turns vim's exit status into a step to the next (1) or previous
// (-1) node, clearing err when it only carried that request.
func exitStep(err error) (int, error) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		switch ee.ExitCode() {
		case exitNextNode:
			return 1, nil
		case exitPrevNode:
			return -1, nil
		}
	}
	return 0, err
}

// strategyPath makes sure the node's directory exists and that strategy.md
// carries the node's current title and url in its header.
func strategyPath(dir string, n board.Node) (string, error) {
	d := board.NodeDir(dir, n.ID)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(d, "strategy.md")
	return path, syncStrategyHeader(path, n.Title, n.URL)
}

// vimScriptPath writes the embedded layout script where vim can source it.
func vimScriptPath() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	dir := filepath.Join(cache, "wq-tui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "wq.vim")
	return path, os.WriteFile(path, nodeVimScript, 0o644)
}

// vimCmd runs vim on the terminal itself: given the wrapped output, a
// plain exec.Cmd would put a pipe in between and vim would not draw.
type vimCmd struct{ *exec.Cmd }

func (c vimCmd) SetStdin(r io.Reader)  { c.Stdin = r }
func (c vimCmd) SetStdout(w io.Writer) { c.Stdout = tty(w) }
func (c vimCmd) SetStderr(w io.Writer) { c.Stderr = w }

// vimString quotes s as a single-quoted Vim string.
func vimString(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// openNode opens the node in vim: strategy on the left, the thread index
// top right and the picked entry below it (see wq.vim).
func openNode(dir string, n board.Node) tea.Cmd {
	fail := func(err error) tea.Cmd { return func() tea.Msg { return editorDoneMsg{err: err} } }
	strategy, err := strategyPath(dir, n)
	if err != nil {
		return fail(err)
	}
	nodeDir := board.NodeDir(dir, n.ID)
	if err := thread.Migrate(nodeDir); err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(thread.Dir(nodeDir), 0o755); err != nil {
		return fail(err)
	}
	script, err := vimScriptPath()
	if err != nil {
		return fail(err)
	}
	vim := os.Getenv("WQ_VIM")
	if vim == "" {
		vim = "vim"
	}
	cmd := exec.Command(vim,
		"--cmd", "let g:wq_thread_dir = "+vimString(thread.Dir(nodeDir)),
		"-S", script,
		strategy,
	)
	return tea.Exec(vimCmd{cmd}, func(err error) tea.Msg {
		step, err := exitStep(err)
		if derr := thread.DropBlank(nodeDir); err == nil {
			err = derr
		}
		return editorDoneMsg{id: n.ID, strategy: strategy, step: step, err: err}
	})
}
