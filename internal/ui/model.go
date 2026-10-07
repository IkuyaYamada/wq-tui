// Package ui is the Bubble Tea front end: a six-column board driven by
// vim-style keys, with vim itself as the node editor.
package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aymanbagabas/go-osc52/v2"
	"github.com/charmbracelet/bubbles/textarea"
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
	modeMoveBreak
	modeEdit
)

const maxUndo = 200

type Model struct {
	dir  string
	b    *board.Board
	undo []*board.Board
	redo []*board.Board

	// The cursor is a cell, not a node, so it can rest on empty cells.
	row, col int

	mode       mode
	input      textinput.Model
	inputNew   bool // the node being titled was just created; Esc discards it
	inputDone  bool // the prompt asks for a completion comment, not a title
	inputBreak bool // the prompt asks for a session break label
	inputFrame bool // the prompt asks for a frame's title (new when frameIDs is set)
	frameIDs   []string
	prevCur    [2]int // cursor to restore when a new node is discarded
	moveIDs    []string
	moveOrig   *board.Board
	moveCur    [2]int

	// yank is the node yy copied (title, url and strategy body as they were
	// then). While it is held, a ghost card shows where p would paste a
	// copy; Esc lets it go.
	yank         *board.Node
	yankStrategy string

	// Visual mode selects the rectangle between visAnchor and the cursor,
	// or whole rows when visLine is set.
	visAnchor [2]int
	visLine   bool

	// frameSel is the frame the cursor has stepped onto (k from its top
	// row); frame keys then act on it rather than on a node.
	frameSel string

	pendingD       bool   // first d of dd was pressed
	pendingG       bool   // first g of gg / gx was pressed
	pendingY       bool   // first y of yy was pressed
	pendingBracket string // "[" or "]" waiting for Space: empty row above / below
	pendingZ       bool   // first z of zz / zt / zb was pressed

	openURL  func(string) error // opens a node's url; swapped out in tests
	copyText func(string) error // puts text on the clipboard; swapped out in tests
	connFrom string

	width, height int
	scroll        int
	msg           string

	routes   []route
	frames   []frame // shaped with the routes; relabel before drawing
	routeKey string
	stats    map[string]nodeStats // how much each node has written, for the meters

	// preview floats the cursor node's strategy and thread beside it (K).
	// previewText is read once per node, not on every frame.
	preview       bool
	previewID     string
	previewText   []pline
	previewScroll int

	// The preview turns into an editor with i (strategy) or a (new entry).
	edit       textarea.Model
	editKind   editKind
	editOrig   string
	editWarned bool // ^c was pressed once on unsaved changes
	editRows   int  // the preview's text height when editing began

	// stamp is board.json as this wq last read or wrote it; stale is set
	// once another wq has written it since.
	stamp board.Stamp
	stale bool

	// ime switches to ASCII on the board and back to imePrev (the input
	// method in use before, e.g. Japanese) while a title is being typed.
	ime        ime.Switcher
	imePrev    string
	asciiAgain bool // schedule a second switch to ASCII after this update

	caret *Caret // where the terminal cursor rests while typing; nil to leave it

	compact bool // one-line cards (wqc)
}

// Option configures a Model.
type Option func(*Model)

// WithCompact draws one-line cards, so far more rows fit on the screen.
func WithCompact() Option { return func(m *Model) { m.compact = true } }

// layout places the board for the terminal's width.
func (m *Model) layout() layout { return makeLayout(m.b, m.width, m.compact) }

// WithIME lets the model switch the keyboard input source.
func WithIME(s ime.Switcher) Option { return func(m *Model) { m.ime = s } }

func New(dir string, b *board.Board, opts ...Option) Model {
	ti := textinput.New()
	ti.Prompt = "title> "
	ti.CharLimit = 200
	m := Model{dir: dir, b: b, input: ti, width: 120, height: 40, ime: ime.Noop{}, openURL: openInBrowser, copyText: copyToClipboard}
	for _, o := range opts {
		o(&m)
	}
	m.toASCII()
	m.asciiAgain = false
	m.cursorTo(firstOpen(b))
	// Open on that node as the second row, with the row before it above
	// for context. ensureVisible clamps this once the size is known.
	m.scroll = m.layout().rowY[max(m.row-1, 0)]
	m.stamp, _ = board.CurrentStamp(dir)
	m.stats = readAllStats(dir, b)
	b.EnsureGroupIDs(time.Now())
	b.EnsureGroupColors()
	return m
}

// firstOpen is the incomplete node in the earliest phase (lowest row, then
// leftmost), or nil when everything is done.
func firstOpen(b *board.Board) *board.Node {
	var best *board.Node
	for i := range b.Nodes {
		n := &b.Nodes[i]
		if n.Done {
			continue
		}
		if best == nil || n.Row < best.Row || (n.Row == best.Row && n.Col < best.Col) {
			best = n
		}
	}
	return best
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

// save writes the board unless another wq has written board.json since
// this one last read or wrote it; then nothing is written and the board is
// flagged stale until R reloads it.
func (m *Model) save() {
	if m.checkStale() {
		m.msg = "not saved: " + staleMsg
		return
	}
	if err := board.Save(m.dir, m.b); err != nil {
		m.msg = "save failed: " + err.Error()
		return
	}
	m.stamp, _ = board.CurrentStamp(m.dir)
}

const staleMsg = "board.json was changed by another wq — R to reload"

// checkStale compares board.json on disk with what this wq last saw.
func (m *Model) checkStale() bool {
	if !m.stale {
		cur, err := board.CurrentStamp(m.dir)
		m.stale = err == nil && !cur.Same(m.stamp)
	}
	return m.stale
}

// reload adopts board.json from disk. Undo history is dropped: replaying
// it would overwrite what the other wq wrote.
func (m *Model) reload() {
	b, err := board.Load(m.dir)
	if err != nil {
		m.msg = "reload failed: " + err.Error()
		return
	}
	m.b = b
	m.stamp, _ = board.CurrentStamp(m.dir)
	m.stats = readAllStats(m.dir, b)
	b.EnsureGroupIDs(time.Now())
	b.EnsureGroupColors()
	m.frameSel = ""
	m.stale = false
	m.undo, m.redo = nil, nil
	m.clampCursor()
	m.msg = "reloaded"
}

// readOnlyKeys still work on a stale board: they only look around or
// leave. Everything else waits for R.
var readOnlyKeys = map[string]bool{
	"h": true, "j": true, "k": true, "l": true,
	"left": true, "down": true, "up": true, "right": true,
	"w": true, "b": true, "g": true, "G": true, "x": true, // x only after g (gx)
	"enter": true, "q": true, "ctrl+c": true, "R": true, "esc": true,
	"v": true, "V": true, // selecting is harmless; m, d and = on it are not
	"ctrl+d": true, "ctrl+u": true, "ctrl+e": true, "ctrl+y": true,
	"z": true, "t": true, // scrolling (zz / zt / zb)
	"y": true, // yy and yp only copy; p waits for R
	"K": true, // preview
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

// openNeighbour moves from node id to the next (step 1) or previous (-1)
// node, the one w / b would pick, and opens it in vim. At either end it
// stays on the board.
func (m *Model) openNeighbour(id string, step int) tea.Cmd {
	m.cursorTo(m.b.Node(id))
	m.jump(step)
	n := m.selected()
	if n == nil || n.ID == id {
		if step > 0 {
			m.msg = "no next node"
		} else {
			m.msg = "no previous node"
		}
		return nil
	}
	return openNode(m.dir, *n)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.FocusMsg:
		// Back from another app, which may have left the Japanese input
		// method on: board keys need ASCII again.
		if m.mode != modeInput && m.mode != modeEdit {
			m.toASCII()
		}
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
			m.pullHeader(msg.id, msg.strategy)
		}
		m.previewID = "" // vim may have changed what it shows
		if msg.id != "" {
			m.stats[msg.id] = readStats(m.dir, msg.id)
		}
		m.toASCII()
		if msg.step != 0 && msg.err == nil {
			cmd = m.openNeighbour(msg.id, msg.step)
		}
	case tea.KeyMsg:
		m.msg = ""
		if m.mode != modeInput && m.mode != modeEdit {
			if wide(msg) {
				// A full-width key means an input method is on; this one
				// still works, the next ones arrive as ASCII.
				m.toASCII()
			}
			msg = halfwidth(msg)
		}
		if m.mode == modeNormal || m.mode == modeVisual {
			key := msg.String()
			if key == "R" && m.mode == modeNormal {
				m.reload()
				break
			}
			if m.checkStale() && (!readOnlyKeys[key] && !(key == "p" && m.pendingY) || (key == "x" && !m.pendingG)) {
				m.pendingD, m.pendingY = false, false
				m.msg = staleMsg
				break
			}
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
			cmd = m.keyVisual(msg)
		case modeMoveBreak:
			cmd = m.keyMoveBreak(msg)
		case modeEdit:
			cmd = m.keyEdit(msg)
		}
	default:
		switch m.mode {
		case modeInput:
			m.input, cmd = m.input.Update(msg)
		case modeEdit:
			m.edit, cmd = m.edit.Update(msg)
		}
	}
	m.ensureVisible()
	m.refreshRoutes()
	m.refreshPreview()
	if m.mode == modeEdit {
		m.sizeEditor()
	}
	if m.asciiAgain {
		m.asciiAgain = false
		again := tea.Tick(asciiAgainDelay, func(time.Time) tea.Msg { return asciiAgainMsg{} })
		cmd = tea.Batch(cmd, again)
	}
	return m, cmd
}

// refreshRoutes reroutes edges and reshapes frames only when positions,
// edges, frames or width changed; they are the expensive part of drawing.
func (m *Model) refreshRoutes() {
	if key := fmt.Sprint(m.compact) + routeKey(m.b, m.width); key != m.routeKey {
		l := m.layout().withFrames(m.b)
		m.routes, m.frames = routeEdges(m.b, l), l.frames
		m.routeKey = key
	}
}

// openNodeURL opens the url of the node under the cursor in the browser.
func (m *Model) openNodeURL() {
	n := m.selected()
	switch {
	case n == nil:
	case n.URL == "":
		m.msg = "no url — add one to url: in the strategy header"
	default:
		if err := m.openURL(n.URL); err != nil {
			m.msg = "open: " + err.Error()
		} else {
			m.msg = "opened " + n.URL
		}
	}
}

// yankPath puts the absolute path of the strategy.md under the cursor (the
// frame's when one is selected) on the clipboard, writing the file first
// unless the board is stale.
func (m *Model) yankPath() {
	n := m.focus()
	if n == nil {
		return
	}
	path := filepath.Join(board.NodeDir(m.dir, n.ID), "strategy.md")
	if !m.stale {
		if _, err := strategyPath(m.dir, *n); err != nil {
			m.msg = "yp: " + err.Error()
			return
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if err := m.copyText(path); err != nil {
		m.msg = "yp: " + err.Error()
		return
	}
	m.msg = "copied " + path
}

// copyToClipboard hands text to the system clipboard tool, or failing
// that to the terminal as an OSC 52 sequence (which also reaches the local
// clipboard over ssh in terminals that allow it).
func copyToClipboard(text string) error {
	var tools [][]string
	switch {
	case runtime.GOOS == "darwin":
		tools = [][]string{{"pbcopy"}}
	case runtime.GOOS == "windows" || os.Getenv("WSL_DISTRO_NAME") != "":
		tools = [][]string{{"clip.exe"}}
	default:
		tools = [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}}
	}
	if os.Getenv("SSH_TTY") == "" {
		for _, t := range tools {
			if _, err := exec.LookPath(t[0]); err != nil {
				continue
			}
			cmd := exec.Command(t[0], t[1:]...)
			cmd.Stdin = strings.NewReader(text)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
	}
	_, err := fmt.Fprint(os.Stdout, osc52.New(text))
	return err
}

// openInBrowser hands url to the system opener; a bare host gets https://.
func openInBrowser(url string) error {
	if !strings.Contains(url, "://") {
		url = "https://" + url
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		// Not cmd /c start: cmd would read & in the url as a command separator.
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		// Under WSL, hand it to the Windows browser through interop.
		return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url).Start()
	}
	return exec.Command("xdg-open", url).Start()
}

// pullHeader adopts the title and url edited in strategy.md's header as
// one undoable change. A blank title is ignored; a blank url clears it.
func (m *Model) pullHeader(id, strategy string) {
	title, url, ok := readStrategyHeader(strategy)
	if !ok {
		return
	}
	if g := m.b.Group(id); g != nil {
		if title == "" {
			title = g.Title
		}
		if g.Title != title || g.URL != url {
			m.checkpoint()
			g = m.b.Group(id)
			g.Title, g.URL = title, url
			m.save()
		}
		return
	}
	n := m.b.Node(id)
	if n == nil {
		return
	}
	if title == "" {
		title = n.Title
	}
	if n.Title == title && n.URL == url {
		return
	}
	m.checkpoint()
	n = m.b.Node(id)
	n.Title, n.URL = title, url
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
	if m.pendingZ {
		m.pendingZ = false
		m.align(key)
		return nil
	}
	if m.pendingBracket != "" {
		side := m.pendingBracket
		m.pendingBracket = ""
		if key == " " {
			m.insertRow(side == "]")
		}
		return nil
	}
	if m.pendingG {
		m.pendingG = false
		switch key {
		case "g":
			// The earliest open node, where work resumes; the top-left
			// cell once everything is done.
			m.row, m.col = 0, 0
			m.cursorTo(firstOpen(m.b))
		case "x":
			m.openNodeURL()
		case "s":
			return m.decompose()
		}
		return nil
	}
	if m.pendingY {
		m.pendingY = false
		switch key {
		case "y":
			m.yankNode()
		case "p":
			m.yankPath()
		}
		return nil
	}
	if m.pendingD {
		// dd deletes the row; any other key just cancels the pending d.
		m.pendingD = false
		if key == "d" {
			m.deleteRow()
		}
		return nil
	}
	if d, ok := dirOf(key); ok {
		if !m.frameNav(d) {
			m.moveCursor(d)
		}
		return nil
	}
	n := m.selected()
	if m.preview {
		switch key {
		case "K", "esc":
			m.preview = false
			return nil
		case "i":
			return m.startEdit(editStrategy)
		case "a":
			return m.startEdit(editEntry)
		case "ctrl+d", "ctrl+u":
			if p, ok := m.previewBox(); ok {
				step := max((p.h-2)/2, 1)
				if key == "ctrl+u" {
					step = -step
				}
				m.previewScroll = max(0, min(m.previewScroll+step, p.maxScroll()))
			}
			return nil
		}
	}
	if g := m.selectedFrame(); g != nil {
		if cmd, done := m.keyFrame(key, g); done {
			return cmd
		}
	}
	switch key {
	case "K":
		m.preview = true
		m.previewID = "" // read it fresh
	case "q", "ctrl+c":
		return tea.Quit
	case "w":
		m.jump(1)
	case "b":
		m.jump(-1)
	case "g":
		m.pendingG = true
		m.msg = "g…"
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
	case "-":
		// Toggle the session break under the cursor's row; a new one asks
		// for an optional label.
		if _, ok := m.b.BreakAfter(m.row); ok {
			m.checkpoint()
			m.b.RemoveBreak(m.row)
			m.save()
			return nil
		}
		m.inputBreak = true
		return m.startInput("break> ", "")
	case "M":
		// Move the session break under the cursor's row to another gap.
		if _, ok := m.b.BreakAfter(m.row); !ok {
			m.msg = "no session break under this row"
			return nil
		}
		m.moveOrig = m.b.Clone()
		m.moveCur = [2]int{m.row, m.col}
		m.mode = modeMoveBreak
	case "z":
		m.pendingZ = true
		m.msg = "z…"
	case "ctrl+d":
		m.halfPage(1)
	case "ctrl+u":
		m.halfPage(-1)
	case "ctrl+e":
		m.scrollRows(1)
	case "ctrl+y":
		m.scrollRows(-1)
	case "[", "]":
		m.pendingBracket = key
		m.msg = key + "…"
	case "v", "V":
		m.mode = modeVisual
		m.visAnchor = [2]int{m.row, m.col}
		m.visLine = key == "V"
	case "d":
		m.pendingD = true
		m.msg = "d…"
	case "y":
		m.pendingY = true
		m.msg = "y…"
	case "p":
		m.paste()
	case "esc":
		m.yank = nil
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

// halfPage scrolls half a screen down (dir 1) or up (-1), moving the
// cursor by the same number of rows, like vim's C-d / C-u.
func (m *Model) halfPage(dir int) {
	rowH := m.layout().cardH + laneH
	rows := max(1, m.bodyHeight()/2/rowH)
	before := m.row
	m.row = max(0, min(m.row+dir*rows, m.lastRow()))
	m.scroll += (m.row - before) * rowH
}

// scrollRows scrolls the view by n rows (C-e / C-y) without moving the
// cursor, unless it would leave the screen; then it moves onto the nearest
// visible row, as in vim.
func (m *Model) scrollRows(n int) {
	l := m.layout()
	rowH := l.cardH + laneH
	h := m.bodyHeight()
	m.scroll = max(0, min(m.scroll+n*rowH, max(l.height-h, 0)))
	for m.row < len(l.rowY)-1 && l.rowY[m.row] < m.scroll {
		m.row++
	}
	for m.row > 0 && l.rowY[m.row]+l.cardH+1 > m.scroll+h {
		m.row--
	}
}

// align puts the cursor's row at the middle (zz), top (zt) or bottom (zb)
// of the screen.
func (m *Model) align(key string) {
	l := m.layout()
	top := l.rowY[min(m.row, len(l.rowY)-1)]
	h := m.bodyHeight()
	switch key {
	case "z":
		m.scroll = top + l.cardH/2 - h/2
	case "t":
		m.scroll = top
	case "b":
		m.scroll = top + l.cardH + 1 - h
	}
}

// insertRow opens an empty row above the cursor's row ([ Space) or below it
// (] Space); the cursor stays on its node.
func (m *Model) insertRow(below bool) {
	row := m.row
	if below {
		row++
	}
	_, brk := m.b.BreakAfter(m.row)
	if row > m.b.MaxRow() && !(below && brk) {
		return // the buffer rows below the last node are already empty
	}
	m.checkpoint()
	if below {
		m.b.OpenRowBelow(m.row) // a break under the cursor's row moves down too
	} else {
		m.b.InsertRow(row)
		m.row++
	}
	m.save()
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
	group := -1 // the group the new node joins: the cursor node's, or the frame it is in
	if cur != nil {
		group = m.b.GroupOf(cur.ID)
	} else {
		group = m.b.GroupAt(m.row, m.col)
	}
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
	if group >= 0 {
		m.b.Groups[group].Members = append(m.b.Groups[group].Members, nn.ID)
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
	if m.inputFrame {
		switch k.String() {
		case "enter", "esc", "ctrl+c":
			m.input.Blur()
			m.mode = modeNormal
			m.inputFrame = false
			m.toASCII()
			ids := m.frameIDs
			m.frameIDs = nil
			if k.String() != "enter" {
				return nil
			}
			title := strings.TrimSpace(m.input.Value())
			if ids != nil {
				now := time.Now()
				m.checkpoint()
				m.b.Frame(board.Group{ID: board.NewID(now), Title: title, CreatedAt: now}, ids)
				m.save()
			} else if g := m.selectedFrame(); g != nil && g.Title != title {
				m.checkpoint()
				m.selectedFrame().Title = title
				m.save()
			}
			return nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(k)
		return cmd
	}
	if m.inputBreak {
		switch k.String() {
		case "enter", "esc", "ctrl+c":
			m.input.Blur()
			m.mode = modeNormal
			m.inputBreak = false
			m.toASCII()
			label := strings.TrimSpace(m.input.Value())
			if br, ok := m.b.BreakAfter(m.row); k.String() == "enter" && (!ok || br.Label != label) {
				m.checkpoint()
				m.b.SetBreak(m.row, label)
				m.save()
			}
			return nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(k)
		return cmd
	}
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

// yankNode copies the node under the cursor for p.
func (m *Model) yankNode() {
	n := m.selected()
	if n == nil {
		return
	}
	y := board.Node{Title: n.Title, URL: n.URL}
	m.yank, m.yankStrategy = &y, strategyBody(m.dir, *n)
	m.msg = "yanked: " + n.Title
}

// pasteCell is where p would put the copy: the empty cell nearest the
// cursor, as for a.
func (m *Model) pasteCell() (row, col int) { return m.b.NearestEmpty(m.row, m.col) }

// paste puts a copy of the yanked node where the ghost shows and moves the
// cursor onto it, as vim's p does. It starts open, with its own empty
// thread, and joins the frame the cursor is in. The yank stays held, so p
// can paste again.
func (m *Model) paste() {
	if m.yank == nil {
		m.msg = "nothing yanked — yy on a node first"
		return
	}
	m.checkpoint()
	now := time.Now()
	nn := board.Node{ID: board.NewID(now), Title: m.yank.Title, URL: m.yank.URL, CreatedAt: now}
	group := m.b.GroupAt(m.row, m.col)
	if cur := m.selected(); cur != nil {
		group = m.b.GroupOf(cur.ID)
	}
	m.b.Add(nn, m.row, m.col)
	if group >= 0 {
		m.b.Groups[group].Members = append(m.b.Groups[group].Members, nn.ID)
	}
	m.cursorTo(m.b.Node(nn.ID))
	m.save()
	if m.yankStrategy != "" && !m.stale {
		if err := writeStrategyBody(m.dir, nn, m.yankStrategy); err != nil {
			m.msg = "strategy: " + err.Error()
		}
	}
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

// keyMoveBreak slides the break under the cursor's row from gap to gap;
// the cursor rides along on the row above it.
func (m *Model) keyMoveBreak(k tea.KeyMsg) tea.Cmd {
	switch key := k.String(); key {
	case "j", "down", "k", "up":
		step := 1
		if key == "k" || key == "up" {
			step = -1
		}
		to, ok := m.b.MoveBreak(m.row, step, m.lastRow())
		if !ok {
			m.msg = "can't move there"
			return nil
		}
		m.row = to
	case "enter", "M", "i":
		m.mode = modeNormal
		if m.row != m.moveCur[0] {
			m.undo = append(m.undo, m.moveOrig)
			m.redo = nil
			m.save()
		}
		if key == "i" {
			// Edit the label where the break now sits.
			br, _ := m.b.BreakAfter(m.row)
			m.inputBreak = true
			return m.startInput("break> ", br.Label)
		}
	case "x", "d":
		// Erase the break; one undo brings it back where it was before M.
		m.mode = modeNormal
		m.b.RemoveBreak(m.row)
		m.undo = append(m.undo, m.moveOrig)
		m.redo = nil
		m.save()
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.b = m.moveOrig
		m.row, m.col = m.moveCur[0], m.moveCur[1]
	}
	return nil
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

func (m *Model) keyVisual(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	if d, ok := dirOf(key); ok {
		m.moveCursor(d)
		return nil
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
			return nil
		}
		m.startMove(ids)
	case "=":
		ids := m.visualIDs()
		m.mode = modeNormal
		if len(ids) == 0 {
			return nil
		}
		m.checkpoint()
		if err := m.b.Organize(ids); err != nil {
			m.undo = m.undo[:len(m.undo)-1]
			m.msg = err.Error()
			return nil
		}
		m.msg = fmt.Sprintf("organized %d nodes", len(ids))
		m.save()
	case "g", "u":
		ids := m.visualIDs()
		m.mode = modeNormal
		if len(ids) == 0 {
			m.msg = "no nodes selected"
			return nil
		}
		if key == "g" {
			// A selection reaching into one frame grows that frame; one
			// touching none makes a new frame, named first (Esc gives up).
			switch frames := m.b.FramesOf(ids); len(frames) {
			case 0:
				m.frameIDs, m.inputFrame = ids, true
				return m.startInput("frame> ", "")
			case 1:
				m.checkpoint()
				added := m.b.AddToGroup(frames[0], ids)
				m.msg = fmt.Sprintf("added %d nodes to %s", added, m.b.Group(frames[0]).Title)
				m.save()
			default:
				m.msg = "the selection reaches into more than one frame"
			}
			return nil
		}
		m.checkpoint()
		m.b.Ungroup(ids)
		m.msg = fmt.Sprintf("took %d nodes out of their frames", len(ids))
		m.save()
	case "d", "x":
		ids := m.visualIDs()
		m.mode = modeNormal
		if len(ids) == 0 {
			return nil
		}
		m.checkpoint()
		for _, id := range ids {
			_ = m.b.Delete(id)
		}
		m.save()
	case "esc", "ctrl+c":
		m.mode = modeNormal
	}
	return nil
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
	n.DoneNote = ""
	if n.Done {
		n.DoneNote = comment
	}
	if n.Done {
		now := time.Now()
		n.DoneAt = &now
		event = "Completed"
	}
	m.save()
	if m.stale {
		return // not saved, so do not log it either
	}
	if comment != "" {
		event += ": " + comment
	}
	if _, err := thread.Add(board.NodeDir(m.dir, n.ID), time.Now(), event+"\n"); err != nil {
		m.msg = "thread: " + err.Error()
	}
}

func (m *Model) bodyHeight() int { return max(m.height-2, 1) }

func (m *Model) ensureVisible() {
	l := m.layout()
	top := l.rowY[min(m.row, len(l.rowY)-1)]
	bottom := top + l.cardH + 1
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
	modeNormal:    "hjkl cursor · ^d/^u half page · ^e/^y scroll · zz/zt/zb align · w/b next/prev node · gg first open · gx open url · R reload · a add here · o/O insert below/above · i rename · m move · yy/p copy / paste (esc drops) · yp copy strategy.md path · v/V select · c connect · ␣ done · ⏎ open · K preview · gs split into a frame · x delete · dd/D delete row · [␣/]␣ add row · - add / remove break below the row · M move / relabel / x delete break · u/^r undo/redo · q quit",
	modeInput:     "⏎ ok · esc cancel",
	modeMove:      "hjkl slide to next empty cell · ⏎ place · esc cancel",
	modeConnect:   "hjkl pick target · ⏎ connect / disconnect · esc cancel",
	modeEdit:      "esc save & close · ^c discard · ⏎ new line",
	modeMoveBreak: "jk move the session break · i edit label · x delete · ⏎ place · esc cancel",
	modeVisual:    "hjkl extend · = organize · m move together · g frame (or add to the frame it reaches into) · u unframe · d delete · v block / V rows · esc cancel",
}

func (m Model) View() string {
	done := 0
	for _, n := range m.b.Nodes {
		if n.Done {
			done++
		}
	}
	header := headerStyle.Render("wq") + dimStyle.Render(fmt.Sprintf("  %d nodes · %d done", len(m.b.Nodes), done))
	if m.stale {
		header += "  " + msgStyle.Render("⟳ changed in another wq — R to reload")
	}
	switch m.mode {
	case modeMove, modeMoveBreak:
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
	header += m.breadcrumb(m.width - lipgloss.Width(header) - 3)

	body := m.viewBoard()

	var footer string
	switch {
	case m.mode == modeInput:
		footer = m.input.View()
	case m.msg != "":
		footer = msgStyle.Render(m.msg)
	default:
		h := help[m.mode]
		if m.selectedFrame() != nil && m.mode == modeNormal && !m.preview {
			h = "FRAME  m move · x unframe · c color · ␣ done all · i rename · K preview · ⏎ open · j back"
		}
		if m.preview && m.mode == modeNormal {
			h = "PREVIEW  hjkl follow the cursor · ^d/^u scroll · i edit strategy · a new entry · ⏎ open in vim · K/esc close"
		}
		footer = dimStyle.Render(runewidth.Truncate(h, max(m.width-1, 1), "…"))
	}
	m.placeCaret(1 + len(body))
	return header + "\n" + strings.Join(body, "\n") + "\n" + footer
}

func (m Model) viewBoard() []string {
	body := make([]string, m.bodyHeight())
	l := m.layout()
	m.refreshRoutes() // no-op unless View runs before the first Update
	l.frames = relabel(m.b, m.frames)
	v := view{cursorRow: m.row, cursorCol: m.col, cursorSt: stBorderSel, stats: m.stats, frameSel: m.frameSel}
	if m.selectedFrame() != nil {
		v.cursorRow, v.cursorCol = -1, -1 // the frame is the cursor
	}
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
	case modeMoveBreak:
		v.movingBreak = true
	case modeNormal:
		if m.yank != nil && m.selectedFrame() == nil {
			r, c := m.pasteCell()
			v.ghost = &ghost{row: r, col: c, title: m.yank.Title}
		}
	}
	cv := renderBoard(m.b, l, m.routes, v)
	screen := newCanvas(cv.w, len(body))
	for i := range body {
		if y := m.scroll + i; y < cv.h {
			copy(screen.cells[i*cv.w:(i+1)*cv.w], cv.cells[y*cv.w:(y+1)*cv.w])
		}
	}
	if m.mode == modeEdit {
		return m.drawEditor(screen)
	}
	if p, ok := m.previewBox(); ok {
		p.draw(screen, m.focus().Title, m.previewScroll)
	}
	for i := range body {
		body[i] = screen.line(i)
	}
	return body
}

// refreshPreview rereads the preview when the cursor lands on another
// node, starting it from the top.
func (m *Model) refreshPreview() {
	n := m.focus()
	if !m.preview || n == nil {
		m.previewID = ""
		return
	}
	if n.ID != m.previewID {
		m.previewID = n.ID
		st := readStats(m.dir, n.ID)
		m.stats[n.ID] = st
		head := pline{fmt.Sprintf("strategy %d字 · thread %d件", st.chars, st.entries), stPreviewDim}
		m.previewText = append([]pline{head, {"", stPlain}}, previewLines(m.dir, *n)...)
		m.previewScroll = 0
	}
}

// previewBox places the preview beside the cursor's card, if it is shown.
func (m *Model) previewBox() (previewBox, bool) {
	if !m.preview || m.mode != modeNormal || m.previewID == "" || m.focus() == nil {
		return previewBox{}, false
	}
	l := m.layout()
	return placePreview(l.width, m.bodyHeight(), m.previewText, l.colX(m.col), l.cardW, l.rowY[m.row]-m.scroll)
}

func setOf(ids []string) map[string]bool {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// wide reports whether a key came through an input method: anything
// beyond ASCII, as board keys all are.
func wide(k tea.KeyMsg) bool {
	if k.Type != tea.KeyRunes {
		return false
	}
	for _, r := range k.Runes {
		if r > 0x7E {
			return true
		}
	}
	return false
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
