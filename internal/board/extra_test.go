package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A board.json written by a newer wq keeps the fields this build does not
// know about through a load/edit/save round trip.
func TestUnknownFieldsSurviveSave(t *testing.T) {
	dir := t.TempDir()
	in := `{
  "schema": 3,
  "nodes": [
    {"id": "a", "title": "設計", "row": 0, "col": 0, "done": false,
     "created_at": "2026-10-03T10:00:00+09:00", "priority": "high", "tags": ["x", "y"]}
  ],
  "edges": []
}`
	os.WriteFile(filepath.Join(dir, "board.json"), []byte(in), 0o644)
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	b.Nodes[0].Title = "設計し直し"
	b.Add(Node{ID: "b", Title: "新規"}, 0, 0)
	if err := Save(dir, b); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(filepath.Join(dir, "board.json"))
	for _, want := range []string{`"schema": 3`, `"priority": "high"`, `"tags": [`, `"設計し直し"`, `"新規"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
	again, err := Load(dir)
	if err != nil || again.Nodes[0].Title != "設計し直し" || again.Node("b") == nil {
		t.Errorf("reload: %v %+v", err, again)
	}
}

func TestNoExtraKeepsPlainOutput(t *testing.T) {
	dir := t.TempDir()
	Save(dir, &Board{Nodes: []Node{{ID: "a", Title: "x"}}})
	out, _ := os.ReadFile(filepath.Join(dir, "board.json"))
	if strings.Index(string(out), `"id"`) > strings.Index(string(out), `"title"`) {
		t.Errorf("known fields should keep struct order without extras:\n%s", out)
	}
}
