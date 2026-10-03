package ui

import (
	"container/heap"
	"fmt"
	"sort"
	"strings"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

type point struct{ x, y int }

// route is one edge drawn on the canvas: it leaves the source card through
// srcPort on its bottom border, runs along path, and enters the target card
// through dstPort on its top border.
type route struct {
	from, to         string
	srcPort, dstPort point
	path             []point
}

// Routing cost knobs. Overlapping another edge in the same direction is
// expensive so parallel edges spread out; crossing one is cheap.
const (
	costStep    = 1
	costTurn    = 3
	costOverlap = 12
	costCross   = 1
	costSide    = 40
)

// routeEdges lays out every edge with a turn-penalised shortest path that
// only moves down, left or right, so lines never double back upward.
func routeEdges(b *board.Board, l layout) []route {
	idx := index(b)
	w, h := l.width, l.height
	blocked := make([]bool, w*h)
	for _, n := range b.Nodes {
		x0, y0 := l.colX(n.Col), l.rowY[n.Row]
		for y := y0; y < y0+cardH; y++ {
			for x := x0; x < x0+l.cardW; x++ {
				blocked[y*w+x] = true
			}
		}
	}

	src, dst := assignPorts(b, l, idx)

	// Running down along a frame's side would hide it; crossing it is fine.
	sides := make([]bool, w*h)
	for _, f := range l.frames {
		for y := f.y0; y <= f.y1; y++ {
			sides[y*w+f.x0], sides[y*w+f.x1] = true, true
		}
	}

	edges := append([]board.Edge(nil), b.Edges...)
	sort.SliceStable(edges, func(i, j int) bool {
		return span(idx, edges[i]) < span(idx, edges[j])
	})

	usedH := make([]bool, w*h)
	usedV := make([]bool, w*h)
	var out []route
	for _, e := range edges {
		f, t := idx[e.From], idx[e.To]
		if f == nil || t == nil || t.Row <= f.Row {
			continue
		}
		rt := route{from: e.From, to: e.To, srcPort: src[e], dstPort: dst[e]}
		start := point{rt.srcPort.x, rt.srcPort.y + 1}
		end := point{rt.dstPort.x, rt.dstPort.y - 1}
		rt.path = shortestPath(w, start, end, blocked, usedH, usedV, l.noH, sides)
		for i := 1; i < len(rt.path); i++ {
			p, q := rt.path[i-1], rt.path[i]
			if p.y == q.y {
				usedH[p.y*w+p.x], usedH[q.y*w+q.x] = true, true
			} else {
				usedV[p.y*w+p.x], usedV[q.y*w+q.x] = true, true
			}
		}
		if len(rt.path) == 1 {
			usedV[start.y*w+start.x] = true
		}
		out = append(out, rt)
	}
	return out
}

func span(idx map[string]*board.Node, e board.Edge) int {
	f, t := idx[e.From], idx[e.To]
	if f == nil || t == nil {
		return 0
	}
	return (t.Row-f.Row)*board.Cols + abs(t.Col-f.Col)
}

// assignPorts spreads a card's outgoing edges across its bottom border and
// incoming edges across its top border, ordered by the other end's column so
// lines leave toward where they are heading.
func assignPorts(b *board.Board, l layout, idx map[string]*board.Node) (src, dst map[board.Edge]point) {
	out := map[string][]board.Edge{}
	in := map[string][]board.Edge{}
	for _, e := range b.Edges {
		out[e.From] = append(out[e.From], e)
		in[e.To] = append(in[e.To], e)
	}
	byCol := func(es []board.Edge, other func(board.Edge) string) {
		sort.SliceStable(es, func(i, j int) bool {
			a, c := idx[other(es[i])], idx[other(es[j])]
			if a == nil || c == nil {
				return false
			}
			if a.Col != c.Col {
				return a.Col < c.Col
			}
			return a.Row < c.Row
		})
	}
	portX := func(n *board.Node, k, m int) int {
		inner := l.cardW - 2
		return l.colX(n.Col) + 1 + (k+1)*inner/(m+1)
	}
	src, dst = map[board.Edge]point{}, map[board.Edge]point{}
	for id, es := range out {
		n := idx[id]
		if n == nil {
			continue
		}
		byCol(es, func(e board.Edge) string { return e.To })
		for k, e := range es {
			src[e] = point{portX(n, k, len(es)), l.rowY[n.Row] + cardH - 1}
		}
	}
	for id, es := range in {
		n := idx[id]
		if n == nil {
			continue
		}
		byCol(es, func(e board.Edge) string { return e.From })
		for k, e := range es {
			dst[e] = point{portX(n, k, len(es)), l.rowY[n.Row]}
		}
	}
	return src, dst
}

const (
	moveDown = iota
	moveLeft
	moveRight
)

type pqItem struct{ cost, state int }
type pq []pqItem

func (q pq) Len() int           { return len(q) }
func (q pq) Less(i, j int) bool { return q[i].cost < q[j].cost }
func (q pq) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *pq) Push(x any)        { *q = append(*q, x.(pqItem)) }
func (q *pq) Pop() any          { old := *q; it := old[len(old)-1]; *q = old[:len(old)-1]; return it }

// shortestPath runs Dijkstra over (cell, last move) states between the
// start row and the end row. Lines in noH (session breaks, frame tops and
// bottoms) are only crossed straight down, never run along; running down a
// frame side (sides) costs extra.
func shortestPath(w int, start, end point, blocked, usedH, usedV []bool, noH map[int]bool, sides []bool) []point {
	if start.y > end.y {
		return nil
	}
	rows := end.y - start.y + 1
	n := w * rows * 3
	dist := make([]int, n)
	prev := make([]int, n)
	for i := range dist {
		dist[i] = 1 << 30
		prev[i] = -1
	}
	enc := func(x, y, d int) int { return ((y-start.y)*w+x)*3 + d }
	dec := func(s int) (x, y, d int) {
		d = s % 3
		c := s / 3
		return c % w, c/w + start.y, d
	}
	s0 := enc(start.x, start.y, moveDown)
	dist[s0] = 0
	q := &pq{{0, s0}}
	goal := -1
	for q.Len() > 0 {
		it := heap.Pop(q).(pqItem)
		if it.cost > dist[it.state] {
			continue
		}
		x, y, d := dec(it.state)
		if x == end.x && y == end.y {
			goal = it.state
			break
		}
		for nd, delta := range [3]point{{0, 1}, {-1, 0}, {1, 0}} {
			if (d == moveLeft && nd == moveRight) || (d == moveRight && nd == moveLeft) {
				continue
			}
			nx, ny := x+delta.x, y+delta.y
			if nx < 0 || nx >= w || ny > end.y || blocked[ny*w+nx] || (nd != moveDown && noH[ny]) {
				continue
			}
			k := ny*w + nx
			c := it.cost + costStep
			if nd != d {
				c += costTurn
			}
			if nd == moveDown {
				if sides[k] {
					c += costSide
				}
				if usedV[k] {
					c += costOverlap
				}
				if usedH[k] {
					c += costCross
				}
			} else {
				if usedH[k] {
					c += costOverlap
				}
				if usedV[k] {
					c += costCross
				}
			}
			ns := enc(nx, ny, nd)
			if c < dist[ns] {
				dist[ns] = c
				prev[ns] = it.state
				heap.Push(q, pqItem{c, ns})
			}
		}
	}
	if goal < 0 {
		return nil
	}
	var path []point
	for s := goal; s >= 0; s = prev[s] {
		x, y, _ := dec(s)
		path = append(path, point{x, y})
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// routeKey captures everything routing depends on, so routes are recomputed
// only when positions, edges or the terminal width change.
func routeKey(b *board.Board, width int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d|", width)
	for _, n := range b.Nodes {
		fmt.Fprintf(&sb, "%s:%d:%d,", n.ID, n.Row, n.Col)
	}
	sb.WriteByte('|')
	for _, e := range b.Edges {
		fmt.Fprintf(&sb, "%s>%s,", e.From, e.To)
	}
	sb.WriteByte('|')
	for _, br := range b.Breaks {
		fmt.Fprintf(&sb, "%d,", br.After)
	}
	for _, g := range b.Groups {
		fmt.Fprintf(&sb, "|%v", g.Members)
	}
	return sb.String()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
