package board

import (
	"reflect"
	"sort"
	"testing"
)

func all(b *Board) []string {
	var ids []string
	for _, n := range b.Nodes {
		ids = append(ids, n.ID)
	}
	return ids
}

func edgeSet(b *Board) []Edge {
	es := append([]Edge(nil), b.Edges...)
	sort.Slice(es, func(i, j int) bool { return es[i].From+es[i].To < es[j].From+es[j].To })
	return es
}

func TestOrganizeAlignsChainAndClosesGaps(t *testing.T) {
	b := &Board{
		Nodes: []Node{node("A", 0, 1), node("B", 3, 4), node("C", 6, 0)},
		Edges: []Edge{{"A", "B"}, {"B", "C"}},
	}
	before := edgeSet(b)
	if err := b.Organize(all(b)); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string][2]int{"A": {0, 1}, "B": {1, 1}, "C": {2, 1}} {
		if got := pos(t, b, id); got != want {
			t.Errorf("%s at %v, want %v", id, got, want)
		}
	}
	if !reflect.DeepEqual(edgeSet(b), before) {
		t.Errorf("edges changed: %v", b.Edges)
	}
}

func TestOrganizeUncrossesEdges(t *testing.T) {
	// A → B and C → D cross: B sits under C and D under A.
	b := &Board{
		Nodes: []Node{node("A", 0, 0), node("C", 0, 2), node("D", 2, 0), node("B", 2, 2)},
		Edges: []Edge{{"A", "B"}, {"C", "D"}},
	}
	if err := b.Organize(all(b)); err != nil {
		t.Fatal(err)
	}
	if pos(t, b, "B") != [2]int{1, 0} || pos(t, b, "D") != [2]int{1, 2} {
		t.Errorf("B %v D %v", pos(t, b, "B"), pos(t, b, "D"))
	}
	if !b.HasEdge("A", "B") || !b.HasEdge("C", "D") || len(b.Edges) != 2 {
		t.Errorf("edges changed: %v", b.Edges)
	}
}

func TestOrganizeLeavesOthersAlone(t *testing.T) {
	b := &Board{
		Nodes: []Node{
			node("A", 0, 0), node("B", 4, 3), // selected chain
			node("Note", 1, 0), // selected but unlinked: stays, and blocks (1,0)
			node("Out", 2, 5),  // not selected
		},
		Edges: []Edge{{"A", "B"}},
	}
	if err := b.Organize([]string{"A", "B", "Note"}); err != nil {
		t.Fatal(err)
	}
	if pos(t, b, "Note") != [2]int{1, 0} || pos(t, b, "Out") != [2]int{2, 5} {
		t.Errorf("moved: Note %v Out %v", pos(t, b, "Note"), pos(t, b, "Out"))
	}
	if got := pos(t, b, "B"); got != [2]int{1, 1} {
		t.Errorf("B should take the free cell nearest under A: %v", got)
	}
}

func TestOrganizeRespectsOutsideNeighbours(t *testing.T) {
	// P (outside) feeds X. Organizing never lifts nodes above the
	// selection's top row, and lines X up under P's column.
	b := &Board{
		Nodes: []Node{node("P", 2, 0), node("X", 5, 3), node("Y", 7, 3)},
		Edges: []Edge{{"P", "X"}, {"X", "Y"}},
	}
	if err := b.Organize([]string{"X", "Y"}); err != nil {
		t.Fatal(err)
	}
	if pos(t, b, "X") != [2]int{5, 0} || pos(t, b, "Y") != [2]int{6, 0} {
		t.Errorf("X %v Y %v", pos(t, b, "X"), pos(t, b, "Y"))
	}
}

func TestOrganizeRefusesWhenARowOverflowsAboveAnOutsideNode(t *testing.T) {
	// Seven nodes compete for row 1. Y has the right-most barycenter, so it
	// is the one pushed to row 2 — level with S, its successor outside the
	// selection. Nothing may change.
	b := &Board{
		Nodes: []Node{
			node("W", 0, 0), node("X2", 0, 4), node("X", 0, 5),
			node("W2", 1, 0), node("Z1", 1, 1), node("Z2", 1, 2), node("Z3", 1, 3), node("Z4", 1, 4), node("Y", 1, 5),
			node("Z5", 2, 1), node("S", 2, 5),
		},
		Edges: []Edge{
			{"W", "W2"}, {"X", "Y"}, {"Y", "S"},
			{"X2", "Z1"}, {"X2", "Z2"}, {"X2", "Z3"}, {"X2", "Z4"}, {"X2", "Z5"},
		},
	}
	var ids []string
	for _, n := range b.Nodes {
		if n.ID != "S" {
			ids = append(ids, n.ID)
		}
	}
	snapshot := b.Clone()
	if err := b.Organize(ids); err != ErrNoRoom {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(b, snapshot) {
		t.Errorf("board changed on error")
	}
}
