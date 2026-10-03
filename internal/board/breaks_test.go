package board

import (
	"reflect"
	"testing"
)

func TestBreaksFollowRowChanges(t *testing.T) {
	b := &Board{Nodes: []Node{node("A", 0, 0), node("B", 1, 0), node("C", 2, 0)}}
	b.SetBreak(1, "休憩")  // between B and C
	b.SetBreak(0, "")    // between A and B
	b.SetBreak(1, "昼休み") // replaces the first

	want := []Break{{0, ""}, {1, "昼休み"}}
	if !reflect.DeepEqual(b.Breaks, want) {
		t.Fatalf("set: %+v", b.Breaks)
	}

	// Opening a row above B pushes both B and the break under it down; the
	// break under A stays put (the new empty row is below it).
	b.InsertRow(1)
	if want := []Break{{0, ""}, {2, "昼休み"}}; !reflect.DeepEqual(b.Breaks, want) {
		t.Errorf("insert row: %+v", b.Breaks)
	}

	// Deleting that empty row brings it back; deleting B's row folds its
	// break into the gap above, where one already exists: that one wins.
	b.DeleteRow(1)
	b.DeleteRow(1)
	if want := []Break{{0, ""}}; !reflect.DeepEqual(b.Breaks, want) {
		t.Errorf("delete rows: %+v", b.Breaks)
	}

	if _, ok := b.BreakAfter(0); !ok {
		t.Error("BreakAfter(0) missing")
	}
	b.RemoveBreak(0)
	if len(b.Breaks) != 0 {
		t.Errorf("remove: %+v", b.Breaks)
	}
}

func TestSameRowConnectKeepsBreakInPlace(t *testing.T) {
	b := &Board{Nodes: []Node{node("A", 0, 0), node("B", 0, 2), node("C", 1, 0)}}
	b.SetBreak(0, "")
	b.SetBreak(1, "end")
	b.ToggleEdge("A", "B") // B drops to row 1, C to row 2
	if want := []Break{{0, ""}, {2, "end"}}; !reflect.DeepEqual(b.Breaks, want) {
		t.Errorf("%+v", b.Breaks)
	}
}

func TestMoveBreakHopsOverTakenGaps(t *testing.T) {
	b := &Board{}
	b.SetBreak(0, "a")
	b.SetBreak(1, "b")

	if to, ok := b.MoveBreak(0, 1, 3); !ok || to != 2 {
		t.Fatalf("down: %d %v", to, ok)
	}
	if want := []Break{{1, "b"}, {2, "a"}}; !reflect.DeepEqual(b.Breaks, want) {
		t.Errorf("down: %+v", b.Breaks)
	}
	if _, ok := b.MoveBreak(1, -1, 3); !ok {
		t.Error("up to the top gap should work")
	}
	if _, ok := b.MoveBreak(0, -1, 3); ok {
		t.Error("nothing above the top gap")
	}
	if _, ok := b.MoveBreak(2, 1, 2); ok {
		t.Error("past the last row")
	}
	if _, ok := b.MoveBreak(5, 1, 9); ok {
		t.Error("no break there")
	}
}

func TestInsertsStayInTheirSession(t *testing.T) {
	// A | break | B, with a free cell under A and over B.
	b := &Board{Nodes: []Node{node("A", 0, 0), node("B", 1, 1)}}
	b.SetBreak(0, "昼")
	b.InsertAfter("A", node("N", 0, 0))
	if got := b.Node("N"); got.Row != 1 || got.Col != 0 {
		t.Errorf("o: N at %d,%d, want 1,0 (above the break)", got.Row, got.Col)
	}
	if want := []Break{{1, "昼"}}; !reflect.DeepEqual(b.Breaks, want) {
		t.Errorf("o: break should move down under N: %+v", b.Breaks)
	}
	if got := b.Node("B"); got.Row != 2 {
		t.Errorf("o: B row %d, want 2", got.Row)
	}

	// O on B: the free cell over B is across the break, so a row opens.
	b.InsertBefore("B", node("M", 0, 0))
	if got := b.Node("M"); got.Row != 2 || got.Col != 1 {
		t.Errorf("O: M at %d,%d, want 2,1 (below the break)", got.Row, got.Col)
	}
	if want := []Break{{1, "昼"}}; !reflect.DeepEqual(b.Breaks, want) {
		t.Errorf("O: break should stay above M: %+v", b.Breaks)
	}

	// Without a break the free cell is still used as is.
	c := &Board{Nodes: []Node{node("A", 0, 0), node("B", 2, 0)}}
	c.InsertAfter("A", node("N", 0, 0))
	if c.Node("B").Row != 2 || c.Node("N").Row != 1 {
		t.Errorf("free cell: %+v", c.Nodes)
	}
}
