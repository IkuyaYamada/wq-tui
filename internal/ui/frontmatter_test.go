package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

func TestSyncStrategyTitle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "strategy.md")
	cases := []struct{ name, before, title, want string }{
		{"new file", "", "設計", "---\ntitle: 設計\n---\n\n"},
		{"legacy heading", "# 設計\n\nまず調べる\n", "設計", "---\ntitle: 設計\n---\n\nまず調べる\n"},
		{"no heading", "メモだけ\n", "設計", "---\ntitle: 設計\n---\n\nメモだけ\n"},
		{"rename keeps body and extra keys", "---\ntitle: 旧\nowner: me\n---\n\n本文\n", "新", "---\ntitle: 新\nowner: me\n---\n\n本文\n"},
	}
	for _, c := range cases {
		os.Remove(path)
		if c.before != "" {
			os.WriteFile(path, []byte(c.before), 0o644)
		}
		if err := syncStrategyTitle(path, c.title); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if string(got) != c.want {
			t.Errorf("%s:\ngot  %q\nwant %q", c.name, got, c.want)
		}
		if title, ok := readStrategyTitle(path); !ok || title != c.title {
			t.Errorf("%s: read back %q %v", c.name, title, ok)
		}
	}
}

func TestTitleEditedInVimRenamesNode(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	m = press(t, m, "a", "仮タイトル", "<enter>")
	n := *m.selected()
	strategy, thread, err := nodeFiles(dir, n)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate the vim session: the user rewrites the header line.
	os.WriteFile(strategy, []byte("---\ntitle: 本タイトル\n---\n\n方針\n"), 0o644)
	next, _ := m.Update(editorDoneMsg{id: n.ID, strategy: strategy, thread: thread})
	m = next.(Model)
	if got := m.selected().Title; got != "本タイトル" {
		t.Fatalf("title %q", got)
	}
	saved, _ := board.Load(dir)
	if saved.Nodes[0].Title != "本タイトル" {
		t.Errorf("board.json not updated: %q", saved.Nodes[0].Title)
	}

	m = press(t, m, "u")
	if got := m.selected().Title; got != "仮タイトル" {
		t.Errorf("undo: %q", got)
	}

	// Blanking the title leaves the node's title alone.
	os.WriteFile(strategy, []byte("---\ntitle:\n---\n"), 0o644)
	next, _ = m.Update(editorDoneMsg{id: n.ID, strategy: strategy, thread: thread})
	m = next.(Model)
	if got := m.selected().Title; got != "仮タイトル" {
		t.Errorf("blank title applied: %q", got)
	}
}
