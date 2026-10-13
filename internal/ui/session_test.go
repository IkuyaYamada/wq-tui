package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

// fakeSessions points both agents' session stores at temp directories and
// returns writers for each.
func fakeSessions(t *testing.T) (codex, claude func(id, cwd string)) {
	t.Helper()
	root := t.TempDir()
	oldCodex, oldClaude := codexSessions, claudeSessions
	codexSessions, claudeSessions = filepath.Join(root, "codex"), filepath.Join(root, "claude")
	t.Cleanup(func() { codexSessions, claudeSessions = oldCodex, oldClaude })
	write := func(path, body string) {
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	codex = func(id, cwd string) {
		write(filepath.Join(codexSessions, "2026", "10", "12", "rollout-2026-10-12T10-00-00-"+id+".jsonl"),
			`{"type":"session_meta","payload":{"id":"`+id+`","cwd":"`+cwd+`"}}`+"\n"+`{"type":"event"}`+"\n")
	}
	claude = func(id, cwd string) {
		write(filepath.Join(claudeSessions, strings.ReplaceAll(cwd, "/", "-"), id+".jsonl"),
			`{"type":"summary"}`+"\n"+`{"type":"user","cwd":"`+cwd+`"}`+"\n")
	}
	return codex, claude
}

func TestFindSessionOfEitherAgent(t *testing.T) {
	codex, claude := fakeSessions(t)
	codex("01a1-c0de", "/src/app")
	claude("5e55-c1a0", "/src/api")
	if s, ok := findSession("01a1-c0de"); !ok || s.agent != "codex" || s.cwd != "/src/app" {
		t.Errorf("codex: %+v %v", s, ok)
	}
	if s, ok := findSession("5e55-c1a0"); !ok || s.agent != "claude" || s.cwd != "/src/api" {
		t.Errorf("claude: %+v %v", s, ok)
	}
	if _, ok := findSession("nope"); ok {
		t.Error("an unknown id should not be found")
	}
}

func TestSessionEditedInTheHeaderIsAdopted(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	m = press(t, m, "a", "設計", "<enter>")
	n := *m.selected()
	strategy, _ := strategyPath(dir, n)
	os.WriteFile(strategy, []byte("---\ntitle: 設計\nurl:\nsession: 01a1-beef\n---\n\n方針\n"), 0o644)
	next, _ := m.Update(editorDoneMsg{id: n.ID, strategy: strategy})
	m = next.(Model)
	if got := m.selected().Session; got != "01a1-beef" {
		t.Errorf("session %q", got)
	}
	if !strings.Contains(m.View(), "↻") {
		t.Error("the card should show it has a session")
	}
	m = press(t, m, "g", "r")
	if !strings.Contains(m.msg, "not found") {
		t.Errorf("gr on a session that is not on this machine: %q", m.msg)
	}
}

func TestGRCopiesTheResumeCommand(t *testing.T) {
	codex, claude := fakeSessions(t)
	codex("01a1-c0de", "/src/my app")
	claude("5e55-c1a0", "/src/api")
	b := &board.Board{Nodes: []board.Node{
		{ID: "A", Title: "設計", Row: 0, Col: 0, Session: "01a1-c0de"},
		{ID: "B", Title: "実装", Row: 0, Col: 1, Session: "5e55-c1a0"},
	}}
	m := New(t.TempDir(), b)
	var copied []string
	m.copyText = func(s string) error { copied = append(copied, s); return nil }
	m.row, m.col = 0, 0
	m = press(t, m, "g", "r", "l", "g", "r")
	want := []string{"cd '/src/my app' && codex resume 01a1-c0de", "cd /src/api && claude --resume 5e55-c1a0"}
	if strings.Join(copied, "\n") != strings.Join(want, "\n") {
		t.Errorf("copied %q, want %q", copied, want)
	}
	if m.mode != modeNormal || !strings.Contains(m.msg, "copied") {
		t.Errorf("mode %v msg %q", m.mode, m.msg)
	}
}
