package board

import (
	"errors"
	"math"
	"sort"
)

var ErrNoRoom = errors.New("not enough room to organize without an edge turning upward")

// Organize re-lays out the given nodes without touching any edge. Nodes in
// ids that have edges are re-layered (each as high as its predecessors
// allow, so a child sits right under its parent and empty rows close up)
// and, row by row, ordered by the mean column of their predecessors, which
// lines chains up vertically and undoes crossings. Nodes outside ids, and
// nodes in ids with no edges at all, stay where they are. If a full row
// would push a node below one of its successors outside ids, nothing is
// changed and ErrNoRoom is returned.
func (b *Board) Organize(ids []string) error {
	sel := map[string]bool{}
	for _, id := range ids {
		if b.Node(id) != nil {
			sel[id] = true
		}
	}
	linked := map[string]bool{}
	for _, e := range b.Edges {
		if sel[e.From] {
			linked[e.From] = true
		}
		if sel[e.To] {
			linked[e.To] = true
		}
	}
	if len(linked) == 0 {
		return nil
	}

	// Everything not being placed is an obstacle.
	taken := map[[2]int]bool{}
	top := math.MaxInt
	for _, n := range b.Nodes {
		if linked[n.ID] {
			top = min(top, n.Row)
		} else {
			taken[[2]int{n.Row, n.Col}] = true
		}
	}

	orig := map[string][2]int{}
	for _, n := range b.Nodes {
		orig[n.ID] = [2]int{n.Row, n.Col}
	}
	placed := map[string][2]int{}
	pos := func(id string) ([2]int, bool) {
		if linked[id] {
			p, ok := placed[id]
			return p, ok
		}
		return orig[id], true
	}

	// earliest is the first row id may take: below every predecessor, or
	// -1 while some linked predecessor is still unplaced.
	earliest := func(id string) int {
		r := top
		for _, p := range b.Preds(id) {
			pp, ok := pos(p)
			if !ok {
				return -1
			}
			r = max(r, pp[0]+1)
		}
		return r
	}
	// barycenter is the mean predecessor column, or the node's own column
	// for a root, so roots keep their left-to-right order.
	barycenter := func(id string) float64 {
		preds := b.Preds(id)
		if len(preds) == 0 {
			return float64(orig[id][1])
		}
		sum := 0.0
		for _, p := range preds {
			pp, _ := pos(p)
			sum += float64(pp[1])
		}
		return sum / float64(len(preds))
	}

	pending := make([]string, 0, len(linked))
	for id := range linked {
		pending = append(pending, id)
	}
	for row := top; len(pending) > 0; row++ {
		if row > top+len(b.Nodes)+Cols {
			return ErrNoRoom // cannot happen with downward edges; guards the loop
		}
		var ready, rest []string
		for _, id := range pending {
			if r := earliest(id); r >= 0 && r <= row {
				ready = append(ready, id)
			} else {
				rest = append(rest, id)
			}
		}
		sort.SliceStable(ready, func(i, j int) bool {
			bi, bj := barycenter(ready[i]), barycenter(ready[j])
			if bi != bj {
				return bi < bj
			}
			oi, oj := orig[ready[i]], orig[ready[j]]
			if oi[0] != oj[0] {
				return oi[0] < oj[0]
			}
			return oi[1] < oj[1]
		})
		for _, id := range ready {
			col, ok := nearestFreeCol(taken, row, int(math.Round(barycenter(id))))
			if !ok {
				rest = append(rest, id) // row is full; try the next one
				continue
			}
			taken[[2]int{row, col}] = true
			placed[id] = [2]int{row, col}
		}
		pending = rest
	}

	for _, e := range b.Edges {
		f, _ := pos(e.From)
		t, _ := pos(e.To)
		if f[0] >= t[0] {
			return ErrNoRoom
		}
	}
	for i := range b.Nodes {
		if p, ok := placed[b.Nodes[i].ID]; ok {
			b.Nodes[i].Row, b.Nodes[i].Col = p[0], p[1]
		}
	}
	return nil
}

// nearestFreeCol finds the free column on row closest to want, trying the
// left side first on ties.
func nearestFreeCol(taken map[[2]int]bool, row, want int) (int, bool) {
	want = max(0, min(want, Cols-1))
	for d := 0; d < Cols; d++ {
		for _, c := range []int{want - d, want + d} {
			if c >= 0 && c < Cols && !taken[[2]int{row, c}] {
				return c, true
			}
		}
	}
	return 0, false
}
