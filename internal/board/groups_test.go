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
	if want := []Group{{Members: []string{"A"}}, {Members: []string{"B", "C"}}}; !reflect.DeepEqual(b.Groups, want) {
		t.Fatalf("groups %+v", b.Groups)
	}
	if r0, c0, r1, c1, ok := b.Bounds(b.Groups[1]); !ok || [4]int{r0, c0, r1, c1} != [4]int{0, 1, 1, 2} {
		t.Errorf("bounds %d %d %d %d", r0, c0, r1, c1)
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
	if want := []Group{{Members: []string{"C", "D"}}}; !reflect.DeepEqual(b.Groups, want) {
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
