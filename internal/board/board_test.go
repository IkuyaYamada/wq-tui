package board

import (
	"testing"
	"time"
)

func node(id string, row, col int) Node {
	return Node{ID: id, Title: id, Row: row, Col: col, CreatedAt: time.Unix(0, 0)}
}

func pos(t *testing.T, b *Board, id string) [2]int {
	t.Helper()
	n := b.Node(id)
	if n == nil {
		t.Fatalf("node %s missing", id)
	}
	return [2]int{n.Row, n.Col}
}

func TestInsertAfterTakesOverOutgoingEdges(t *testing.T) {
	b := &Board{Nodes: []Node{node("A", 0, 0), node("B", 1, 0), node("C", 1, 3)}, Edges: []Edge{{"A", "B"}}}
	if err := b.InsertAfter("A", node("N", 0, 0)); err != nil {
		t.Fatal(err)
	}
	if got := pos(t, b, "N"); got != [2]int{1, 0} {
		t.Errorf("N at %v", got)
	}
	if got := pos(t, b, "B"); got != [2]int{2, 0} {
		t.Errorf("B at %v", got)
	}
	if got := pos(t, b, "C"); got != [2]int{2, 3} {
		t.Errorf("C at %v, rows below should shift too", got)
	}
	if !b.HasEdge("A", "N") || !b.HasEdge("N", "B") || b.HasEdge("A", "B") {
		t.Errorf("edges %v", b.Edges)
	}
}

func TestInsertBeforeTakesOverIncomingEdges(t *testing.T) {
	b := &Board{Nodes: []Node{node("A", 0, 0), node("B", 0, 1), node("C", 1, 0)}, Edges: []Edge{{"A", "C"}, {"B", "C"}}}
	if err := b.InsertBefore("C", node("N", 0, 0)); err != nil {
		t.Fatal(err)
	}
	if got := pos(t, b, "N"); got != [2]int{1, 0} {
		t.Errorf("N at %v", got)
	}
	if got := pos(t, b, "C"); got != [2]int{2, 0} {
		t.Errorf("C at %v", got)
	}
	for _, e := range []Edge{{"A", "N"}, {"B", "N"}, {"N", "C"}} {
		if !b.HasEdge(e.From, e.To) {
			t.Errorf("missing %v in %v", e, b.Edges)
		}
	}
}

func TestDeleteBridgesSimpleChain(t *testing.T) {
	b := &Board{Nodes: []Node{node("A", 0, 0), node("B", 1, 0), node("C", 2, 0)}, Edges: []Edge{{"A", "B"}, {"B", "C"}}}
	if err := b.Delete("B"); err != nil {
		t.Fatal(err)
	}
	if len(b.Edges) != 1 || !b.HasEdge("A", "C") {
		t.Errorf("edges %v", b.Edges)
	}
}

func TestToggleEdge(t *testing.T) {
	b := &Board{Nodes: []Node{node("A", 1, 0), node("B", 0, 0), node("C", 1, 2), node("D", 2, 0)}}
	if _, err := b.ToggleEdge("A", "B"); err != ErrUpward {
		t.Errorf("upward edge: err = %v", err)
	}
	added, err := b.ToggleEdge("A", "C")
	if err != nil || !added {
		t.Fatalf("same-row connect: added=%v err=%v", added, err)
	}
	if got := pos(t, b, "C"); got != [2]int{2, 2} {
		t.Errorf("C should drop one row, at %v", got)
	}
	if got := pos(t, b, "D"); got != [2]int{3, 0} {
		t.Errorf("D below should drop too, at %v", got)
	}
	if got := pos(t, b, "A"); got != [2]int{1, 0} {
		t.Errorf("A should stay, at %v", got)
	}
	added, err = b.ToggleEdge("A", "C")
	if err != nil || added || len(b.Edges) != 0 {
		t.Errorf("second toggle should remove: added=%v err=%v edges=%v", added, err, b.Edges)
	}
}

func TestMoveStepHopsAndRespectsEdges(t *testing.T) {
	b := &Board{Nodes: []Node{node("A", 0, 0), node("B", 0, 1), node("C", 1, 0)}, Edges: []Edge{{"A", "C"}}}
	if !b.MoveStep("A", Right) {
		t.Fatal("move right failed")
	}
	if got := pos(t, b, "A"); got != [2]int{0, 2} {
		t.Errorf("A should hop over B, at %v", got)
	}
	if b.MoveStep("A", Down) {
		t.Errorf("A moved onto C's row: %v", pos(t, b, "A"))
	}
	if b.MoveStep("C", Up) {
		t.Errorf("C moved above its predecessor")
	}
}

func TestNearestEmpty(t *testing.T) {
	b := &Board{Nodes: []Node{node("A", 0, 0), node("B", 0, 1)}}
	if r, c := b.NearestEmpty(0, 0); r != 1 || c != 0 {
		t.Errorf("got %d,%d", r, c)
	}
	full := &Board{}
	for c := 0; c < Cols; c++ {
		full.Nodes = append(full.Nodes, node(string(rune('a'+c)), 0, c))
	}
	if r, c := full.NearestEmpty(0, 5); r != 1 || c != 5 {
		t.Errorf("full row: got %d,%d", r, c)
	}
}
