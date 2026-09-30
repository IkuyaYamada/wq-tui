// Package board holds the six-column node grid and the rules that keep it
// consistent: one node per cell, and every edge pointing strictly downward.
package board

import (
	"errors"
	"sort"
	"time"
)

// Cols is the fixed number of grid columns.
const Cols = 6

type Node struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Row       int        `json:"row"`
	Col       int        `json:"col"`
	Done      bool       `json:"done"`
	CreatedAt time.Time  `json:"created_at"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Board struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Dir int

const (
	Left Dir = iota
	Down
	Up
	Right
)

func (d Dir) delta() (dr, dc int) {
	switch d {
	case Left:
		return 0, -1
	case Right:
		return 0, 1
	case Up:
		return -1, 0
	default:
		return 1, 0
	}
}

var (
	ErrNotFound = errors.New("node not found")
	ErrSelf     = errors.New("cannot connect a node to itself")
)

func (b *Board) Clone() *Board {
	return &Board{
		Nodes: append([]Node(nil), b.Nodes...),
		Edges: append([]Edge(nil), b.Edges...),
	}
}

// Node returns a pointer into b.Nodes; it is invalidated by any append.
func (b *Board) Node(id string) *Node {
	for i := range b.Nodes {
		if b.Nodes[i].ID == id {
			return &b.Nodes[i]
		}
	}
	return nil
}

func (b *Board) At(row, col int) *Node {
	for i := range b.Nodes {
		if b.Nodes[i].Row == row && b.Nodes[i].Col == col {
			return &b.Nodes[i]
		}
	}
	return nil
}

// MaxRow is the lowest occupied row, or -1 for an empty board.
func (b *Board) MaxRow() int {
	m := -1
	for _, n := range b.Nodes {
		m = max(m, n.Row)
	}
	return m
}

func (b *Board) Preds(id string) []string {
	var out []string
	for _, e := range b.Edges {
		if e.To == id {
			out = append(out, e.From)
		}
	}
	return out
}

func (b *Board) Succs(id string) []string {
	var out []string
	for _, e := range b.Edges {
		if e.From == id {
			out = append(out, e.To)
		}
	}
	return out
}

func (b *Board) HasEdge(from, to string) bool {
	for _, e := range b.Edges {
		if e.From == from && e.To == to {
			return true
		}
	}
	return false
}

// Sort orders nodes by position so board.json diffs stay readable.
func (b *Board) Sort() {
	sort.SliceStable(b.Nodes, func(i, j int) bool {
		if b.Nodes[i].Row != b.Nodes[j].Row {
			return b.Nodes[i].Row < b.Nodes[j].Row
		}
		return b.Nodes[i].Col < b.Nodes[j].Col
	})
}

func (b *Board) shiftRows(from int) {
	for i := range b.Nodes {
		if b.Nodes[i].Row >= from {
			b.Nodes[i].Row++
		}
	}
}

// NearestEmpty finds the empty cell closest to (row, col) by Manhattan
// distance, breaking ties by exploring right, down, left, up.
func (b *Board) NearestEmpty(row, col int) (int, int) {
	type cell struct{ r, c int }
	limit := b.MaxRow() + 1
	seen := map[cell]bool{{row, col}: true}
	queue := []cell{{row, col}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if b.At(cur.r, cur.c) == nil {
			return cur.r, cur.c
		}
		for _, d := range []Dir{Right, Down, Left, Up} {
			dr, dc := d.delta()
			next := cell{cur.r + dr, cur.c + dc}
			if next.c < 0 || next.c >= Cols || next.r < 0 || next.r > limit || seen[next] {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	return limit, 0
}

// Add places n in the empty cell nearest to (row, col).
func (b *Board) Add(n Node, row, col int) {
	n.Row, n.Col = b.NearestEmpty(row, col)
	b.Nodes = append(b.Nodes, n)
}

// InsertAfter places n directly below id, pushes every lower row down by
// one, and hands id's outgoing edges over to n: A → B becomes A → n → B.
func (b *Board) InsertAfter(id string, n Node) error {
	a := b.Node(id)
	if a == nil {
		return ErrNotFound
	}
	row, col := a.Row, a.Col
	b.shiftRows(row + 1)
	n.Row, n.Col = row+1, col
	for i := range b.Edges {
		if b.Edges[i].From == id {
			b.Edges[i].From = n.ID
		}
	}
	b.Nodes = append(b.Nodes, n)
	b.Edges = append(b.Edges, Edge{From: id, To: n.ID})
	return nil
}

// InsertBefore places n in id's cell, pushes id and every lower row down by
// one, and hands id's incoming edges over to n: A → B becomes A → n → B.
func (b *Board) InsertBefore(id string, n Node) error {
	t := b.Node(id)
	if t == nil {
		return ErrNotFound
	}
	row, col := t.Row, t.Col
	b.shiftRows(row)
	n.Row, n.Col = row, col
	for i := range b.Edges {
		if b.Edges[i].To == id {
			b.Edges[i].To = n.ID
		}
	}
	b.Nodes = append(b.Nodes, n)
	b.Edges = append(b.Edges, Edge{From: n.ID, To: id})
	return nil
}

// DeleteRow removes a row: its nodes are deleted (bridging simple chains as
// Delete does) and every row below is pulled up by one. Relative row order
// is unchanged, so every edge still points down.
func (b *Board) DeleteRow(row int) {
	var ids []string
	for _, n := range b.Nodes {
		if n.Row == row {
			ids = append(ids, n.ID)
		}
	}
	for _, id := range ids {
		_ = b.Delete(id)
	}
	for i := range b.Nodes {
		if b.Nodes[i].Row > row {
			b.Nodes[i].Row--
		}
	}
}

// Delete removes id and its edges. A node with exactly one incoming and one
// outgoing edge is bridged, so A → B → C becomes A → C.
func (b *Board) Delete(id string) error {
	if b.Node(id) == nil {
		return ErrNotFound
	}
	preds, succs := b.Preds(id), b.Succs(id)
	nodes := b.Nodes[:0]
	for _, n := range b.Nodes {
		if n.ID != id {
			nodes = append(nodes, n)
		}
	}
	b.Nodes = nodes
	edges := b.Edges[:0]
	for _, e := range b.Edges {
		if e.From != id && e.To != id {
			edges = append(edges, e)
		}
	}
	b.Edges = edges
	if len(preds) == 1 && len(succs) == 1 && !b.HasEdge(preds[0], succs[0]) {
		b.Edges = append(b.Edges, Edge{From: preds[0], To: succs[0]})
	}
	return nil
}

// ToggleEdge removes the edge between from and to if one exists, in either
// direction, and creates it otherwise. Edges always point down, so picking a
// node above connects it to from rather than the other way round. Connecting
// two nodes on the same row first pushes the target and every lower row down
// by one. Since edges always point down, cycles are impossible.
func (b *Board) ToggleEdge(from, to string) (added bool, err error) {
	if from == to {
		return false, ErrSelf
	}
	for i, e := range b.Edges {
		if (e.From == from && e.To == to) || (e.From == to && e.To == from) {
			b.Edges = append(b.Edges[:i], b.Edges[i+1:]...)
			return false, nil
		}
	}
	f, t := b.Node(from), b.Node(to)
	if f == nil || t == nil {
		return false, ErrNotFound
	}
	if t.Row < f.Row {
		b.Edges = append(b.Edges, Edge{From: to, To: from})
		return true, nil
	}
	if t.Row == f.Row {
		row := f.Row
		for i := range b.Nodes {
			if b.Nodes[i].Row > row || b.Nodes[i].ID == to {
				b.Nodes[i].Row++
			}
		}
	}
	b.Edges = append(b.Edges, Edge{From: from, To: to})
	return true, nil
}

// MoveStep slides id to the first empty cell in direction d, hopping over
// occupied cells. It refuses moves that would break an edge's direction.
func (b *Board) MoveStep(id string, d Dir) bool {
	return b.MoveGroup([]string{id}, d)
}

// MoveGroup slides ids together, keeping their shape, by the smallest number
// of steps in direction d that lands every one of them on a cell not taken
// by a node outside the group. It refuses moves that leave the grid or turn
// an edge between the group and the rest flat or upward.
func (b *Board) MoveGroup(ids []string, d Dir) bool {
	in := make(map[string]bool, len(ids))
	for _, id := range ids {
		if b.Node(id) == nil {
			return false
		}
		in[id] = true
	}
	if len(in) == 0 {
		return false
	}
	taken := map[[2]int]bool{}
	for _, n := range b.Nodes {
		if !in[n.ID] {
			taken[[2]int{n.Row, n.Col}] = true
		}
	}
	dr, dc := d.delta()
	limit := b.MaxRow() + 1
	for k := 1; ; k++ {
		fits := true
		for _, n := range b.Nodes {
			if !in[n.ID] {
				continue
			}
			r, c := n.Row+dr*k, n.Col+dc*k
			if c < 0 || c >= Cols || r < 0 || r > limit+k*max(dr, 0) {
				return false
			}
			if taken[[2]int{r, c}] {
				fits = false
			}
		}
		if !fits {
			continue
		}
		row := func(id string) int {
			n := b.Node(id)
			if in[id] {
				return n.Row + dr*k
			}
			return n.Row
		}
		for _, e := range b.Edges {
			if in[e.From] != in[e.To] && row(e.From) >= row(e.To) {
				return false
			}
		}
		for i := range b.Nodes {
			if in[b.Nodes[i].ID] {
				b.Nodes[i].Row += dr * k
				b.Nodes[i].Col += dc * k
			}
		}
		return true
	}
}
