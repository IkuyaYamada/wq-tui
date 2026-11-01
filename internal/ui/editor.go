package ui

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

//go:embed wq.vim
var nodeVimScript []byte

// editorDoneMsg reports that the vim session on a node has ended. step is
// 1 or -1 when vim was left with C-j / C-k to open the next or previous
// node in focus; focus is what ZF left the node at, if it was pressed.
type editorDoneMsg struct {
	id       string
	strategy string
	step     int
	focus    *bool
	err      error
}

// vimFocus is what wq.vim is told about focus when a node opens: the
// node's place in it (0 when out of it) and how many other nodes C-j /
// C-k can go to.
type vimFocus struct {
	place, others int
}

// focusFile is where ZF in vim leaves its answer for this wq.
func focusFile() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("wq-focus-%d", os.Getpid()))
}

// readFocusFile takes ZF's answer, if there is one, and clears it.
func readFocusFile(path string) *bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	os.Remove(path)
	on := strings.TrimSpace(string(data)) == "1"
	return &on
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
// carries the node's current title, url and done comment in its header.
func strategyPath(dir string, n board.Node) (string, error) {
	d := board.NodeDir(dir, n.ID)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(d, "strategy.md")
	var done *string
	if n.Done {
		done = &n.DoneNote
	}
	return path, syncStrategyHeader(path, strategyHeader{title: n.Title, url: n.URL, done: done, session: n.Session})
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

// openNode opens the node's strategy.md in vim, full screen (see wq.vim).
func openNode(dir string, n board.Node, f vimFocus) tea.Cmd {
	fail := func(err error) tea.Cmd { return func() tea.Msg { return editorDoneMsg{err: err} } }
	strategy, err := strategyPath(dir, n)
	if err != nil {
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
	ff := focusFile()
	os.Remove(ff)
	cmd := exec.Command(vim, "-S", script, strategy)
	cmd.Env = append(os.Environ(),
		"WQ_FOCUS="+strconv.Itoa(f.place),
		"WQ_FOCUS_OTHERS="+strconv.Itoa(f.others),
		"WQ_FOCUS_FILE="+ff,
	)
	return tea.Exec(vimCmd{cmd}, func(err error) tea.Msg {
		step, err := exitStep(err)
		return editorDoneMsg{id: n.ID, strategy: strategy, step: step, focus: readFocusFile(ff), err: err}
	})
}
