package ui

import (
	_ "embed"
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

// editorDoneMsg reports that the vim session opened on node id has ended.
// C-j / C-k may have gone round other nodes in focus meanwhile: visited
// lists every node it showed, last the one it ended on, and focus what ZF
// left nodes at, in the order it was pressed.
type editorDoneMsg struct {
	id       string
	strategy string
	last     string
	visited  []string
	focus    []focusChange
	err      error
}

type focusChange struct {
	id string
	on bool
}

// roundNode is a node C-j / C-k go round in vim, with its place in focus
// (0 for a node visited from outside it).
type roundNode struct {
	node  board.Node
	place int
}

// vimFile is where this wq and wq.vim pass a list for the session: the
// round going in (round), what happened coming out (result).
func vimFile(name string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("wq-%s-%d", name, os.Getpid()))
}

// vimResult is what wq.vim left in the result file.
type vimResult struct {
	last    string
	visited []string
	focus   []focusChange
}

// readVimResult takes what wq.vim wrote, one tab-separated fact a line:
// last <id>, visit <id>, focus <id> <1|0>. A missing file means nothing
// happened beyond the node vim opened on.
func readVimResult(path string) vimResult {
	var r vimResult
	data, err := os.ReadFile(path)
	if err != nil {
		return r
	}
	os.Remove(path)
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Split(line, "\t")
		switch {
		case f[0] == "last" && len(f) == 2:
			r.last = f[1]
		case f[0] == "visit" && len(f) == 2:
			r.visited = append(r.visited, f[1])
		case f[0] == "focus" && len(f) == 3:
			r.focus = append(r.focus, focusChange{id: f[1], on: f[2] == "1"})
		}
	}
	return r
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

// openNode opens node n's strategy.md in vim, full screen (see wq.vim),
// with place its place in focus and round the nodes C-j / C-k go round.
// Every strategy in the round gets its header brought up to date first:
// vim moves between them without coming back to wq.
func openNode(dir string, n board.Node, place int, round []roundNode) tea.Cmd {
	fail := func(err error) tea.Cmd { return func() tea.Msg { return editorDoneMsg{err: err} } }
	strategy, err := strategyPath(dir, n)
	if err != nil {
		return fail(err)
	}
	var lines []string
	for _, r := range round {
		path := strategy
		if r.node.ID != n.ID {
			if path, err = strategyPath(dir, r.node); err != nil {
				return fail(err)
			}
		}
		lines = append(lines, r.node.ID+"\t"+strconv.Itoa(r.place)+"\t"+path)
	}
	roundFile, resultFile := vimFile("round"), vimFile("result")
	if err := os.WriteFile(roundFile, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return fail(err)
	}
	os.Remove(resultFile)
	script, err := vimScriptPath()
	if err != nil {
		return fail(err)
	}
	vim := os.Getenv("WQ_VIM")
	if vim == "" {
		vim = "vim"
	}
	cmd := exec.Command(vim, "-S", script, strategy)
	cmd.Env = append(os.Environ(),
		"WQ_START="+n.ID,
		"WQ_FOCUS="+strconv.Itoa(place),
		"WQ_ROUND="+roundFile,
		"WQ_RESULT="+resultFile,
	)
	return tea.Exec(vimCmd{cmd}, func(err error) tea.Msg {
		os.Remove(roundFile)
		r := readVimResult(resultFile)
		return editorDoneMsg{id: n.ID, strategy: strategy, last: r.last, visited: r.visited, focus: r.focus, err: err}
	})
}
