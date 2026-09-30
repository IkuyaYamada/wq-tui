package ui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/thread"
)

// The detail screen shows one node: strategy on the left, the thread's
// entries on the right with the selected entry's full text below the list.

func (m *Model) detailNode() *board.Node { return m.b.Node(m.detailID) }

func (m *Model) nodeDir() string { return board.NodeDir(m.dir, m.detailID) }

func (m *Model) openDetail(n *board.Node) {
	m.detailID = n.ID
	m.mode = modeDetail
	m.trashed = nil
	m.reloadDetail()
	m.entrySel = len(m.entries) - 1
}

// reloadDetail rereads the thread and strategy from disk. Selecting an
// entry by path keeps the selection on it; "" keeps the index.
func (m *Model) reloadDetail(selectPath ...string) {
	n := m.detailNode()
	if n == nil {
		m.mode = modeNormal
		return
	}
	entries, err := thread.List(m.nodeDir())
	if err != nil {
		m.msg = "thread: " + err.Error()
	}
	m.entries = entries
	m.strategy = strategyBody(m.dir, *n)
	if len(selectPath) > 0 {
		for i, e := range entries {
			if e.Path == selectPath[0] {
				m.entrySel = i
			}
		}
	}
	m.entrySel = max(0, min(m.entrySel, len(m.entries)-1))
}

func (m *Model) keyDetail(k tea.KeyMsg) tea.Cmd {
	n := m.detailNode()
	switch k.String() {
	case "j", "down":
		m.entrySel = min(m.entrySel+1, len(m.entries)-1)
	case "k", "up":
		m.entrySel = max(m.entrySel-1, 0)
	case "g":
		m.entrySel = 0
	case "G":
		m.entrySel = len(m.entries) - 1
	case "a", "o":
		return newEntry(m.dir, *n)
	case "enter", "e", "i":
		if len(m.entries) == 0 {
			return newEntry(m.dir, *n)
		}
		return editEntry(m.entries[m.entrySel].Path, false)
	case "s":
		return editStrategy(m.dir, *n)
	case "x", "d":
		if len(m.entries) == 0 {
			return nil
		}
		trashed, err := thread.Trash(m.nodeDir(), m.entries[m.entrySel])
		if err != nil {
			m.msg = "thread: " + err.Error()
			return nil
		}
		m.trashed = append(m.trashed, trashed)
		m.reloadDetail()
		m.msg = "entry deleted — u to restore"
	case "u":
		if len(m.trashed) == 0 {
			m.msg = "nothing to restore"
			return nil
		}
		last := m.trashed[len(m.trashed)-1]
		m.trashed = m.trashed[:len(m.trashed)-1]
		if err := thread.Restore(m.nodeDir(), last); err != nil {
			m.msg = "thread: " + err.Error()
			return nil
		}
		m.reloadDetail(filepath.Join(thread.Dir(m.nodeDir()), filepath.Base(last)))
	case " ":
		m.cursorTo(n)
		m.reloadDetail(m.toggleDone())
	case "esc", "q", "ctrl+c":
		m.mode = modeNormal
		m.cursorTo(n)
	}
	return nil
}

var (
	paneTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Bold(true)
	entrySelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	entryTimeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	ruleStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

// wrap breaks text into lines no wider than w cells, splitting anywhere so
// Japanese without spaces wraps too.
func wrap(text string, w int) []string {
	if w <= 0 {
		return nil
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		cur, cw := strings.Builder{}, 0
		for _, r := range line {
			rw := runewidth.RuneWidth(r)
			if cw+rw > w {
				out = append(out, cur.String())
				cur.Reset()
				cw = 0
			}
			cur.WriteRune(r)
			cw += rw
		}
		out = append(out, cur.String())
	}
	return out
}

// pane is a column of already-sized lines; styled holds the rendered form
// of each (plain text may carry styles, so width is measured on plain).
type pane struct {
	plain  []string
	styled []string
}

func (p *pane) add(text string, st lipgloss.Style, styled bool) {
	p.plain = append(p.plain, text)
	if styled {
		text = st.Render(text)
	}
	p.styled = append(p.styled, text)
}

func (m Model) viewDetail(h int) []string {
	leftW := max(m.width*2/5, 20)
	rightW := max(m.width-leftW-3, 20)

	var left pane
	left.add(runewidth.Truncate("strategy", leftW, "…"), paneTitleStyle, true)
	if m.strategy == "" {
		left.add("s で strategy を書く", dimStyle, true)
	}
	for _, l := range wrap(m.strategy, leftW) {
		left.add(l, lipgloss.Style{}, false)
	}

	var right pane
	right.add("thread", paneTitleStyle, true)
	if len(m.entries) == 0 {
		right.add("a でエントリを追加", dimStyle, true)
	}
	listH := min(len(m.entries), max(h/3, 3))
	first := max(0, min(m.entrySel-listH/2, len(m.entries)-listH))
	for i := first; i < first+listH && i < len(m.entries); i++ {
		e := m.entries[i]
		marker := "  "
		if i == m.entrySel {
			marker = "▶ "
		}
		stamp := e.Time.Format("01/02 15:04")
		summary := runewidth.Truncate(e.Summary(), rightW-len(marker)-len(stamp)-2, "…")
		line := marker + stamp + "  " + summary
		if i == m.entrySel {
			right.add(line, entrySelStyle, true)
		} else {
			right.plain = append(right.plain, line)
			right.styled = append(right.styled, marker+entryTimeStyle.Render(stamp)+"  "+summary)
		}
	}
	if len(m.entries) > 0 {
		right.add(strings.Repeat("─", rightW), ruleStyle, true)
		for _, l := range wrap(strings.TrimRight(m.entries[m.entrySel].Body, "\n"), rightW) {
			right.add(l, lipgloss.Style{}, false)
		}
	}

	body := make([]string, h)
	for i := range body {
		var l, r string
		if i < len(left.plain) {
			l = left.styled[i] + strings.Repeat(" ", max(leftW-runewidth.StringWidth(left.plain[i]), 0))
		} else {
			l = strings.Repeat(" ", leftW)
		}
		if i < len(right.plain) {
			r = right.styled[i]
		}
		body[i] = l + ruleStyle.Render(" │ ") + r
	}
	return body
}
