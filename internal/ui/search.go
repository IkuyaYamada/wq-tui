package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

// Search (/) finds nodes by their title, completion comment or frame
// title, ignoring case. The cursor jumps to the first match as the query
// is typed; n / N go on to the next / previous match afterwards.

// startSearch opens the search prompt, remembering where the cursor was
// so Esc can put it back.
func (m *Model) startSearch() tea.Cmd {
	m.inputSearch = true
	m.searchOrig = [2]int{m.row, m.col}
	return m.startInput("/", "")
}

// keySearch handles a key on the search prompt.
func (m *Model) keySearch(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "enter", "esc", "ctrl+c":
		m.input.Blur()
		m.mode = modeNormal
		m.inputSearch = false
		m.toASCII()
		q := strings.TrimSpace(m.input.Value())
		if k.String() != "enter" || q == "" {
			m.row, m.col = m.searchOrig[0], m.searchOrig[1]
			return nil
		}
		if m.searchJump(q, 0) {
			m.searchQuery = q
		} else {
			m.row, m.col = m.searchOrig[0], m.searchOrig[1]
		}
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(k)
	// Incremental: look again from where the search began.
	m.row, m.col = m.searchOrig[0], m.searchOrig[1]
	if q := strings.TrimSpace(m.input.Value()); q != "" {
		m.searchJump(q, 0)
	}
	return cmd
}

// searchNext moves to the next (step 1) or previous (-1) match of the
// last query, wrapping around the board.
func (m *Model) searchNext(step int) {
	if m.searchQuery == "" {
		m.msg = "no search yet — / to search"
		return
	}
	m.searchJump(m.searchQuery, step)
}

// searchJump moves the cursor to a node matching q in reading order:
// step 0 takes the cursor's own cell or the first match after it, 1 the
// first one strictly after, -1 the last one strictly before. It wraps
// around the board and reports which match the cursor is on.
func (m *Model) searchJump(q string, step int) bool {
	hits := m.searchHits(q)
	if len(hits) == 0 {
		m.msg = "no match: " + q
		return false
	}
	cur := m.row*board.Cols + m.col
	pos := func(n board.Node) int { return n.Row*board.Cols + n.Col }
	at := -1
	switch step {
	case -1:
		for i := len(hits) - 1; i >= 0; i-- {
			if pos(hits[i]) < cur {
				at = i
				break
			}
		}
		if at < 0 {
			at = len(hits) - 1
		}
	default:
		for i, n := range hits {
			if pos(n) > cur || step == 0 && pos(n) == cur {
				at = i
				break
			}
		}
		if at < 0 {
			at = 0
		}
	}
	m.frameSel = ""
	m.cursorTo(&hits[at])
	m.msg = fmt.Sprintf("/%s  %d/%d", q, at+1, len(hits))
	return true
}

// searchHits is every node matching q, in reading order.
func (m *Model) searchHits(q string) []board.Node {
	q = strings.ToLower(q)
	var hits []board.Node
	for _, n := range m.listNodes() {
		text := n.Title + "\n" + n.DoneNote
		if gi := m.b.GroupOf(n.ID); gi >= 0 {
			text += "\n" + m.b.Groups[gi].Title
		}
		if strings.Contains(strings.ToLower(text), q) {
			hits = append(hits, n)
		}
	}
	return hits
}
