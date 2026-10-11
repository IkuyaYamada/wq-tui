package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

// The list (L) shows every node's whole title, in reading order, so the
// board can be read through without titles cut short on their cards.

var (
	listCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	listTitleStyle  = lipgloss.NewStyle()
	listDoneStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Strikethrough(true)
	listNoteStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("108")).Italic(true)
	listBreakStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
)

// listNodes is the board's nodes in reading order.
func (m *Model) listNodes() []board.Node {
	c := m.b.Clone()
	c.Sort()
	return c.Nodes
}

// startList opens the list on the node under the cursor, or the first one
// after it.
func (m *Model) startList() {
	nodes := m.listNodes()
	if len(nodes) == 0 {
		m.msg = "no nodes yet"
		return
	}
	m.listAt = len(nodes) - 1
	for i, n := range nodes {
		if n.Row > m.row || n.Row == m.row && n.Col >= m.col {
			m.listAt = i
			break
		}
	}
	m.listScroll = 0
	m.mode = modeList
	m.listFollow()
}

func (m *Model) keyList(key string) {
	nodes := m.listNodes()
	switch key {
	case "j", "down":
		m.listAt = min(m.listAt+1, len(nodes)-1)
	case "k", "up":
		m.listAt = max(m.listAt-1, 0)
	case "ctrl+d":
		m.listAt = min(m.listAt+10, len(nodes)-1)
	case "ctrl+u":
		m.listAt = max(m.listAt-10, 0)
	case "G":
		m.listAt = len(nodes) - 1
	case "g":
		m.listAt = 0
	case "enter", "esc", "L", "q", "ctrl+c":
		// Back to the board, on the node the list was at.
		if m.listAt >= 0 && m.listAt < len(nodes) {
			m.row, m.col = nodes[m.listAt].Row, nodes[m.listAt].Col
		}
		m.mode = modeNormal
		return
	}
	m.listFollow()
}

// listLines lays out the list: one entry per node, the title wrapped to
// the screen's width under its row number and frame, a done node's
// comment below it, and session breaks between rows. at and end are the
// lines the entry under the cursor spans.
func (m *Model) listLines() (lines []string, at, end int) {
	w := max(m.width-1, 20)
	nodes := m.listNodes()
	lastRow := -1
	for i, n := range nodes {
		for r := max(lastRow, 0); lastRow >= 0 && r < n.Row; r++ {
			if br, ok := m.b.BreakAfter(r); ok {
				label := br.Label
				if label == "" {
					label = "break"
				}
				lines = append(lines, listBreakStyle.Render(runewidth.Truncate("── "+label+" "+strings.Repeat("─", w), w, "")))
			}
		}
		lastRow = n.Row

		marker, st := "  ", listTitleStyle
		if n.Done {
			st = listDoneStyle
		}
		if i == m.listAt {
			marker = listCursorStyle.Render("▶ ")
			if !n.Done {
				st = listCursorStyle
			}
			at = len(lines)
		}
		head := marker + dimStyle.Render(fmt.Sprintf("%3d ", n.Row+1))
		if gi := m.b.GroupOf(n.ID); gi >= 0 && m.b.Groups[gi].Title != "" {
			g := m.b.Groups[gi]
			head += lipgloss.NewStyle().Foreground(framePalette[frameColor(g)][1]).Render("["+runewidth.Truncate(g.Title, 16, "…")+"]") + " "
		}
		indent := lipgloss.Width(head) // later lines hang under the title
		label := n.Title
		if n.Done {
			label = "✓ " + label
		}
		for k, row := range wrapWidth(label, w-indent) {
			pre := strings.Repeat(" ", indent)
			if k == 0 {
				pre = head
			}
			lines = append(lines, pre+st.Render(row))
		}
		if n.Done && n.DoneNote != "" {
			lines = append(lines, strings.Repeat(" ", indent)+listNoteStyle.Render(runewidth.Truncate("— "+n.DoneNote, w-indent, "…")))
		}
		if i == m.listAt {
			end = len(lines)
		}
	}
	return lines, at, end
}

// listFollow scrolls the list so the entry under the cursor is on screen.
func (m *Model) listFollow() {
	lines, at, end := m.listLines()
	h := m.bodyHeight()
	if at < m.listScroll {
		m.listScroll = at
	}
	if end > m.listScroll+h {
		m.listScroll = end - h
	}
	m.listScroll = max(0, min(m.listScroll, max(len(lines)-h, 0)))
}

func (m Model) viewList() []string {
	lines, _, _ := m.listLines()
	out := make([]string, m.bodyHeight())
	for i := range out {
		if j := m.listScroll + i; j < len(lines) {
			out[i] = lines[j]
		}
	}
	return out
}

// wrapWidth wraps s into lines no wider than w.
func wrapWidth(s string, w int) []string {
	w = max(w, 4)
	var rows []string
	for runewidth.StringWidth(s) > w {
		head := runewidth.Truncate(s, w, "")
		if head == "" {
			break
		}
		rows = append(rows, head)
		s = s[len(head):]
	}
	return append(rows, s)
}
