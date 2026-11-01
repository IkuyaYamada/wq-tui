package board

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFocusKeepsOrderAndFollowsNodes(t *testing.T) {
	b := &Board{Nodes: []Node{node("A", 0, 0), node("B", 0, 1), node("C", 1, 0)}, Edges: []Edge{{"A", "C"}}}
	if !b.ToggleFocus("C") || !b.ToggleFocus("A") || !b.ToggleFocus("B") {
		t.Fatal("adding should report in")
	}
	if b.ToggleFocus("A") {
		t.Fatal("toggling again should take it out")
	}
	if want := []string{"C", "B"}; !reflect.DeepEqual(b.Focused(), want) {
		t.Errorf("focus %v, want %v in the order added", b.Focused(), want)
	}

	// A done node stays; a deleted one leaves.
	b.Node("C").Done = true
	_ = b.Delete("B")
	if want := []string{"C"}; !reflect.DeepEqual(b.Focus, want) {
		t.Errorf("after delete %v, want %v", b.Focus, want)
	}

	// Split into a frame, the work goes on in the child.
	if err := b.Decompose("C", node("C1", 0, 0)); err != nil {
		t.Fatal(err)
	}
	if want := []string{"C1"}; !reflect.DeepEqual(b.Focused(), want) {
		t.Errorf("after split %v, want %v", b.Focused(), want)
	}

	// It survives board.json, and a clone does not share it.
	data, _ := json.Marshal(b)
	var back Board
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.Focus, b.Focus) {
		t.Errorf("round trip %v, want %v", back.Focus, b.Focus)
	}
	c := b.Clone()
	c.ToggleFocus("A")
	if len(b.Focus) != 1 {
		t.Errorf("clone shares focus: %v", b.Focus)
	}
}
