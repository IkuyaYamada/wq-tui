package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/thread"
)

// selectedFrame is the frame the cursor stands on, if any.
func (m *Model) selectedFrame() *board.Group {
	if m.frameSel == "" {
		return nil
	}
	return m.b.Group(m.frameSel)
}

// focus is what the preview and its editor show: the selected frame (as a
// node carrying its id, title and url) or the node under the cursor.
func (m *Model) focus() *board.Node {
	if g := m.selectedFrame(); g != nil {
		return &board.Node{ID: g.ID, Title: g.Title, URL: g.URL, CreatedAt: g.CreatedAt, Row: m.row, Col: m.col}
	}
	return m.selected()
}

// frameNav steps onto and off frames: k from a member with the frame's
// line over it selects the frame, j comes back to the card, k again leaves
// it upward. It reports whether it handled the key.
func (m *Model) frameNav(d board.Dir) bool {
	if m.frameSel != "" {
		m.frameSel = ""
		switch d {
		case board.Up:
			if m.row > 0 {
				m.row--
			}
			return true
		case board.Down:
			return true
		}
		return false
	}
	if d != board.Up {
		return false
	}
	n := m.selected()
	if n == nil {
		return false
	}
	gi := m.b.GroupOf(n.ID)
	if gi < 0 {
		return false
	}
	if up := m.b.At(m.row-1, m.col); up != nil && m.b.GroupOf(up.ID) == gi {
		return false
	}
	m.frameSel = m.b.Groups[gi].ID
	return true
}

// keyFrame runs a key on the selected frame. done is false for keys that
// are not about frames; the frame is let go and the key acts as usual.
func (m *Model) keyFrame(key string, g *board.Group) (cmd tea.Cmd, done bool) {
	switch key {
	case "m":
		m.startMove(append([]string(nil), g.Members...))
	case "x":
		m.checkpoint()
		m.b.RemoveGroup(g.ID)
		m.frameSel = ""
		m.save()
	case " ":
		m.doneAll(g.Members)
	case "i":
		m.inputFrame = true
		return m.startInput("frame> ", g.Title), true
	case "enter":
		return openNode(m.dir, *m.focus()), true
	case "c":
		m.checkpoint()
		m.b.CycleColor(g.ID)
		m.save()
	case "esc":
		m.frameSel = ""
	case "K", "q", "ctrl+c":
		return nil, false
	default:
		m.frameSel = ""
		return nil, false
	}
	return nil, true
}

// doneAll completes every node in ids, or reopens them all when they are
// all done already, logging each in its thread.
func (m *Model) doneAll(ids []string) {
	all := true
	for _, id := range ids {
		if n := m.b.Node(id); n != nil && !n.Done {
			all = false
		}
	}
	m.checkpoint()
	now := time.Now()
	var changed []string
	for _, id := range ids {
		n := m.b.Node(id)
		if n == nil || n.Done == !all {
			continue
		}
		n.Done, n.DoneNote, n.DoneAt = !all, "", nil
		if n.Done {
			n.DoneAt = &now
		}
		changed = append(changed, id)
	}
	m.save()
	if m.stale {
		return
	}
	event := "Completed\n"
	if all {
		event = "Reopened\n"
	}
	for _, id := range changed {
		if _, err := thread.Add(board.NodeDir(m.dir, id), now, event); err != nil {
			m.msg = "thread: " + err.Error()
		}
	}
}

// decompose breaks the node under the cursor up into a frame (gs): the
// frame keeps its title and notes, and a first child takes its cell and
// asks for a title. Esc on that title undoes the whole thing.
func (m *Model) decompose() tea.Cmd {
	n := m.selected()
	if n == nil {
		return nil
	}
	now := time.Now()
	child := board.Node{ID: board.NewID(now), CreatedAt: now}
	m.prevCur = [2]int{m.row, m.col}
	m.checkpoint()
	if err := m.b.Decompose(n.ID, child); err != nil {
		m.undo = m.undo[:len(m.undo)-1]
		m.msg = err.Error()
		return nil
	}
	m.inputNew = true
	return m.startInput("title> ", "")
}

var crumbNodeStyle = lipgloss.NewStyle().Bold(true)

// breadcrumb names where the cursor is, in full, for the header: the frame
// (with its progress) and the node, as cards and frame lines cut titles
// short. It fits in room cells, giving the node's title priority.
func (m *Model) breadcrumb(room int) string {
	frame, node := "", ""
	frameSt := lipgloss.NewStyle()
	if g := m.selectedFrame(); g != nil {
		frame = "▸ " + g.Title
		if r := m.frameProgress(*g); r != "" {
			frame += " " + r
		}
		frameSt = lipgloss.NewStyle().Foreground(framePalette[frameColor(*g)][1])
	} else if n := m.selected(); n != nil {
		node = n.Title
		if gi := m.b.GroupOf(n.ID); gi >= 0 && m.b.Groups[gi].Title != "" {
			frame = m.b.Groups[gi].Title
			frameSt = lipgloss.NewStyle().Foreground(framePalette[frameColor(m.b.Groups[gi])][1])
		}
	}
	if room < 8 || (frame == "" && node == "") {
		return ""
	}
	sep := ""
	if frame != "" && node != "" {
		sep = " › "
	}
	nodeW := min(runewidth.StringWidth(node), room-runewidth.StringWidth(sep))
	if frame != "" {
		frameW := room - nodeW - runewidth.StringWidth(sep)
		if frameW < 6 && node != "" { // keep a little of the frame's name
			frameW = min(6, runewidth.StringWidth(frame))
			nodeW = room - frameW - runewidth.StringWidth(sep)
		}
		frame = runewidth.Truncate(frame, frameW, "…")
	}
	node = runewidth.Truncate(node, max(nodeW, 0), "…")
	out := "  "
	if frame != "" {
		out += frameSt.Render(frame)
	}
	out += dimStyle.Render(sep)
	if node != "" {
		out += crumbNodeStyle.Render(node)
	}
	return out
}

func (m *Model) frameProgress(g board.Group) string {
	done, total := 0, 0
	for _, id := range g.Members {
		if n := m.b.Node(id); n != nil {
			total++
			if n.Done {
				done++
			}
		}
	}
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", done, total)
}
