package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

func TestSyncStrategyTitle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "strategy.md")
	cases := []struct{ name, before, title, want string }{
		{"new file", "", "設計", "---\ntitle: 設計\nurl:\n---\n\n"},
		{"legacy heading", "# 設計\n\nまず調べる\n", "設計", "---\ntitle: 設計\nurl:\n---\n\nまず調べる\n"},
		{"no heading", "メモだけ\n", "設計", "---\ntitle: 設計\nurl:\n---\n\nメモだけ\n"},
		{"rename keeps body and extra keys", "---\ntitle: 旧\nowner: me\n---\n\n本文\n", "新", "---\ntitle: 新\nowner: me\nurl:\n---\n\n本文\n"},
	}
	for _, c := range cases {
		os.Remove(path)
		if c.before != "" {
			os.WriteFile(path, []byte(c.before), 0o644)
		}
		if err := syncStrategyHeader(path, c.title, "", nil); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if string(got) != c.want {
			t.Errorf("%s:\ngot  %q\nwant %q", c.name, got, c.want)
		}
		if title, _, _, ok := readStrategyHeader(path); !ok || title != c.title {
			t.Errorf("%s: read back %q %v", c.name, title, ok)
		}
	}
}

func TestTitleEditedInVimRenamesNode(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	m = press(t, m, "a", "仮タイトル", "<enter>")
	n := *m.selected()
	strategy, err := strategyPath(dir, n)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate the vim session: the user rewrites the header line.
	os.WriteFile(strategy, []byte("---\ntitle: 本タイトル\n---\n\n方針\n"), 0o644)
	next, _ := m.Update(editorDoneMsg{id: n.ID, strategy: strategy})
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
	next, _ = m.Update(editorDoneMsg{id: n.ID, strategy: strategy})
	m = next.(Model)
	if got := m.selected().Title; got != "仮タイトル" {
		t.Errorf("blank title applied: %q", got)
	}
}

func TestDoneCommentLivesInStrategy(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	m = press(t, m, "a", "設計", "<enter>")
	strategy, err := strategyPath(dir, *m.selected())
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(strategy, []byte("---\ntitle: 設計\nurl:\n---\n\n方針\n"), 0o644)

	m = press(t, m, "<space>", "初回", "<enter>")
	doc, _ := os.ReadFile(strategy)
	if !strings.Contains(string(doc), "url:\ndone: 初回\n---") || !strings.HasSuffix(string(doc), " — 初回\n") {
		t.Fatalf("after done:\n%s", doc)
	}

	// The comment is fixed in vim's header.
	fixed := strings.Replace(string(doc), "done: 初回", "done: 直した", 1)
	os.WriteFile(strategy, []byte(fixed), 0o644)
	next, _ := m.Update(editorDoneMsg{id: m.selected().ID, strategy: strategy})
	m = next.(Model)
	if got := m.selected().DoneNote; got != "直した" {
		t.Fatalf("note %q", got)
	}

	// Reopening drops the header line; the log keeps every completion.
	m = press(t, m, "<space>")
	doc, _ = os.ReadFile(strategy)
	if strings.Contains(string(doc), "done: ") {
		t.Errorf("reopened header still has done:\n%s", doc)
	}
	m = press(t, m, "<space>", "二回目", "<enter>")
	doc, _ = os.ReadFile(strategy)
	if strings.Count(string(doc), "✓ done ") != 2 || !strings.Contains(string(doc), "done: 二回目\n") {
		t.Errorf("after second done:\n%s", doc)
	}
}
