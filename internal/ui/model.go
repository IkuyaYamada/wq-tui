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
	"github.com/IkuyaYamada/wq-tui/internal/ime"
	"github.com/IkuyaYamada/wq-tui/internal/thread"
)

type mode int

const (
	modeNormal mode = iota
	modeInput
	modeMove
	modeConnect
	modeVisual
)

const maxUndo = 200

type Model struct {
	dir  string
	b    *board.Board
	undo []*board.Board
	redo []*board.Board

	// The cursor is a cell, not a node, so it can rest on empty cells.
	row, col int

	mode      mode
	input     textinput.Model
	inputNew  bool   // the node being titled was just created; Esc discards it
	inputDone bool   // the prompt asks for a completion comment, not a title
	prevCur   [2]int // cursor to restore when a new node is discarded
	moveIDs   []string
	moveOrig  *board.Board
	moveCur   [2]int

	// Visual mode selects the rectangle between visAnchor and the cursor,
	// or whole rows when visLine is set.
	visAnchor [2]int
	visLine   bool

	pendingD bool // first d of dd was pressed
	connFrom string

	width, height int
	scroll        int
	msg           string

	routes   []route
	routeKey string

	// ime switches to ASCII on the board and back to imePrev (the input
	// method in use before, e.g. Japanese) while a title is being typed.
	ime        ime.Switcher
	imePrev    string
	asciiAgain bool // schedule a second switch to ASCII after this update
}

// Option configures a Model.
type Option func(*Model)

// WithIME lets the model switch the keyboard input source.
func WithIME(s ime.Switcher) Option { return func(m *Model) { m.ime = s } }

func New(dir string, b *board.Board, opts ...Option) Model {
	ti := textinput.New()
	ti.Prompt = "title> "
	ti.CharLimit = 200
	m := Model{dir: dir, b: b, input: ti, width: 120, height: 40, ime: ime.Noop{}}
	for _, o := range opts {
		o(&m)
	}
	m.toASCII()
	m.asciiAgain = false
	return m
}

// toASCII puts the keyboard in ASCII for board keys, remembering what was
// active so title input can bring it back.
func (m *Model) toASCII() {
	if prev := m.ime.ASCII(); prev != "" {
		m.imePrev = prev
	}
	m.asciiAgain = true
}

// asciiAgainMsg repeats the switch to ASCII shortly after leaving a prompt
// or vim: when Enter both commits the IME conversion and closes the prompt,
// the input method can reassert itself after the first switch.
type asciiAgainMsg struct{}

const asciiAgainDelay = 150 * time.Millisecond

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

// selected is the node under the cursor, if any.
func (m *Model) selected() *board.Node { return m.b.At(m.row, m.col) }

func (m *Model) lastRow() int { return m.b.MaxRow() + bufferRows }

func (m *Model) moveCursor(d board.Dir) {
	switch d {
	case board.Left:
		m.col = max(m.col-1, 0)
	case board.Right:
		m.col = min(m.col+1, board.Cols-1)
	case board.Up:
		m.row = max(m.row-1, 0)
	case board.Down:
		m.row = min(m.row+1, m.lastRow())
	}
}

func (m *Model) cursorTo(n *board.Node) {
	if n != nil {
		m.row, m.col = n.Row, n.Col
	}
}

func (m *Model) clampCursor() {
	m.row = max(0, min(m.row, m.lastRow()))
	m.col = max(0, min(m.col, board.Cols-1))
}

// jump moves the cursor to the next (step 1) or previous (step -1) node in
// reading order, which skips across empty stretches of the board.
func (m *Model) jump(step int) {
	nodes := m.b.Clone()
	nodes.Sort()
	pos := func(n board.Node) int { return n.Row*board.Cols + n.Col }
	cur := m.row*board.Cols + m.col
	if step > 0 {
		for _, n := range nodes.Nodes {
			if pos(n) > cur {
				m.cursorTo(&n)
				return
			}
		}
	} else {
		for i := len(nodes.Nodes) - 1; i >= 0; i-- {
			if pos(nodes.Nodes[i]) < cur {
				m.cursorTo(&nodes.Nodes[i])
				return
			}
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case asciiAgainMsg:
		if m.mode != modeInput {
			if prev := m.ime.ASCII(); prev != "" {
				m.imePrev = prev
			}
		}
	case editorDoneMsg:
		if msg.err != nil {
			m.msg = "vim: " + msg.err.Error()
		}
		if msg.strategy != "" {
			m.pullTitle(msg.id, msg.strategy)
		}
		m.toASCII()
	case tea.KeyMsg:
		m.msg = ""
		if m.mode != modeInput {
			msg = halfwidth(msg)
		}
		switch m.mode {
		case modeNormal:
			cmd = m.keyNormal(msg)
		case modeInput:
			cmd = m.keyInput(msg)
		case modeMove:
			m.keyMove(msg)
		case modeConnect:
			m.keyConnect(msg)
		case modeVisual:
			m.keyVisual(msg)
		}
	default:
		if m.mode == modeInput {
			m.input, cmd = m.input.Update(msg)
		}
	}
	m.ensureVisible()
	m.refreshRoutes()
	if m.asciiAgain {
		m.asciiAgain = false
		again := tea.Tick(asciiAgainDelay, func(time.Time) tea.Msg { return asciiAgainMsg{} })
		cmd = tea.Batch(cmd, again)
	}
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

// pullTitle adopts a title edited in strategy.md's header as one undoable
// rename. An emptied title is ignored.
func (m *Model) pullTitle(id, strategy string) {
	title, ok := readStrategyTitle(strategy)
	n := m.b.Node(id)
	if !ok || n == nil || n.Title == title {
		return
	}
	m.checkpoint()
	m.b.Node(id).Title = title
	m.save()
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

func (m *Model) keyNormal(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	if m.pendingD {
		// dd deletes the row; any other key just cancels the pending d.
		m.pendingD = false
		if key == "d" {
			m.deleteRow()
		}
		return nil
	}
	if d, ok := dirOf(key); ok {
		m.moveCursor(d)
		return nil
	}
	n := m.selected()
	switch key {
	case "q", "ctrl+c":
		return tea.Quit
	case "w":
		m.jump(1)
	case "b":
		m.jump(-1)
	case "g":
		m.row, m.col = 0, 0
	case "G":
		m.row, m.col = max(m.b.MaxRow(), 0), 0
	case "o", "O", "a", "n":
		return m.startNew(key)
	case "i":
		if n == nil {
			return nil
		}
		m.inputNew = false
		return m.startInput("title> ", n.Title)
	case "m":
		if n == nil {
			return nil
		}
		m.startMove([]string{n.ID})
	case "c":
		if n == nil {
			return nil
		}
		m.connFrom = n.ID
		m.mode = modeConnect
	case " ":
		if n == nil {
			return nil
		}
		if n.Done {
			m.toggleDone("")
			return nil
		}
		// Completing asks for a comment first; Esc leaves the node open.
		m.inputDone = true
		return m.startInput("done> ", "")
	case "enter":
		if n == nil {
			return nil
		}
		return openNode(m.dir, *n)
	case "D":
		m.deleteRow()
	case "v", "V":
		m.mode = modeVisual
		m.visAnchor = [2]int{m.row, m.col}
		m.visLine = key == "V"
	case "d":
		m.pendingD = true
		m.msg = "d…"
	case "x":
		if n == nil {
			return nil
		}
		m.checkpoint()
		_ = m.b.Delete(n.ID)
		m.save()
	case "u":
		m.history(&m.undo, &m.redo, "undo")
	case "ctrl+r":
		m.history(&m.redo, &m.undo, "redo")
	}
	return nil
}

// deleteRow removes the cursor's row, nodes included, and pulls the rows
// below up. The buffer rows below the last node are already empty.
func (m *Model) deleteRow() {
	if m.row > m.b.MaxRow() {
		return
	}
	m.checkpoint()
	m.b.DeleteRow(m.row)
	m.save()
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
	m.clampCursor()
	m.save()
}

// startNew creates an untitled node right away (so it shows on the board)
// and asks for its title. Esc on the prompt discards it again. On an empty
// cell every add key drops the node right there.
func (m *Model) startNew(key string) tea.Cmd {
	now := time.Now()
	nn := board.Node{ID: board.NewID(now), CreatedAt: now}
	m.prevCur = [2]int{m.row, m.col}
	m.checkpoint()
	cur := m.selected()
	switch {
	case cur == nil:
		m.b.Add(nn, m.row, m.col)
	case key == "o" || key == "O":
		insert := m.b.InsertAfter
		if key == "O" {
			insert = m.b.InsertBefore
		}
		if err := insert(cur.ID, nn); err != nil {
			m.undo = m.undo[:len(m.undo)-1]
			m.msg = err.Error()
			return nil
		}
	default:
		m.b.Add(nn, cur.Row, cur.Col)
	}
	m.cursorTo(m.b.Node(nn.ID))
	m.inputNew = true
	return m.startInput("title> ", "")
}

func (m *Model) startInput(prompt, value string) tea.Cmd {
	m.ime.Select(m.imePrev)
	m.mode = modeInput
	m.input.Prompt = prompt
	m.input.SetValue(value)
	m.input.CursorEnd()
	return m.input.Focus()
}

func (m *Model) keyInput(k tea.KeyMsg) tea.Cmd {
	if m.inputDone {
		switch k.String() {
		case "enter", "esc", "ctrl+c":
			m.input.Blur()
			m.mode = modeNormal
			m.inputDone = false
			m.toASCII()
			if k.String() == "enter" {
				m.toggleDone(strings.TrimSpace(m.input.Value()))
			}
			return nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(k)
		return cmd
	}
	switch k.String() {
	case "enter":
		title := strings.TrimSpace(m.input.Value())
		m.input.Blur()
		m.mode = modeNormal
		m.toASCII()
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
		m.toASCII()
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
	m.row, m.col = m.prevCur[0], m.prevCur[1]
}

func (m *Model) startMove(ids []string) {
	m.moveOrig = m.b.Clone()
	m.moveIDs = ids
	m.moveCur = [2]int{m.row, m.col}
	m.mode = modeMove
}

// keyMove slides the moving nodes as one block; the cursor rides along.
func (m *Model) keyMove(k tea.KeyMsg) {
	key := k.String()
	if d, ok := dirOf(key); ok {
		ref := m.b.Node(m.moveIDs[0])
		r0, c0 := ref.Row, ref.Col
		if !m.b.MoveGroup(m.moveIDs, d) {
			m.msg = "can't move there"
			return
		}
		ref = m.b.Node(m.moveIDs[0])
		m.row += ref.Row - r0
		m.col += ref.Col - c0
		return
	}
	switch key {
	case "enter", "m":
		m.mode = modeNormal
		orig, cur := m.moveOrig.Node(m.moveIDs[0]), m.b.Node(m.moveIDs[0])
		if orig.Row != cur.Row || orig.Col != cur.Col {
			m.undo = append(m.undo, m.moveOrig)
			m.redo = nil
			m.save()
		}
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.b = m.moveOrig
		m.row, m.col = m.moveCur[0], m.moveCur[1]
	}
}

// visualIDs lists the nodes inside the visual selection.
func (m *Model) visualIDs() []string {
	r0, r1 := min(m.visAnchor[0], m.row), max(m.visAnchor[0], m.row)
	c0, c1 := min(m.visAnchor[1], m.col), max(m.visAnchor[1], m.col)
	if m.visLine {
		c0, c1 = 0, board.Cols-1
	}
	var ids []string
	for _, n := range m.b.Nodes {
		if n.Row >= r0 && n.Row <= r1 && n.Col >= c0 && n.Col <= c1 {
			ids = append(ids, n.ID)
		}
	}
	return ids
}

func (m *Model) keyVisual(k tea.KeyMsg) {
	key := k.String()
	if d, ok := dirOf(key); ok {
		m.moveCursor(d)
		return
	}
	switch key {
	case "v", "V":
		if m.visLine == (key == "V") {
			m.mode = modeNormal
		} else {
			m.visLine = key == "V"
		}
	case "m":
		ids := m.visualIDs()
		if len(ids) == 0 {
			m.msg = "no nodes selected"
			return
		}
		m.startMove(ids)
	case "=":
		ids := m.visualIDs()
		m.mode = modeNormal
		if len(ids) == 0 {
			return
		}
		m.checkpoint()
		if err := m.b.Organize(ids); err != nil {
			m.undo = m.undo[:len(m.undo)-1]
			m.msg = err.Error()
			return
		}
		m.msg = fmt.Sprintf("organized %d nodes", len(ids))
		m.save()
	case "d", "x":
		ids := m.visualIDs()
		m.mode = modeNormal
		if len(ids) == 0 {
			return
		}
		m.checkpoint()
		for _, id := range ids {
			_ = m.b.Delete(id)
		}
		m.save()
	case "esc", "ctrl+c":
		m.mode = modeNormal
	}
}

func (m *Model) keyConnect(k tea.KeyMsg) {
	key := k.String()
	if d, ok := dirOf(key); ok {
		m.moveCursor(d)
		return
	}
	switch key {
	case "enter", "c":
		target := m.selected()
		if target == nil {
			m.msg = "no node here — pick a node to connect to"
			return
		}
		m.mode = modeNormal
		if target.ID == m.connFrom {
			return
		}
		m.checkpoint()
		added, err := m.b.ToggleEdge(m.connFrom, target.ID)
		if err != nil {
			m.b = m.undo[len(m.undo)-1]
			m.undo = m.undo[:len(m.undo)-1]
			m.msg = err.Error()
			m.cursorTo(m.b.Node(m.connFrom))
			return
		}
		// A same-row connection pushes the target down; follow it.
		m.cursorTo(m.b.Node(target.ID))
		if added {
			m.msg = "connected"
		} else {
			m.msg = "disconnected"
		}
		m.save()
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.cursorTo(m.b.Node(m.connFrom))
	}
}

// toggleDone flips the done flag and logs it as a thread entry, with the
// comment after the event: "Completed: <comment>".
func (m *Model) toggleDone(comment string) {
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
	if comment != "" {
		event += ": " + comment
	}
	if _, err := thread.Add(board.NodeDir(m.dir, n.ID), time.Now(), event+"\n"); err != nil {
		m.msg = "thread: " + err.Error()
	}
}

func (m *Model) bodyHeight() int { return max(m.height-2, 1) }

func (m *Model) ensureVisible() {
	l := newLayout(m.b, m.width)
	top := l.rowY[min(m.row, len(l.rowY)-1)]
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
	modeNormal:  "hjkl cursor · w/b next/prev node · a add here · o/O insert below/above · i rename · m move · v/V select · c connect · ␣ done · ⏎ open · x delete · dd/D delete row · u/^r undo/redo · q quit",
	modeInput:   "⏎ ok · esc cancel",
	modeMove:    "hjkl slide to next empty cell · ⏎ place · esc cancel",
	modeConnect: "hjkl pick target · ⏎ connect / disconnect · esc cancel",
	modeVisual:  "hjkl extend · = organize · m move together · d delete · v block / V rows · esc cancel",
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
	case modeVisual:
		label := " VISUAL "
		if m.visLine {
			label = " VISUAL LINE "
		}
		header += "  " + modeStyle.Background(lipgloss.Color("141")).Render(label)
	}

	body := m.viewBoard()

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

func (m Model) viewBoard() []string {
	body := make([]string, m.bodyHeight())
	l := newLayout(m.b, m.width)
	m.refreshRoutes() // no-op unless View runs before the first Update
	v := view{cursorRow: m.row, cursorCol: m.col, cursorSt: stBorderSel}
	if n := m.selected(); n != nil {
		v.lit = n.ID
	}
	switch m.mode {
	case modeConnect:
		v.cursorSt, v.anchor, v.lit = stBorderTarget, m.connFrom, m.connFrom
	case modeMove:
		v.cursorSt = stBorderMove
		v.marked = setOf(m.moveIDs)
	case modeVisual:
		v.marked = setOf(m.visualIDs())
	}
	cv := renderBoard(m.b, l, m.routes, v)
	for i := range body {
		if y := m.scroll + i; y < cv.h {
			body[i] = cv.line(y)
		}
	}
	return body
}

func setOf(ids []string) map[string]bool {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// halfwidth maps full-width ASCII (ｈｊｋｌ, typed with a Japanese input
// method in full-width mode) to plain ASCII so board keys still work.
func halfwidth(k tea.KeyMsg) tea.KeyMsg {
	if k.Type != tea.KeyRunes {
		return k
	}
	runes := make([]rune, len(k.Runes))
	for i, r := range k.Runes {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			r -= 0xFEE0
		case r == 0x3000:
			r = ' '
		}
		runes[i] = r
	}
	if len(runes) == 1 && runes[0] == ' ' {
		return tea.KeyMsg{Type: tea.KeySpace, Runes: runes}
	}
	k.Runes = runes
	return k
}
