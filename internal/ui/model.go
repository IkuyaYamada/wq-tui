// Package ui is the Bubble Tea front end: a six-column board driven by
// vim-style keys, with vim itself as the node editor.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

type mode int

const (
	modeNormal mode = iota
	modeInput
	modeMove
	modeConnect
)

const maxUndo = 200

type Model struct {
	dir  string
	b    *board.Board
	sel  string
	undo []*board.Board
	redo []*board.Board

	mode     mode
	input    textinput.Model
	inputNew bool   // the node being titled was just created; Esc discards it
	prevSel  string // selection to restore when a new node is discarded
	moveOrig *board.Board
	connFrom string

	width, height int
	scroll        int
	msg           string

	routes   []route
	routeKey string
}

func New(dir string, b *board.Board) Model {
	ti := textinput.New()
	ti.Prompt = "title> "
	ti.CharLimit = 200
	m := Model{dir: dir, b: b, input: ti, width: 120, height: 40}
	m.sel = b.Nearest(0, 0)
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m *Model) checkpoint() {
	m.undo = append(m.undo, m.b.Clone())
	if len(m.undo) > maxUndo {
		m.undo = m.undo[1:]
	}
	m.redo = nil
}

func (m *Model) save() {
	if err := board.Save(m.dir, m.b); err != nil {
		m.msg = "save failed: " + err.Error()
	}
}

func (m *Model) selected() *board.Node { return m.b.Node(m.sel) }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case editorDoneMsg:
		if msg.err != nil {
			m.msg = "vim: " + msg.err.Error()
		}
		if msg.thread != "" {
			if err := dropUnused(msg.thread, msg.appended); err != nil {
				m.msg = "thread: " + err.Error()
			}
		}
	case tea.KeyMsg:
		m.msg = ""
		switch m.mode {
		case modeNormal:
			cmd = m.keyNormal(msg)
		case modeInput:
			cmd = m.keyInput(msg)
		case modeMove:
			m.keyMove(msg)
		case modeConnect:
			m.keyConnect(msg)
		}
	default:
		if m.mode == modeInput {
			m.input, cmd = m.input.Update(msg)
		}
	}
	m.ensureVisible()
	m.refreshRoutes()
	return m, cmd
}

// refreshRoutes reroutes edges only when positions, edges or width changed;
// routing is the one expensive part of drawing.
func (m *Model) refreshRoutes() {
	if key := routeKey(m.b, m.width); key != m.routeKey {
		m.routes = routeEdges(m.b, newLayout(m.b, m.width))
		m.routeKey = key
	}
}

func dirOf(k string) (board.Dir, bool) {
	switch k {
	case "h", "left":
		return board.Left, true
	case "j", "down":
		return board.Down, true
	case "k", "up":
		return board.Up, true
	case "l", "right":
		return board.Right, true
	}
	return 0, false
}

func (m *Model) navigate(d board.Dir) {
	if next := m.b.Neighbor(m.sel, d); next != "" {
		m.sel = next
	}
}

func (m *Model) keyNormal(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	if d, ok := dirOf(key); ok {
		m.navigate(d)
		return nil
	}
	n := m.selected()
	switch key {
	case "q", "ctrl+c":
		return tea.Quit
	case "g":
		m.sel = m.b.Nearest(0, 0)
	case "G":
		m.sel = m.b.Nearest(m.b.MaxRow(), board.Cols-1)
	case "o", "O", "a", "n":
		return m.startNew(key)
	case "i":
		if n == nil {
			return nil
		}
		m.inputNew = false
		return m.startInput(n.Title)
	case "m":
		if n == nil {
			return nil
		}
		m.moveOrig = m.b.Clone()
		m.mode = modeMove
	case "c":
		if n == nil {
			return nil
		}
		m.connFrom = m.sel
		m.mode = modeConnect
	case " ":
		if n == nil {
			return nil
		}
		m.toggleDone()
	case "enter":
		if n == nil {
			return nil
		}
		return openEditor(m.dir, *n)
	case "d", "x":
		if n == nil {
			return nil
		}
		row, col := n.Row, n.Col
		m.checkpoint()
		_ = m.b.Delete(m.sel)
		m.sel = m.b.Nearest(row, col)
		m.save()
	case "u":
		m.history(&m.undo, &m.redo, "undo")
	case "ctrl+r":
		m.history(&m.redo, &m.undo, "redo")
	}
	return nil
}

func (m *Model) history(from, to *[]*board.Board, label string) {
	if len(*from) == 0 {
		m.msg = "nothing to " + label
		return
	}
	last := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	*to = append(*to, m.b.Clone())
	m.b = last
	if m.b.Node(m.sel) == nil {
		m.sel = m.b.Nearest(0, 0)
	}
	m.save()
}

// startNew creates an untitled node right away (so it shows on the board)
// and asks for its title. Esc on the prompt discards it again.
func (m *Model) startNew(key string) tea.Cmd {
	now := time.Now()
	nn := board.Node{ID: board.NewID(now), CreatedAt: now}
	m.prevSel = m.sel
	m.checkpoint()
	cur := m.selected()
	switch {
	case cur == nil:
		m.b.Add(nn, 0, 0)
	case key == "o":
		_ = m.b.InsertAfter(m.sel, nn)
	case key == "O":
		_ = m.b.InsertBefore(m.sel, nn)
	default:
		m.b.Add(nn, cur.Row, cur.Col)
	}
	m.sel = nn.ID
	m.inputNew = true
	return m.startInput("")
}

func (m *Model) startInput(value string) tea.Cmd {
	m.mode = modeInput
	m.input.SetValue(value)
	m.input.CursorEnd()
	return m.input.Focus()
}

func (m *Model) keyInput(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "enter":
		title := strings.TrimSpace(m.input.Value())
		m.input.Blur()
		m.mode = modeNormal
		if title == "" {
			if m.inputNew {
				m.discardNew()
			}
			return nil
		}
		n := m.selected()
		if n.Title != title {
			if !m.inputNew {
				m.checkpoint()
			}
			n = m.selected()
			n.Title = title
		}
		m.save()
		return nil
	case "esc", "ctrl+c":
		m.input.Blur()
		m.mode = modeNormal
		if m.inputNew {
			m.discardNew()
		}
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(k)
	return cmd
}

func (m *Model) discardNew() {
	if len(m.undo) > 0 {
		m.b = m.undo[len(m.undo)-1]
		m.undo = m.undo[:len(m.undo)-1]
	}
	m.sel = m.prevSel
}

func (m *Model) keyMove(k tea.KeyMsg) {
	key := k.String()
	if d, ok := dirOf(key); ok {
		if !m.b.MoveStep(m.sel, d) {
			m.msg = "can't move there"
		}
		return
	}
	switch key {
	case "enter", "m":
		m.mode = modeNormal
		orig, cur := m.moveOrig.Node(m.sel), m.selected()
		if orig.Row != cur.Row || orig.Col != cur.Col {
			m.undo = append(m.undo, m.moveOrig)
			m.redo = nil
			m.save()
		}
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.b = m.moveOrig
	}
}

func (m *Model) keyConnect(k tea.KeyMsg) {
	key := k.String()
	if d, ok := dirOf(key); ok {
		m.navigate(d)
		return
	}
	switch key {
	case "enter", "c":
		m.mode = modeNormal
		if m.sel == m.connFrom {
			return
		}
		m.checkpoint()
		added, err := m.b.ToggleEdge(m.connFrom, m.sel)
		if err != nil {
			m.b = m.undo[len(m.undo)-1]
			m.undo = m.undo[:len(m.undo)-1]
			m.msg = err.Error()
			m.sel = m.connFrom
			return
		}
		if added {
			m.msg = "connected"
		} else {
			m.msg = "disconnected"
		}
		m.save()
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.sel = m.connFrom
	}
}

// toggleDone flips the done flag and leaves a line in the node's thread.
func (m *Model) toggleDone() {
	m.checkpoint()
	n := m.selected()
	n.Done = !n.Done
	event := "Reopened"
	n.DoneAt = nil
	if n.Done {
		now := time.Now()
		n.DoneAt = &now
		event = "Completed"
	}
	m.save()
	if _, thread, err := nodeFiles(m.dir, *n); err == nil {
		_, err = appendThread(thread, "- "+stamp()+" "+event+"\n")
		if err != nil {
			m.msg = "thread: " + err.Error()
		}
	}
}

func (m *Model) bodyHeight() int { return max(m.height-2, 1) }

func (m *Model) ensureVisible() {
	n := m.selected()
	if n == nil {
		m.scroll = 0
		return
	}
	l := newLayout(m.b, m.width)
	top := l.rowY[n.Row]
	bottom := top + cardH + 1
	h := m.bodyHeight()
	if top < m.scroll {
		m.scroll = top
	}
	if bottom > m.scroll+h {
		m.scroll = bottom - h
	}
	m.scroll = max(0, min(m.scroll, max(l.height-h, 0)))
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	modeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Bold(true)
	msgStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

var help = map[mode]string{
	modeNormal:  "hjkl move · o/O insert below/above · a add · i rename · m move · c connect · ␣ done · ⏎ open · d delete · u/^r undo/redo · q quit",
	modeInput:   "⏎ save · esc cancel",
	modeMove:    "hjkl slide to next empty cell · ⏎ place · esc cancel",
	modeConnect: "hjkl pick target · ⏎ connect / disconnect · esc cancel",
}

func (m Model) View() string {
	done := 0
	for _, n := range m.b.Nodes {
		if n.Done {
			done++
		}
	}
	header := headerStyle.Render("wq") + dimStyle.Render(fmt.Sprintf("  %d nodes · %d done", len(m.b.Nodes), done))
	switch m.mode {
	case modeMove:
		header += "  " + modeStyle.Background(lipgloss.Color("81")).Render(" MOVE ")
	case modeConnect:
		header += "  " + modeStyle.Background(lipgloss.Color("220")).Render(" CONNECT ")
	}

	body := make([]string, m.bodyHeight())
	if len(m.b.Nodes) == 0 {
		body[0] = dimStyle.Render("  empty board — press a to add the first node")
	} else {
		l := newLayout(m.b, m.width)
		m.refreshRoutes() // no-op unless View runs before the first Update
		target, moving := "", ""
		sel := m.sel
		switch m.mode {
		case modeConnect:
			target, sel = m.sel, m.connFrom
		case modeMove:
			moving = m.sel
		}
		cv := renderBoard(m.b, l, m.routes, sel, target, moving)
		for i := range body {
			if y := m.scroll + i; y < cv.h {
				body[i] = cv.line(y)
			}
		}
	}

	var footer string
	switch {
	case m.mode == modeInput:
		footer = m.input.View()
	case m.msg != "":
		footer = msgStyle.Render(m.msg)
	default:
		footer = dimStyle.Render(runewidth.Truncate(help[m.mode], max(m.width-1, 1), "…"))
	}
	return header + "\n" + strings.Join(body, "\n") + "\n" + footer
}
