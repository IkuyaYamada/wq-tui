package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

func init() {
	// Box-drawing characters are "ambiguous" width; terminals draw them
	// narrow, so measure titles the same way even under a ja_JP locale.
	runewidth.DefaultCondition.EastAsianWidth = false
}

const (
	cardH = 2 + titleLines // borders plus room for a two-line title

	// titleLines is how many lines a card gives its title before cutting it
	// off with "…". Every card reserves them, so row heights never change.
	titleLines = 2
	gap        = 2
	margin     = 1

	// bufferRows empty rows always follow the lowest node, so there is
	// somewhere to move the cursor and drop a new node.
	bufferRows = 3

	// laneH is the fixed gap between rows that edges run through. It never
	// depends on the edges themselves, so moving or connecting a node does
	// not shift every row below it.
	laneH = 1
)

// layout maps grid cells to canvas coordinates. Rows are separated by
// fixed-height lanes for edges. Some gaps get lines of their own: a group
// frame's bottom right under its last row, a session break below the lane,
// and a group frame's top right over its first row.
type layout struct {
	cardW   int
	rowY    []int
	breakAt map[int]int  // row → the line its break is drawn on
	breakY  map[int]bool // lines holding a session break
	noH     map[int]bool // lines edges only cross: breaks and frame tops / bottoms
	frames  []frame
	width   int
	height  int
}

// frame is a group's rectangle on the canvas, drawn in the gaps around its
// members' cards.
type frame struct {
	x0, y0, x1, y1 int
	id, title      string
	done, total    int
	members        map[string]bool
}

func newLayout(b *board.Board, termW int) layout {
	l := layout{cardW: (termW - 2*margin - (board.Cols-1)*gap) / board.Cols}
	l.cardW = max(l.cardW, 8)
	l.width = 2*margin + board.Cols*l.cardW + (board.Cols-1)*gap
	rows := b.MaxRow() + 1 + bufferRows
	type span struct {
		r0, c0, r1, c1 int
		g              board.Group
	}
	var spans []span
	top, bottom := map[int]bool{}, map[int]bool{}
	for _, g := range b.Groups {
		if r0, c0, r1, c1, ok := b.Bounds(g); ok {
			spans = append(spans, span{r0, c0, r1, c1, g})
			top[r0], bottom[r1] = true, true
		}
	}
	l.breakAt, l.breakY, l.noH = map[int]int{}, map[int]bool{}, map[int]bool{}
	y := 0
	for r := 0; r < rows; r++ {
		if top[r] {
			l.noH[y] = true
			y++
		}
		l.rowY = append(l.rowY, y)
		y += cardH
		if bottom[r] {
			l.noH[y] = true
			y++
		}
		y += laneH
		if _, ok := b.BreakAfter(r); ok {
			l.breakAt[r] = y
			l.breakY[y], l.noH[y] = true, true
			y++
		}
	}
	l.height = y
	for _, s := range spans {
		f := frame{
			x0: l.colX(s.c0) - 1, y0: l.rowY[s.r0] - 1,
			x1: l.colX(s.c1) + l.cardW, y1: l.rowY[s.r1] + cardH,
			id: s.g.ID, title: s.g.Title,
			members: map[string]bool{},
		}
		for _, id := range s.g.Members {
			if n := b.Node(id); n != nil {
				f.members[id] = true
				f.total++
				if n.Done {
					f.done++
				}
			}
		}
		l.frames = append(l.frames, f)
	}
	return l
}

func (l layout) colX(c int) int { return margin + c*(l.cardW+gap) }

func index(b *board.Board) map[string]*board.Node {
	m := make(map[string]*board.Node, len(b.Nodes))
	for i := range b.Nodes {
		m[b.Nodes[i].ID] = &b.Nodes[i]
	}
	return m
}

type style uint8

const (
	stPlain style = iota
	stBorder
	stBorderSel
	stBorderDone
	stBorderTarget
	stBorderMove
	stTitle
	stTitleSel
	stTitleDone
	stTitleDoneSel
	stDoneNote
	stBreak
	stPreviewBorder
	stPreviewDim
	stPreviewWhen
	stMeter
	stChecks
	stChecksDone
	stFrame
	stFrameLit
	stEdge
	stEdgeDim
	stEdgeIn
	stEdgeOut
	stDot
)

var styles = map[style]lipgloss.Style{
	stBorder:        lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
	stBorderSel:     lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),
	stBorderDone:    lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
	stBorderTarget:  lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true),
	stBorderMove:    lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true),
	stTitle:         lipgloss.NewStyle(),
	stTitleSel:      lipgloss.NewStyle().Bold(true),
	stTitleDone:     lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Strikethrough(true),
	stTitleDoneSel:  lipgloss.NewStyle().Foreground(lipgloss.Color("248")).Strikethrough(true),
	stDoneNote:      lipgloss.NewStyle().Foreground(lipgloss.Color("108")).Italic(true),
	stBreak:         lipgloss.NewStyle().Foreground(lipgloss.Color("179")),
	stPreviewBorder: lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
	stPreviewDim:    lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
	stPreviewWhen:   lipgloss.NewStyle().Foreground(lipgloss.Color("108")).Bold(true),
	stMeter:         lipgloss.NewStyle().Foreground(lipgloss.Color("108")),
	stChecks:        lipgloss.NewStyle().Foreground(lipgloss.Color("108")),
	stChecksDone:    lipgloss.NewStyle().Foreground(lipgloss.Color("242")),
	stFrame:         lipgloss.NewStyle().Foreground(lipgloss.Color("97")),
	stFrameLit:      lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true),
	stEdge:          lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	stEdgeDim:       lipgloss.NewStyle().Foreground(lipgloss.Color("237")), // the rest, while a node's edges are lit
	stEdgeIn:        lipgloss.NewStyle().Foreground(lipgloss.Color("215")).Bold(true),
	stEdgeOut:       lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true),
	stDot:           lipgloss.NewStyle().Foreground(lipgloss.Color("237")),
}

type cell struct {
	r    rune
	st   style
	cont bool // right half of a wide rune
}

type canvas struct {
	w, h  int
	cells []cell
}

func newCanvas(w, h int) *canvas {
	c := &canvas{w: w, h: h, cells: make([]cell, w*h)}
	for i := range c.cells {
		c.cells[i].r = ' '
	}
	return c
}

// set writes one narrow rune. A wide rune it cuts in half loses its other
// half too, or the rest of the line would shift by a cell.
func (c *canvas) set(x, y int, r rune, st style) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	i := y*c.w + x
	if c.cells[i].cont && x > 0 {
		c.cells[i-1] = cell{r: ' ', st: c.cells[i-1].st}
	}
	if x+1 < c.w && c.cells[i+1].cont {
		c.cells[i+1] = cell{r: ' ', st: c.cells[i+1].st}
	}
	c.cells[i] = cell{r: r, st: st}
}

func (c *canvas) text(x, y int, s string, st style) {
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		c.set(x, y, r, st)
		if w == 2 && x+1 < c.w && y >= 0 && y < c.h && x >= 0 {
			c.set(x+1, y, ' ', st)
			c.cells[y*c.w+x+1].cont = true
		}
		x += w
	}
}

func (c *canvas) line(y int) string {
	return strings.TrimRight(c.span(y, 0, c.w), " ")
}

// spliced is line y with cells x0..x1-1 replaced by mid, an already
// styled string exactly x1-x0 cells wide.
func (c *canvas) spliced(y, x0, x1 int, mid string) string {
	return c.span(y, 0, x0) + mid + strings.TrimRight(c.span(y, x1, c.w), " ")
}

// span renders cells x0..x1-1 of line y with their styles.
func (c *canvas) span(y, x0, x1 int) string {
	var sb, run strings.Builder
	cur := stPlain
	flush := func() {
		if run.Len() == 0 {
			return
		}
		if cur == stPlain {
			sb.WriteString(run.String())
		} else {
			sb.WriteString(styles[cur].Render(run.String()))
		}
		run.Reset()
	}
	for x := x0; x < x1; x++ {
		cl := c.cells[y*c.w+x]
		if cl.cont {
			continue
		}
		if cl.st != cur {
			flush()
			cur = cl.st
		}
		run.WriteRune(cl.r)
	}
	flush()
	return sb.String()
}

var (
	frameNormal = [6]rune{'╭', '─', '╮', '│', '╰', '╯'}
	frameBold   = [6]rune{'┏', '━', '┓', '┃', '┗', '┛'}
	frameDashed = [6]rune{'┌', '╌', '┐', '╎', '└', '┘'}
)

func drawFrame(cv *canvas, l layout, row, col int, f [6]rune, border style) {
	x, y := l.colX(col), l.rowY[row]
	w, bottom := l.cardW, y+cardH-1
	cv.set(x, y, f[0], border)
	cv.set(x+w-1, y, f[2], border)
	cv.set(x, bottom, f[4], border)
	cv.set(x+w-1, bottom, f[5], border)
	for i := 1; i < w-1; i++ {
		cv.set(x+i, y, f[1], border)
		cv.set(x+i, bottom, f[1], border)
	}
	for yy := y + 1; yy < bottom; yy++ {
		cv.set(x, yy, f[3], border)
		cv.set(x+w-1, yy, f[3], border)
		for i := 1; i < w-1; i++ {
			cv.set(x+i, yy, ' ', stPlain)
		}
	}
}

// titleRows wraps a title into at most titleLines lines of width w, ending
// the last one with "…" when the title does not fit.
func titleRows(title string, w int) []string {
	var rows []string
	rest := title
	for len(rows) < titleLines-1 && runewidth.StringWidth(rest) > w {
		head := runewidth.Truncate(rest, w, "")
		rows = append(rows, head)
		rest = rest[len(head):]
	}
	return append(rows, runewidth.Truncate(rest, w, "…"))
}

func drawCard(cv *canvas, l layout, n board.Node, border, title style, bold bool, st nodeStats) {
	f := frameNormal
	if bold {
		f = frameBold
	}
	drawFrame(cv, l, n.Row, n.Col, f, border)
	x, y, w := l.colX(n.Col), l.rowY[n.Row], l.cardW
	// How much is written: strategy, then thread, as bars on the bottom
	// border's left end (edge ports sit further in).
	for i, r := range st.meters() {
		if r != 0 {
			cv.set(x+1+i, y+cardH-1, r, stMeter)
		}
	}
	end := x + w - 1 // the task list's progress ends before the corner
	if n.URL != "" {
		cv.set(x+w-3, y, '↗', border) // has a link: gx opens it
		end = x + w - 3
	} else {
		end--
	}
	// The strategy's task list, as "done/total" at the top border's right
	// end, dimmed once every box is ticked. Incoming edges drawn later win
	// where they land on it.
	if p := st.progress(); p != "" && runewidth.StringWidth(p) <= w-4 {
		ps := stChecks
		if st.checked == st.checks {
			ps = stChecksDone
		}
		cv.text(end-runewidth.StringWidth(p), y, p, ps)
	}
	label := n.Title
	if n.Done {
		label = "✓ " + label
	}
	if label == "" {
		label = "…"
	}
	// A completion comment takes the second line, so the card (and the
	// grid) keeps its size; the title gives up its second line for it.
	if n.Done && n.DoneNote != "" {
		cv.text(x+2, y+1, runewidth.Truncate(label, w-4, "…"), title)
		cv.text(x+2, y+2, runewidth.Truncate(n.DoneNote, w-4, "…"), stDoneNote)
		return
	}
	for i, line := range titleRows(label, w-4) {
		cv.text(x+2, y+1+i, line, title)
	}
}

// drawRect draws a group frame. Edges are drawn later and win where they
// cross it.
func drawRect(cv *canvas, f frame, st style, heavy bool) {
	r := frameNormal
	if heavy {
		r = frameBold
	}
	for x := f.x0 + 1; x < f.x1; x++ {
		cv.set(x, f.y0, r[1], st)
		cv.set(x, f.y1, r[1], st)
	}
	for y := f.y0 + 1; y < f.y1; y++ {
		cv.set(f.x0, y, r[3], st)
		cv.set(f.x1, y, r[3], st)
	}
	cv.set(f.x0, f.y0, r[0], st)
	cv.set(f.x1, f.y0, r[2], st)
	cv.set(f.x0, f.y1, r[4], st)
	cv.set(f.x1, f.y1, r[5], st)
}

// drawFrameLabels writes a frame's progress at the right end of its top
// line and its title at the left, after the edges, into stretches no edge
// crosses (bits marks edge cells). A title that does not fit before the
// first crossing takes the first stretch it fits in, on the top line or
// else the bottom one, or failing that the widest, cut short; the header's
// breadcrumb still names it in full.
func drawFrameLabels(cv *canvas, f frame, st style, bits []uint8) {
	if f.y0 < 0 || f.y1 >= cv.h {
		return
	}
	free := func(x, y int) bool { return x >= 0 && x < cv.w && bits[y*cv.w+x] == 0 }
	end := f.x1 - 1 // labels stay clear of the corners
	if p := f.progress(); p != "" {
		pw := runewidth.StringWidth(p)
		ok := true
		for x := end - pw; x < end; x++ {
			ok = ok && free(x, f.y0)
		}
		if ok {
			cv.text(end-pw, f.y0, p, st)
			end -= pw
		}
	}
	if f.title == "" {
		return
	}
	// Each label keeps a cell of line on either side, so it reads as part
	// of the frame.
	full := runewidth.StringWidth(f.title) + 4
	type stretch struct{ x0, x1, y int }
	var best stretch
	for _, y := range []int{f.y0, f.y1} {
		lineEnd := end
		if y == f.y1 {
			lineEnd = f.x1 - 1
		}
		for x := f.x0 + 1; x < lineEnd; x++ {
			if !free(x, y) {
				continue
			}
			s := stretch{x, x, y}
			for s.x1 < lineEnd && free(s.x1, y) {
				s.x1++
			}
			if s.x1-s.x0 >= full {
				best = s
				goto draw
			}
			if s.x1-s.x0 > best.x1-best.x0 {
				best = s
			}
			x = s.x1
		}
	}
draw:
	if room := best.x1 - best.x0 - 4; room >= 1 {
		cv.text(best.x0+1, best.y, " "+runewidth.Truncate(f.title, room, "…")+" ", st)
	}
}

// progress is the done count on a frame's top line, "" when there is no
// room for it.
func (f frame) progress() string {
	p := fmt.Sprintf(" %d/%d ", f.done, f.total)
	if f.x1-f.x0 <= runewidth.StringWidth(p)+3 {
		return ""
	}
	return p
}

// drawBreaks draws each session break as a dotted line on its own line under
// its row's lane. Edges only cross it straight down; they are drawn later
// and win where they do.
func drawBreaks(cv *canvas, l layout, breaks []board.Break, v view) {
	for _, br := range breaks {
		if br.After < 0 || br.After >= len(l.rowY) {
			continue
		}
		st := stBreak
		if v.movingBreak && br.After == v.cursorRow {
			st = stBorderMove
		}
		y, ok := l.breakAt[br.After]
		if !ok {
			continue
		}
		for x := 0; x < cv.w; x++ {
			cv.set(x, y, '┄', st)
		}
	}
}

// drawBreakLabels writes each break's label into the leftmost stretch of its
// gap that no edge crosses (bits marks edge cells), so lines never cut it.
func drawBreakLabels(cv *canvas, l layout, breaks []board.Break, bits []uint8) {
	for _, br := range breaks {
		if br.Label == "" || br.After < 0 || br.After >= len(l.rowY) {
			continue
		}
		y, ok := l.breakAt[br.After]
		if !ok {
			continue
		}
		label := " " + runewidth.Truncate(br.Label, max(cv.w-8, 1), "…") + " "
		w := runewidth.StringWidth(label)
		for x := 4; x+w <= cv.w; x++ {
			free := true
			for i := x; i < x+w; i++ {
				if bits[y*cv.w+i] != 0 {
					free = false
					x = i // resume the search past this edge
					break
				}
			}
			if free {
				cv.text(x, y, label, stBreak)
				break
			}
		}
	}
}

// edgeRune turns a set of connection bits into a box-drawing character.
func edgeRune(bits uint8) rune {
	switch bits {
	case bitU | bitD, bitU, bitD:
		return '│'
	case bitL | bitR, bitL, bitR:
		return '─'
	case bitD | bitR:
		return '┌'
	case bitD | bitL:
		return '┐'
	case bitU | bitR:
		return '└'
	case bitU | bitL:
		return '┘'
	case bitU | bitD | bitR:
		return '├'
	case bitU | bitD | bitL:
		return '┤'
	case bitD | bitL | bitR:
		return '┬'
	case bitU | bitL | bitR:
		return '┴'
	default:
		return '┼'
	}
}

// edgeRuneHeavy is edgeRune in heavy lines, for the lit node's edges.
func edgeRuneHeavy(bits uint8) rune {
	light := []rune("│─┌┐└┘├┤┬┴┼")
	heavy := []rune("┃━┏┓┗┛┣┫┳┻╋")
	r := edgeRune(bits)
	for i, l := range light {
		if l == r {
			return heavy[i]
		}
	}
	return r
}

const (
	bitU uint8 = 1 << iota
	bitD
	bitL
	bitR
)

// view describes what to highlight: the cursor cell (drawn in cursorSt, as
// a dashed placeholder when empty), an anchor node such as the source of a
// connection, and the node whose edges light up.
type view struct {
	cursorRow, cursorCol int
	cursorSt             style
	anchor               string
	lit                  string
	marked               map[string]bool // multi-selection (visual / group move)
	movingBreak          bool            // the break under cursorRow is being moved
	frameSel             string          // the selected frame; no card is then the cursor
	stats                map[string]nodeStats
	ghost                *ghost // where p would paste the yanked node
}

// ghost is a not-yet-pasted copy, drawn dashed and dim in an empty cell.
type ghost struct {
	row, col int
	title    string
}

// renderBoard draws every card, the cursor and the routed edges.
func renderBoard(b *board.Board, l layout, routes []route, v view) *canvas {
	cv := newCanvas(l.width, l.height)
	drawBreaks(cv, l, b.Breaks, v)
	// A faint dot marks every cell so empty rows still read as a grid.
	for r := range l.rowY {
		for c := 0; c < board.Cols; c++ {
			cv.set(l.colX(c)+l.cardW/2, l.rowY[r]+1, '·', stDot)
		}
	}
	cur := b.At(v.cursorRow, v.cursorCol)
	frameSt := make([]style, len(l.frames))
	for i, f := range l.frames {
		st, heavy := stFrame, f.id != "" && f.id == v.frameSel
		if heavy || (cur != nil && f.members[cur.ID]) {
			st = stFrameLit
		}
		frameSt[i] = st
		drawRect(cv, f, st, heavy)
	}
	onCursor := false
	for _, n := range b.Nodes {
		border, title, bold := stBorder, stTitle, false
		switch {
		case n.Row == v.cursorRow && n.Col == v.cursorCol:
			border, title, bold = v.cursorSt, stTitleSel, true
			onCursor = true
		case v.marked[n.ID]:
			border, title, bold = stBorderMove, stTitleSel, true
		case n.ID == v.anchor:
			border, title, bold = stBorderSel, stTitleSel, true
		case n.Done:
			border, title = stBorderDone, stTitleDone
		}
		// A done node keeps its struck-through title even under the cursor.
		if n.Done {
			title = stTitleDone
			if bold {
				title = stTitleDoneSel
			}
		}
		drawCard(cv, l, n, border, title, bold, v.stats[n.ID])
	}
	if !onCursor && v.cursorRow >= 0 && v.cursorRow < len(l.rowY) {
		drawFrame(cv, l, v.cursorRow, v.cursorCol, frameDashed, v.cursorSt)
	}
	if g := v.ghost; g != nil && g.row >= 0 && g.row < len(l.rowY) {
		border := stBorderMove
		if g.row == v.cursorRow && g.col == v.cursorCol {
			border = v.cursorSt
		}
		drawFrame(cv, l, g.row, g.col, frameDashed, border)
		x, y := l.colX(g.col), l.rowY[g.row]
		for i, line := range titleRows(g.title, l.cardW-4) {
			cv.text(x+2, y+1+i, line, stPreviewDim)
		}
	}

	// Every edge adds its connections to bits. The lit node's edges also
	// keep their own in litBits, so where they share cells with others
	// they are drawn heavy, in their own shape and color, and can be
	// followed through: incoming (from predecessors) and outgoing apart.
	bits := make([]uint8, cv.w*cv.h)
	litBits := make([]uint8, cv.w*cv.h)
	litSt := make([]style, cv.w*cv.h)
	base := stEdge
	if v.lit != "" {
		base = stEdgeDim
	}
	for _, rt := range routes {
		st := base
		switch {
		case v.lit == "":
		case rt.from == v.lit:
			st = stEdgeOut
		case rt.to == v.lit:
			st = stEdgeIn
		}
		lit := st != base
		add := func(k int, bit uint8) {
			bits[k] |= bit
			if lit {
				litBits[k] |= bit
				if litSt[k] != stEdgeOut { // outgoing wins where the two cross
					litSt[k] = st
				}
			}
		}
		for i, p := range rt.path {
			k := p.y*cv.w + p.x
			if i == 0 {
				add(k, bitU)
			}
			if i == len(rt.path)-1 {
				add(k, bitD)
			}
			if i > 0 {
				q := rt.path[i-1]
				kq := q.y*cv.w + q.x
				switch {
				case q.y < p.y:
					add(kq, bitD)
					add(k, bitU)
				case q.x < p.x:
					add(kq, bitR)
					add(k, bitL)
				default:
					add(kq, bitL)
					add(k, bitR)
				}
			}
		}
		port := '┬'
		if lit {
			port = '┰'
		}
		cv.set(rt.srcPort.x, rt.srcPort.y, port, st)
		cv.set(rt.dstPort.x, rt.dstPort.y, '▼', st)
	}
	drawBreakLabels(cv, l, b.Breaks, bits)
	for i, f := range l.frames {
		drawFrameLabels(cv, f, frameSt[i], bits)
	}
	for k, v := range bits {
		switch {
		case litBits[k] != 0:
			cv.set(k%cv.w, k/cv.w, edgeRuneHeavy(litBits[k]), litSt[k])
		case v != 0:
			cv.set(k%cv.w, k/cv.w, edgeRune(v), base)
		}
	}
	return cv
}
