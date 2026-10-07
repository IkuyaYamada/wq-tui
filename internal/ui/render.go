package ui

import (
	"fmt"
	"sort"
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
	// fullCardH is a card's height: borders plus room for a two-line
	// title. Compact cards are a single line (compactCardH).
	fullCardH    = 2 + titleLines
	compactCardH = 1

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
// fixed-height lanes for edges. Some gaps get lines of their own: a frame's
// outline right under a row where it closes below a member, a session break
// below the lane, and a frame's outline right over a row where it closes
// above a member.
type layout struct {
	compact bool // one-line cards, no borders: see drawCompactCard
	cardH   int
	cardW   int
	rowY    []int
	breakAt map[int]int  // row → the line its break is drawn on
	breakY  map[int]bool // lines holding a session break
	noH     map[int]bool // lines edges only cross: breaks and frame tops / bottoms
	frames  []frame      // set by withFrames
	width   int
	height  int
}

func newLayout(b *board.Board, termW int) layout { return makeLayout(b, termW, false) }

func makeLayout(b *board.Board, termW int, compact bool) layout {
	l := layout{compact: compact, cardH: fullCardH, cardW: (termW - 2*margin - (board.Cols-1)*gap) / board.Cols}
	if compact {
		l.cardH = compactCardH
	}
	l.cardW = max(l.cardW, 8)
	l.width = 2*margin + board.Cols*l.cardW + (board.Cols-1)*gap
	rows := b.MaxRow() + 1 + bufferRows
	// A member whose neighbour above (below) is not in its frame has the
	// frame's line over (under) it.
	top, bottom := map[int]bool{}, map[int]bool{}
	for _, g := range b.Groups {
		cells := map[[2]int]bool{}
		for _, id := range g.Members {
			if n := b.Node(id); n != nil {
				cells[[2]int{n.Row, n.Col}] = true
			}
		}
		for c := range cells {
			if !cells[[2]int{c[0] - 1, c[1]}] {
				top[c[0]] = true
			}
			if !cells[[2]int{c[0] + 1, c[1]}] {
				bottom[c[0]] = true
			}
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
		y += l.cardH
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
	return l
}

// withFrames shapes the frames too, which costs far more than the rest of
// the layout; the model keeps them with the routes instead.
func (l layout) withFrames(b *board.Board) layout {
	l.frames = layoutFrames(b, l)
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
	stEdge
	stEdgeDim
	stEdgeIn
	stEdgeOut
	stDot
	stCard // compact cards: one-line bands
	stCardSel
	stCardTarget
	stCardMove
	stCardDone
	stCardDoneSel
	stCardDoneBand
	stCardChecks
	stCardChecksDone
	stFrame // the first of the frame styles: see frameStyle
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
	stEdge:          lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	stEdgeDim:       lipgloss.NewStyle().Foreground(lipgloss.Color("237")), // the rest, while a node's edges are lit
	stEdgeIn:        lipgloss.NewStyle().Foreground(lipgloss.Color("215")).Bold(true),
	stEdgeOut:       lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true),
	stDot:           lipgloss.NewStyle().Foreground(lipgloss.Color("237")),

	stCard:           lipgloss.NewStyle().Background(lipgloss.Color("236")),
	stCardSel:        lipgloss.NewStyle().Background(lipgloss.Color("25")).Foreground(lipgloss.Color("231")).Bold(true),
	stCardTarget:     lipgloss.NewStyle().Background(lipgloss.Color("136")).Foreground(lipgloss.Color("16")).Bold(true),
	stCardMove:       lipgloss.NewStyle().Background(lipgloss.Color("30")).Foreground(lipgloss.Color("231")).Bold(true),
	stCardDone:       lipgloss.NewStyle().Background(lipgloss.Color("234")).Foreground(lipgloss.Color("242")).Strikethrough(true),
	stCardDoneSel:    lipgloss.NewStyle().Background(lipgloss.Color("25")).Foreground(lipgloss.Color("250")).Strikethrough(true),
	stCardDoneBand:   lipgloss.NewStyle().Background(lipgloss.Color("234")),
	stCardChecks:     lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("108")),
	stCardChecksDone: lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("242")),
}

// framePalette holds the colors frames take turns in: dim, and lit while
// the cursor is on the frame or one of its members.
var framePalette = [board.FrameColors][2]lipgloss.Color{
	{"97", "141"},  // purple
	{"30", "43"},   // teal
	{"132", "211"}, // rose
	{"100", "149"}, // olive
}

func init() {
	for i, c := range framePalette {
		styles[frameStyle(i, false)] = lipgloss.NewStyle().Foreground(c[0])
		styles[frameStyle(i, true)] = lipgloss.NewStyle().Foreground(c[1]).Bold(true)
	}
}

func frameStyle(color int, lit bool) style {
	st := stFrame + style(2*color)
	if lit {
		st++
	}
	return st
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
	w, bottom := l.cardW, y+l.cardH-1
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
			cv.set(x+1+i, y+l.cardH-1, r, stMeter)
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

// drawFrameLines draws a frame's outline and necks, heavy when the frame
// is selected. Edges are drawn later and win where they cross it.
func drawFrameLines(cv *canvas, f frame, st style, heavy bool) {
	for p, bits := range f.lines {
		r := frameRune(bits)
		if heavy {
			r = edgeRuneHeavy(bits)
		}
		cv.set(p.x, p.y, r, st)
	}
}

// frameRune is edgeRune with the rounded corners of cards.
func frameRune(bits uint8) rune {
	switch bits {
	case bitD | bitR:
		return '╭'
	case bitD | bitL:
		return '╮'
	case bitU | bitR:
		return '╰'
	case bitU | bitL:
		return '╯'
	}
	return edgeRune(bits)
}

// drawFrameLabels writes a frame's progress and title, after the edges,
// on the straight stretches of its outline that no edge crosses (bits
// marks edge cells) and no neck leaves from. The progress takes the right
// end of the topmost stretch, the title the first stretch it fits in, top
// to bottom and left to right, or failing that the widest, cut short; the
// header's breadcrumb still names it in full.
func drawFrameLabels(cv *canvas, f frame, st style, bits []uint8) {
	type stretch struct{ x0, x1, y int } // cells x0..x1-1
	var runs []stretch
	for p, b := range f.lines {
		if b == bitL|bitR && p.y >= 0 && p.y < cv.h {
			runs = append(runs, stretch{p.x, p.x + 1, p.y})
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].y != runs[j].y {
			return runs[i].y < runs[j].y
		}
		return runs[i].x0 < runs[j].x0
	})
	merged := runs[:0]
	for _, r := range runs {
		if k := len(merged) - 1; k >= 0 && merged[k].y == r.y && merged[k].x1 == r.x0 {
			merged[k].x1 = r.x1
			continue
		}
		merged = append(merged, r)
	}
	runs = merged
	if len(runs) == 0 {
		return
	}
	free := func(x, y int) bool { return x >= 0 && x < cv.w && bits[y*cv.w+x] == 0 }
	// Labels keep a cell of line between them and a corner or a crossing.
	if p := fmt.Sprintf(" %d/%d ", f.done, f.total); f.total > 0 {
		pw := runewidth.StringWidth(p)
		r := &runs[0]
		end := r.x1 - 1
		ok := end-pw > r.x0
		for x := end - pw; ok && x < end; x++ {
			ok = free(x, r.y)
		}
		if ok {
			cv.text(end-pw, r.y, p, st)
			r.x1 = end - pw
		}
	}
	if f.title == "" {
		return
	}
	full := runewidth.StringWidth(f.title) + 4
	var best stretch
	for _, r := range runs {
		for x := r.x0; x < r.x1; x++ {
			if !free(x, r.y) {
				continue
			}
			s := stretch{x, x, r.y}
			for s.x1 < r.x1 && free(s.x1, r.y) {
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
			cv.set(l.colX(c)+l.cardW/2, l.rowY[r]+(l.cardH-1)/2, '·', stDot)
		}
	}
	cur := b.At(v.cursorRow, v.cursorCol)
	frameSt := make([]style, len(l.frames))
	for i, f := range l.frames {
		heavy := f.id != "" && f.id == v.frameSel
		frameSt[i] = frameStyle(f.color, heavy || (cur != nil && f.members[cur.ID]))
		drawFrameLines(cv, f, frameSt[i], heavy)
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
		if l.compact {
			drawCompactCard(cv, l, n, compactStyle(border, n.Done), v.stats[n.ID])
			continue
		}
		drawCard(cv, l, n, border, title, bold, v.stats[n.ID])
	}
	if !onCursor && v.cursorRow >= 0 && v.cursorRow < len(l.rowY) {
		if l.compact {
			drawCompactEmpty(cv, l, v.cursorRow, v.cursorCol, "", v.cursorSt)
		} else {
			drawFrame(cv, l, v.cursorRow, v.cursorCol, frameDashed, v.cursorSt)
		}
	}
	if g := v.ghost; g != nil && g.row >= 0 && g.row < len(l.rowY) {
		border := stBorderMove
		if g.row == v.cursorRow && g.col == v.cursorCol {
			border = v.cursorSt
		}
		if l.compact {
			drawCompactEmpty(cv, l, g.row, g.col, g.title, border)
		} else {
			drawFrame(cv, l, g.row, g.col, frameDashed, border)
			x, y := l.colX(g.col), l.rowY[g.row]
			for i, line := range titleRows(g.title, l.cardW-4) {
				cv.text(x+2, y+1+i, line, stPreviewDim)
			}
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
	// Compact cards have no borders to carry ports: an edge just starts
	// under its card, and its arrowhead goes on its last cell, over the
	// target, once the lines are drawn.
	type arrow struct {
		p  point
		st style
	}
	var arrows []arrow
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
		if l.compact {
			if len(rt.path) > 0 {
				arrows = append(arrows, arrow{rt.path[len(rt.path)-1], st})
			}
			continue
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
	for _, a := range arrows { // lit ones last, so they win a shared cell
		if a.st == base {
			cv.set(a.p.x, a.p.y, '▼', a.st)
		}
	}
	for _, a := range arrows {
		if a.st != base {
			cv.set(a.p.x, a.p.y, '▼', a.st)
		}
	}
	return cv
}

// compactStyle turns the border style a full card would get into the band
// a compact card is drawn as.
func compactStyle(border style, done bool) style {
	switch {
	case done && border != stBorder && border != stBorderDone:
		return stCardDoneSel
	case done:
		return stCardDone
	case border == stBorderSel:
		return stCardSel
	case border == stBorderTarget:
		return stCardTarget
	case border == stBorderMove:
		return stCardMove
	}
	return stCard
}

// drawCompactCard draws a card as a one-line band: the title (✓ first
// when done), and at the right end the task list's progress and ↗ when it
// has a link. Meters and the completion comment are left out; K shows
// them.
func drawCompactCard(cv *canvas, l layout, n board.Node, st style, ns nodeStats) {
	x, y, w := l.colX(n.Col), l.rowY[n.Row], l.cardW
	fill := st // the strikethrough of a done title, not of the whole band
	switch st {
	case stCardDone:
		fill = stCardDoneBand
	case stCardDoneSel:
		fill = stCardSel
	}
	for i := 0; i < w; i++ {
		cv.set(x+i, y, ' ', fill)
	}
	right, rst := "", fill
	if p := ns.progress(); p != "" {
		right = p
		if st == stCard {
			rst = stCardChecks
			if ns.checked == ns.checks {
				rst = stCardChecksDone
			}
		}
	}
	if n.URL != "" {
		right += "↗"
	}
	rw := runewidth.StringWidth(right)
	if rw > w/2 {
		right, rw = "", 0
	}
	label := n.Title
	if n.Done {
		label = "✓ " + label
	}
	if label == "" {
		label = "…"
	}
	room := w - 2
	if rw > 0 {
		room -= rw + 1
	}
	cv.text(x+1, y, runewidth.Truncate(label, max(room, 1), "…"), st)
	if rw > 0 {
		cv.text(x+w-1-rw, y, right, rst)
	}
}

// drawCompactEmpty marks an empty cell as a dashed line: the cursor, or
// with a title, the ghost of a yanked node.
func drawCompactEmpty(cv *canvas, l layout, row, col int, title string, st style) {
	x, y, w := l.colX(col), l.rowY[row], l.cardW
	for i := 0; i < w; i++ {
		cv.set(x+i, y, '╌', st)
	}
	if title != "" {
		cv.text(x+1, y, " "+runewidth.Truncate(title, max(w-4, 1), "…")+" ", stPreviewDim)
	}
}
