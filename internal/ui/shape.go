package ui

import (
	"container/heap"
	"sort"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

// frame is a group drawn on the canvas. Its outline hugs the members'
// cards, running in the gaps around them, so it takes any shape they make;
// members that sit apart are joined by necks, thin lines routed like edges,
// so the pieces still read as one frame.
type frame struct {
	id, title   string
	color       int // index into framePalette
	done, total int
	members     map[string]bool
	lines       map[point]uint8 // outline and necks, as connection bits
}

// piece is one 4-connected cluster of a group's member cells: its area on
// the canvas and the outline around it.
type piece struct {
	cells   [][2]int // grid (row, col), in reading order
	mask    map[point]bool
	outline map[point]uint8
}

// layoutFrames shapes every group: first each piece's area and outline,
// then the necks between pieces, which steer clear of every card and of
// every frame's area.
func layoutFrames(b *board.Board, l layout) []frame {
	type shaped struct {
		f      frame
		pieces []piece
	}
	var all []shaped
	taken := map[point]bool{}
	for _, n := range b.Nodes {
		if n.Row >= len(l.rowY) {
			continue
		}
		x0, y0 := l.colX(n.Col), l.rowY[n.Row]
		for y := y0; y < y0+cardH; y++ {
			for x := x0; x < x0+l.cardW; x++ {
				taken[point{x, y}] = true
			}
		}
	}
	for _, g := range b.Groups {
		f := frame{members: map[string]bool{}, lines: map[point]uint8{}}
		f.label(b, g)
		cells := map[[2]int]bool{}
		for _, id := range g.Members {
			if n := b.Node(id); n != nil && n.Row < len(l.rowY) {
				f.members[id] = true
				cells[[2]int{n.Row, n.Col}] = true
			}
		}
		if len(cells) == 0 {
			continue
		}
		s := shaped{f: f, pieces: pieces(l, cells)}
		for _, p := range s.pieces {
			for q := range p.mask {
				taken[q] = true
			}
			for q, bits := range p.outline {
				f.lines[q] |= bits
			}
		}
		all = append(all, s)
	}
	out := make([]frame, len(all))
	for i, s := range all {
		for _, link := range spanningLinks(s.pieces) {
			a, z := s.pieces[link[0]], s.pieces[link[1]]
			path := neckPath(l, a, z, taken)
			for j := 1; j < len(path); j++ {
				p, q := path[j-1], path[j]
				switch {
				case q.y > p.y:
					s.f.lines[p] |= bitD
					s.f.lines[q] |= bitU
				case q.y < p.y:
					s.f.lines[p] |= bitU
					s.f.lines[q] |= bitD
				case q.x > p.x:
					s.f.lines[p] |= bitR
					s.f.lines[q] |= bitL
				default:
					s.f.lines[p] |= bitL
					s.f.lines[q] |= bitR
				}
			}
		}
		out[i] = s.f
	}
	return out
}

// label fills in what a frame shows beyond its shape: title, color and
// progress, which change without the shape changing.
func (f *frame) label(b *board.Board, g board.Group) {
	f.id, f.title, f.color = g.ID, g.Title, frameColor(g)
	f.done, f.total = 0, 0
	for _, id := range g.Members {
		if n := b.Node(id); n != nil {
			f.total++
			if n.Done {
				f.done++
			}
		}
	}
}

// relabel brings cached frames up to date with the board's titles, colors
// and done marks.
func relabel(b *board.Board, frames []frame) []frame {
	out := make([]frame, 0, len(frames))
	for _, f := range frames {
		if g := b.Group(f.id); g != nil {
			f.label(b, *g)
			out = append(out, f)
		}
	}
	return out
}

func frameColor(g board.Group) int {
	return (max(g.Color, 1) - 1) % board.FrameColors
}

// pieces splits member cells into 4-connected clusters and shapes each:
// every card with a cell of room around it, and the gaps between members
// stacked in a column filled in, so the outline runs round the whole
// cluster in the gaps between cards.
func pieces(l layout, cells map[[2]int]bool) []piece {
	order := make([][2]int, 0, len(cells))
	for c := range cells {
		order = append(order, c)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i][0] != order[j][0] {
			return order[i][0] < order[j][0]
		}
		return order[i][1] < order[j][1]
	})
	seen := map[[2]int]bool{}
	var out []piece
	for _, c := range order {
		if seen[c] {
			continue
		}
		p := piece{mask: map[point]bool{}, outline: map[point]uint8{}}
		seen[c] = true
		queue := [][2]int{c}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			p.cells = append(p.cells, cur)
			for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				nb := [2]int{cur[0] + d[0], cur[1] + d[1]}
				if cells[nb] && !seen[nb] {
					seen[nb] = true
					queue = append(queue, nb)
				}
			}
		}
		for _, cell := range p.cells {
			r, col := cell[0], cell[1]
			x0, x1 := l.colX(col)-1, l.colX(col)+l.cardW
			y1 := l.rowY[r] + cardH
			if cells[[2]int{r + 1, col}] && r+1 < len(l.rowY) {
				y1 = l.rowY[r+1] - 1
			}
			for y := l.rowY[r] - 1; y <= y1; y++ {
				for x := x0; x <= x1; x++ {
					p.mask[point{x, y}] = true
				}
			}
		}
		// The outline is every cell of the area with a neighbour outside
		// it, diagonals included, joined to its outline neighbours.
		edge := func(q point) bool {
			if !p.mask[q] {
				return false
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if !p.mask[point{q.x + dx, q.y + dy}] {
						return true
					}
				}
			}
			return false
		}
		for q := range p.mask {
			if !edge(q) {
				continue
			}
			var bits uint8
			if edge(point{q.x, q.y - 1}) {
				bits |= bitU
			}
			if edge(point{q.x, q.y + 1}) {
				bits |= bitD
			}
			if edge(point{q.x - 1, q.y}) {
				bits |= bitL
			}
			if edge(point{q.x + 1, q.y}) {
				bits |= bitR
			}
			p.outline[q] = bits
		}
		out = append(out, p)
	}
	return out
}

// spanningLinks picks which pieces to join, as a minimum spanning tree over
// the grid distance between their nearest cells, so each piece reaches out
// to its closest neighbour and the necks stay short.
func spanningLinks(ps []piece) [][2]int {
	if len(ps) < 2 {
		return nil
	}
	dist := func(a, b piece) int {
		best := 1 << 30
		for _, p := range a.cells {
			for _, q := range b.cells {
				best = min(best, abs(p[0]-q[0])+abs(p[1]-q[1]))
			}
		}
		return best
	}
	in := make([]bool, len(ps))
	in[0] = true
	var links [][2]int
	for range len(ps) - 1 {
		best, bi, bj := 1<<30, -1, -1
		for i := range ps {
			if !in[i] {
				continue
			}
			for j := range ps {
				if in[j] {
					continue
				}
				if d := dist(ps[i], ps[j]); d < best {
					best, bi, bj = d, i, j
				}
			}
		}
		in[bj] = true
		links = append(links, [2]int{bi, bj})
	}
	return links
}

// straight reports whether a neck may join the outline at p: on a top or
// bottom stretch, a couple of cells clear of corners and other necks.
func (p piece) straight(q point) bool {
	for dx := -2; dx <= 2; dx++ {
		if p.outline[point{q.x + dx, q.y}] != bitL|bitR {
			return false
		}
	}
	return true
}

// neckPath routes a neck from piece a's outline to piece z's: it leaves a
// straight top or bottom stretch of one and enters such a stretch of the
// other, clear of the corners, runs through empty space (never a card or
// any frame's area), may go up as well as down, and only crosses the lines
// edges do not run along. It is nil when there is no way through.
func neckPath(l layout, a, z piece, taken map[point]bool) []point {
	ylo, yhi := l.height, 0
	for _, p := range []piece{a, z} {
		for q := range p.mask {
			ylo, yhi = min(ylo, q.y), max(yhi, q.y)
		}
	}
	ylo, yhi = max(ylo-2*cardH, 0), min(yhi+2*cardH, l.height-1)
	w := l.width
	// States are (cell, direction of the last step).
	steps := [4]point{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	enc := func(q point, d int) int { return ((q.y-ylo)*w+q.x)*4 + d }
	dec := func(s int) (point, int) {
		c := s / 4
		return point{c % w, c/w + ylo}, s % 4
	}
	n := (yhi - ylo + 1) * w * 4
	dist := make([]int, n)
	prev := make([]int, n)
	for i := range dist {
		dist[i], prev[i] = 1<<30, -1
	}
	starts := map[int]point{} // first state → the outline cell it leaves
	q := &pq{}
	inside := func(p point) bool { return p.x >= 0 && p.x < w && p.y >= ylo && p.y <= yhi }
	free := func(p point) bool { return inside(p) && !taken[p] }
	for p := range a.outline {
		if !a.straight(p) {
			continue
		}
		for d := 0; d < 2; d++ {
			nxt := point{p.x, p.y + steps[d].y}
			if a.mask[nxt] || !free(nxt) {
				continue
			}
			s := enc(nxt, d)
			if dist[s] > 1 {
				dist[s] = 1
				starts[s] = p
				heap.Push(q, pqItem{1, s})
			}
		}
	}
	const turn = 4
	for q.Len() > 0 {
		it := heap.Pop(q).(pqItem)
		if it.cost > dist[it.state] {
			continue
		}
		p, d := dec(it.state)
		for nd, st := range steps {
			if d^1 == nd { // no doubling back
				continue
			}
			nxt := point{p.x + st.x, p.y + st.y}
			if nd < 2 && z.straight(nxt) {
				path := []point{nxt}
				s := it.state
				for ; prev[s] >= 0; s = prev[s] {
					cell, _ := dec(s)
					path = append(path, cell)
				}
				first, _ := dec(s)
				path = append(path, first, starts[s])
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				return path
			}
			if !free(nxt) || (nd >= 2 && l.noH[nxt.y]) {
				continue
			}
			c := it.cost + 1
			if nd != d {
				c += turn
			}
			ns := enc(nxt, nd)
			if c < dist[ns] {
				dist[ns], prev[ns] = c, it.state
				heap.Push(q, pqItem{c, ns})
			}
		}
	}
	return nil
}
