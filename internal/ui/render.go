package ui

import (
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
// fixed-height lanes for edges.
type layout struct {
	cardW  int
	rowY   []int
	width  int
	height int
}

func newLayout(b *board.Board, termW int) layout {
	l := layout{cardW: (termW - 2*margin - (board.Cols-1)*gap) / board.Cols}
	l.cardW = max(l.cardW, 8)
	l.width = 2*margin + board.Cols*l.cardW + (board.Cols-1)*gap
	rows := b.MaxRow() + 1 + bufferRows
	y := 0
	for r := 0; r < rows; r++ {
		l.rowY = append(l.rowY, y)
		y += cardH + laneH
	}
	l.height = y
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
	stEdge
	stEdgeHL
	stDot
)

var styles = map[style]lipgloss.Style{
	stBorder:       lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
	stBorderSel:    lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),
	stBorderDone:   lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
	stBorderTarget: lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true),
	stBorderMove:   lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true),
	stTitle:        lipgloss.NewStyle(),
	stTitleSel:     lipgloss.NewStyle().Bold(true),
	stTitleDone:    lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Strikethrough(true),
	stTitleDoneSel: lipgloss.NewStyle().Foreground(lipgloss.Color("248")).Strikethrough(true),
	stDoneNote:     lipgloss.NewStyle().Foreground(lipgloss.Color("108")).Italic(true),
	stBreak:        lipgloss.NewStyle().Foreground(lipgloss.Color("179")),
	stEdge:         lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	stEdgeHL:       lipgloss.NewStyle().Foreground(lipgloss.Color("247")), // a notch above stEdge
	stDot:          lipgloss.NewStyle().Foreground(lipgloss.Color("237")),
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

func (c *canvas) set(x, y int, r rune, st style) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	c.cells[y*c.w+x] = cell{r: r, st: st}
}

func (c *canvas) text(x, y int, s string, st style) {
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		c.set(x, y, r, st)
		if w == 2 && x+1 < c.w {
			c.cells[y*c.w+x+1] = cell{cont: true, st: st}
		}
		x += w
	}
}

func (c *canvas) line(y int) string {
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
	for x := 0; x < c.w; x++ {
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
	return strings.TrimRight(sb.String(), " ")
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

func drawCard(cv *canvas, l layout, n board.Node, border, title style, bold bool) {
	f := frameNormal
	if bold {
		f = frameBold
	}
	drawFrame(cv, l, n.Row, n.Col, f, border)
	x, y, w := l.colX(n.Col), l.rowY[n.Row], l.cardW
	if n.URL != "" {
		cv.set(x+w-3, y, '↗', border) // has a link: gx opens it
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

// drawBreaks draws each session break as a dotted line across the gap under
// its row. Edges are drawn later and win where they cross it.
func drawBreaks(cv *canvas, l layout, breaks []board.Break) {
	for _, br := range breaks {
		if br.After < 0 || br.After >= len(l.rowY) {
			continue
		}
		y := l.rowY[br.After] + cardH
		for x := 0; x < cv.w; x++ {
			cv.set(x, y, '┄', stBreak)
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
		y := l.rowY[br.After] + cardH
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
}

// renderBoard draws every card, the cursor and the routed edges.
func renderBoard(b *board.Board, l layout, routes []route, v view) *canvas {
	cv := newCanvas(l.width, l.height)
	drawBreaks(cv, l, b.Breaks)
	// A faint dot marks every cell so empty rows still read as a grid.
	for r := range l.rowY {
		for c := 0; c < board.Cols; c++ {
			cv.set(l.colX(c)+l.cardW/2, l.rowY[r]+1, '·', stDot)
		}
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
		drawCard(cv, l, n, border, title, bold)
	}
	if !onCursor && v.cursorRow >= 0 && v.cursorRow < len(l.rowY) {
		drawFrame(cv, l, v.cursorRow, v.cursorCol, frameDashed, v.cursorSt)
	}

	bits := make([]uint8, cv.w*cv.h)
	hl := make([]bool, cv.w*cv.h)
	for _, rt := range routes {
		lit := v.lit != "" && (rt.from == v.lit || rt.to == v.lit)
		st := stEdge
		if lit {
			st = stEdgeHL
		}
		for i, p := range rt.path {
			k := p.y*cv.w + p.x
			if i == 0 {
				bits[k] |= bitU
			}
			if i == len(rt.path)-1 {
				bits[k] |= bitD
			}
			if i > 0 {
				q := rt.path[i-1]
				kq := q.y*cv.w + q.x
				switch {
				case q.y < p.y:
					bits[kq] |= bitD
					bits[k] |= bitU
				case q.x < p.x:
					bits[kq] |= bitR
					bits[k] |= bitL
				default:
					bits[kq] |= bitL
					bits[k] |= bitR
				}
			}
			hl[k] = hl[k] || lit
		}
		cv.set(rt.srcPort.x, rt.srcPort.y, '┬', st)
		cv.set(rt.dstPort.x, rt.dstPort.y, '▼', st)
	}
	drawBreakLabels(cv, l, b.Breaks, bits)
	for k, v := range bits {
		if v == 0 {
			continue
		}
		st := stEdge
		if hl[k] {
			st = stEdgeHL
		}
		cv.cells[k] = cell{r: edgeRune(v), st: st}
	}
	return cv
}
