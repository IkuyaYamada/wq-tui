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
	cardH  = 3
	gap    = 2
	margin = 1
)

// layout maps grid cells to canvas coordinates. Rows are separated by lanes
// whose height grows with the number of edges that must pass through them.
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
	rows := b.MaxRow() + 1
	// An edge that changes column needs a horizontal run, either in the
	// lane right below its source or the lane right above its target.
	turning := make([]int, rows)
	idx := index(b)
	for _, e := range b.Edges {
		f, t := idx[e.From], idx[e.To]
		if f == nil || t == nil || f.Col == t.Col {
			continue
		}
		turning[f.Row]++
		if t.Row-1 != f.Row {
			turning[t.Row-1]++
		}
	}
	y := 0
	for r := 0; r < rows; r++ {
		l.rowY = append(l.rowY, y)
		y += cardH + 1 + min(turning[r], 2)
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
	stEdge
	stEdgeHL
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
	stEdge:         lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	stEdgeHL:       lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
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
)

func drawCard(cv *canvas, l layout, n board.Node, border, title style, bold bool) {
	f := frameNormal
	if bold {
		f = frameBold
	}
	x, y := l.colX(n.Col), l.rowY[n.Row]
	w := l.cardW
	cv.set(x, y, f[0], border)
	cv.set(x+w-1, y, f[2], border)
	cv.set(x, y+1, f[3], border)
	cv.set(x+w-1, y+1, f[3], border)
	cv.set(x, y+2, f[4], border)
	cv.set(x+w-1, y+2, f[5], border)
	for i := 1; i < w-1; i++ {
		cv.set(x+i, y, f[1], border)
		cv.set(x+i, y+2, f[1], border)
	}
	label := n.Title
	if n.Done {
		label = "✓ " + label
	}
	if label == "" {
		label = "…"
	}
	cv.text(x+2, y+1, runewidth.Truncate(label, w-4, "…"), title)
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

// renderBoard draws every card and routed edge onto a canvas. sel is the
// selected node; target (connect mode) and moving (move mode) get their own
// highlight.
func renderBoard(b *board.Board, l layout, routes []route, sel, target, moving string) *canvas {
	cv := newCanvas(l.width, l.height)
	for _, n := range b.Nodes {
		border, title, bold := stBorder, stTitle, false
		switch {
		case n.ID == moving:
			border, title, bold = stBorderMove, stTitleSel, true
		case n.ID == target:
			border, title, bold = stBorderTarget, stTitleSel, true
		case n.ID == sel:
			border, title, bold = stBorderSel, stTitleSel, true
		case n.Done:
			border, title = stBorderDone, stTitleDone
		}
		if n.Done && title != stTitleSel {
			title = stTitleDone
		}
		drawCard(cv, l, n, border, title, bold)
	}

	bits := make([]uint8, cv.w*cv.h)
	hl := make([]bool, cv.w*cv.h)
	for _, rt := range routes {
		lit := rt.from == sel || rt.to == sel
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
