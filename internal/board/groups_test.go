package board

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestGroups(t *testing.T) {
	b := &Board{Nodes: []Node{node("A", 0, 0), node("B", 0, 2), node("C", 1, 1), node("D", 3, 4)}}
	b.GroupNodes([]string{"A", "C", "nope"})
	b.GroupNodes([]string{"B", "C"}) // C moves over
	if want := []Group{{Color: 1, Members: []string{"A"}}, {Color: 2, Members: []string{"B", "C"}}}; !reflect.DeepEqual(b.Groups, want) {
		t.Fatalf("groups %+v", b.Groups)
	}
	if b.GroupAt(1, 1) != 1 || b.GroupAt(0, 1) != -1 {
		t.Errorf("GroupAt: a member's cell is its frame's, a cell beside it no frame's")
	}

	// Clones do not share members; deleting a node takes it out.
	c := b.Clone()
	c.Groups[1].Members[0] = "X"
	if b.Groups[1].Members[0] != "B" {
		t.Error("clone shares members")
	}
	b.Delete("A")
	if len(b.Groups) != 1 {
		t.Errorf("A's group should be gone with it: %+v", b.Groups)
	}
	b.JoinGroupOf("D", "C")
	b.Ungroup([]string{"B"})
	if want := []Group{{Color: 2, Members: []string{"C", "D"}}}; !reflect.DeepEqual(b.Groups, want) {
		t.Errorf("groups %+v", b.Groups)
	}

	data, _ := json.Marshal(b)
	var back Board
	if err := json.Unmarshal(data, &back); err != nil || !reflect.DeepEqual(back.Groups, b.Groups) {
		t.Errorf("round trip: %v %+v", err, back.Groups)
	}
}

func TestDecomposeHandsCellAndEdgesToTheFirstChild(t *testing.T) {
	b := &Board{
		Nodes: []Node{node("X", 0, 0), node("A", 1, 0), node("Y", 2, 0)},
		Edges: []Edge{{"X", "A"}, {"A", "Y"}},
	}
	b.Node("A").Title, b.Node("A").URL = "大きい", "https://example.com"
	if err := b.Decompose("A", node("C", 9, 9)); err != nil {
		t.Fatal(err)
	}
	if b.Node("A") != nil || b.Node("C").Row != 1 || b.Node("C").Col != 0 {
		t.Errorf("C should take A's cell: %+v", b.Nodes)
	}
	if want := []Edge{{"X", "C"}, {"C", "Y"}}; !reflect.DeepEqual(b.Edges, want) {
		t.Errorf("edges %+v", b.Edges)
	}
	g := b.Group("A")
	if g == nil || g.Title != "大きい" || g.URL != "https://example.com" || !reflect.DeepEqual(g.Members, []string{"C"}) {
		t.Fatalf("frame %+v", g)
	}
	if err := b.Decompose("C", node("D", 0, 0)); err != ErrNested {
		t.Errorf("a member cannot be decomposed: %v", err)
	}
	// Titles and ids survive members leaving.
	b.Frame(Group{ID: "F", Title: "枠"}, []string{"X", "Y"})
	b.Ungroup([]string{"X"})
	if g := b.Group("F"); g == nil || g.Title != "枠" || len(g.Members) != 1 {
		t.Errorf("frame F %+v", g)
	}
	b.RemoveGroup("F")
	if b.Group("F") != nil || b.GroupOf("Y") >= 0 {
		t.Error("RemoveGroup")
	}
}

func TestFrameColorsGoRound(t *testing.T) {
	b := &Board{}
	for i := range FrameColors + 1 {
		b.Nodes = append(b.Nodes, node(string(rune('A'+i)), i, 0))
		b.Frame(Group{ID: string(rune('a' + i))}, []string{string(rune('A' + i))})
	}
	for i, g := range b.Groups {
		if want := i%FrameColors + 1; g.Color != want {
			t.Errorf("frame %d color %d, want %d", i, g.Color, want)
		}
	}
	b.CycleColor("a")
	if b.Group("a").Color != 2 {
		t.Errorf("cycle: %d", b.Group("a").Color)
	}
	b.CycleColor("d")
	if b.Group("d").Color != 1 {
		t.Errorf("cycle wraps: %d", b.Group("d").Color)
	}

	// Frames from before colors get them in turn.
	old := &Board{Groups: []Group{{ID: "x"}, {ID: "y"}, {ID: "z", Color: 1}}}
	old.EnsureGroupColors()
	if c := []int{old.Groups[0].Color, old.Groups[1].Color, old.Groups[2].Color}; !reflect.DeepEqual(c, []int{2, 3, 1}) {
		t.Errorf("colors %v", c)
	}
}

func TestGroupAtWalledInCells(t *testing.T) {
	// A ring of members walls in its middle; a diagonal gap does not.
	b := &Board{Nodes: []Node{
		node("A", 0, 0), node("B", 0, 1), node("C", 0, 2),
		node("D", 1, 0), node("E", 1, 2),
		node("F", 2, 0), node("G", 2, 1), node("H", 2, 2),
		node("P", 0, 4), node("Q", 1, 3), node("R", 1, 5), node("S", 2, 4),
	}}
	b.Frame(Group{ID: "ring"}, []string{"A", "B", "C", "D", "E", "F", "G", "H"})
	b.Frame(Group{ID: "diag"}, []string{"P", "Q", "R", "S"})
	if b.GroupAt(1, 1) != 0 {
		t.Error("the ring's middle should be inside it")
	}
	if b.GroupAt(1, 4) != -1 {
		t.Error("members touching only at corners do not wall a cell in")
	}
	if b.GroupAt(3, 1) != -1 {
		t.Error("below the ring is outside")
	}
}

func TestAddToGroup(t *testing.T) {
	b := &Board{Nodes: []Node{node("A", 0, 0), node("B", 0, 1), node("C", 1, 0)}}
	b.Frame(Group{ID: "F"}, []string{"A"})
	b.Frame(Group{ID: "G"}, []string{"C"})
	if got := b.FramesOf([]string{"A", "B"}); !reflect.DeepEqual(got, []string{"F"}) {
		t.Errorf("FramesOf %v", got)
	}
	if n := b.AddToGroup("F", []string{"A", "B", "C", "nope"}); n != 2 {
		t.Errorf("added %d", n)
	}
	if want := []Group{{ID: "F", Color: 1, Members: []string{"A", "B", "C"}}}; !reflect.DeepEqual(b.Groups, want) {
		t.Errorf("groups %+v (G should be gone with C)", b.Groups)
	}
}
