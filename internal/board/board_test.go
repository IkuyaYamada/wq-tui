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
	b := &Board{Nodes: []Node{node("A", 0, 0), node("B", 0, 1), node("C", 2, 0)}, Edges: []Edge{{"A", "C"}, {"B", "C"}}}
	if err := b.InsertBefore("C", node("N", 0, 0)); err != nil {
		t.Fatal(err)
	}
	if got := pos(t, b, "N"); got != [2]int{1, 0} {
		t.Errorf("N at %v", got)
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

	// Picking a node above connects it downward to the source...
	if added, err := b.ToggleEdge("A", "B"); err != nil || !added || !b.HasEdge("B", "A") {
		t.Fatalf("upward pick: added=%v err=%v edges=%v", added, err, b.Edges)
	}
	// ...and picking it again from either end disconnects.
	if added, err := b.ToggleEdge("A", "B"); err != nil || added || len(b.Edges) != 0 {
		t.Errorf("disconnect from below: added=%v err=%v edges=%v", added, err, b.Edges)
	}
	b.ToggleEdge("B", "A")
	if added, _ := b.ToggleEdge("A", "B"); added || len(b.Edges) != 0 {
		t.Errorf("disconnect of B→A picked from A: edges=%v", b.Edges)
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

func TestMoveGroupKeepsShape(t *testing.T) {
	b := &Board{
		Nodes: []Node{node("A", 0, 0), node("B", 1, 0), node("X", 0, 1), node("Y", 3, 0)},
		Edges: []Edge{{"A", "B"}, {"B", "Y"}},
	}
	// X blocks the right step, so the pair hops two columns together.
	if !b.MoveGroup([]string{"A", "B"}, Right) {
		t.Fatal("move right failed")
	}
	if pos(t, b, "A") != [2]int{0, 2} || pos(t, b, "B") != [2]int{1, 2} {
		t.Errorf("A %v B %v", pos(t, b, "A"), pos(t, b, "B"))
	}
	// One step down is fine; a second would put B level with its successor Y.
	if !b.MoveGroup([]string{"A", "B"}, Down) {
		t.Fatal("move down failed")
	}
	if b.MoveGroup([]string{"A", "B"}, Down) {
		t.Errorf("B moved onto Y's row: %v", pos(t, b, "B"))
	}
}

func TestDeleteRowPullsRowsUp(t *testing.T) {
	b := &Board{
		Nodes: []Node{node("A", 0, 0), node("B", 1, 0), node("C", 3, 1)},
		Edges: []Edge{{"A", "B"}, {"B", "C"}},
	}
	b.DeleteRow(1)
	if b.Node("B") != nil {
		t.Error("B should be deleted with its row")
	}
	if got := pos(t, b, "C"); got != [2]int{2, 1} {
		t.Errorf("C at %v", got)
	}
	if !b.HasEdge("A", "C") {
		t.Errorf("A → B → C should bridge to A → C: %v", b.Edges)
	}
	b.DeleteRow(1)
	if got := pos(t, b, "C"); got != [2]int{1, 1} {
		t.Errorf("empty row: C at %v", got)
	}
}

func TestInsertBeforeUsesFreeCellAbove(t *testing.T) {
	b := &Board{
		Nodes: []Node{node("A", 0, 0), node("B", 2, 0), node("Z", 3, 4)},
		Edges: []Edge{{"A", "B"}},
	}
	if err := b.InsertBefore("B", node("N", 0, 0)); err != nil {
		t.Fatal(err)
	}
	if got := pos(t, b, "N"); got != [2]int{1, 0} {
		t.Errorf("N at %v", got)
	}
	if pos(t, b, "B") != [2]int{2, 0} || pos(t, b, "Z") != [2]int{3, 4} {
		t.Errorf("nothing else should move: B %v Z %v", pos(t, b, "B"), pos(t, b, "Z"))
	}
	if !b.HasEdge("A", "N") || !b.HasEdge("N", "B") {
		t.Errorf("edges %v", b.Edges)
	}

	// No room above: refuse and leave the board untouched.
	cases := []struct {
		b    *Board
		want error
	}{
		{&Board{Nodes: []Node{node("B", 0, 0)}}, ErrTopRow},
		{&Board{Nodes: []Node{node("X", 1, 0), node("B", 2, 0)}}, ErrAboveTaken},
		{&Board{Nodes: []Node{node("P", 1, 3), node("B", 2, 0)}, Edges: []Edge{{"P", "B"}}}, ErrPredAbove},
	}
	for _, c := range cases {
		before := len(c.b.Nodes)
		if err := c.b.InsertBefore("B", node("N", 0, 0)); err != c.want {
			t.Errorf("err = %v, want %v", err, c.want)
		}
		if len(c.b.Nodes) != before {
			t.Errorf("board changed on error")
		}
	}
}
