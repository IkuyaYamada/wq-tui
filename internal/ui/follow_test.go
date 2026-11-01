package ui

import (
	"strings"
	"testing"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

func TestFollowWalksAlongEdges(t *testing.T) {
	n := func(id string, row, col int) board.Node { return board.Node{ID: id, Title: id, Row: row, Col: col} }
	b := &board.Board{
		Nodes: []board.Node{n("a", 0, 0), n("b", 1, 0), n("c", 1, 4), n("d", 6, 1)},
		Edges: []board.Edge{{From: "a", To: "c"}, {From: "a", To: "b"}, {From: "c", To: "d"}},
	}
	m := New(t.TempDir(), b)
	at := func(want string) {
		t.Helper()
		if got := m.selected(); got == nil || got.ID != want {
			t.Fatalf("cursor on %v, want %s", got, want)
		}
	}
	m = press(t, m, "f")
	if m.mode != modeFollow || !strings.Contains(m.View(), "FOLLOW") {
		t.Fatalf("f should enter FOLLOW: %v", m.mode)
	}
	m = press(t, m, "j")
	at("b") // the leftmost successor first
	if !strings.Contains(m.msg, "1/2") {
		t.Errorf("msg %q", m.msg)
	}
	m = press(t, m, "l")
	at("c")
	m = press(t, m, "l")
	at("b") // wraps
	m = press(t, m, "h", "j")
	at("d") // however far away
	m = press(t, m, "j")
	if at("d"); !strings.Contains(m.msg, "no edge") {
		t.Errorf("a dead end should say so: %q", m.msg)
	}
	m = press(t, m, "k")
	at("c")
	m = press(t, m, "<esc>")
	at("a") // back to the start
	if m.mode != modeNormal {
		t.Errorf("esc should leave FOLLOW: %v", m.mode)
	}

	m = press(t, m, "f", "j", "l", "<enter>")
	at("c") // enter stays
	if m.mode != modeNormal {
		t.Errorf("enter should leave FOLLOW: %v", m.mode)
	}
}
