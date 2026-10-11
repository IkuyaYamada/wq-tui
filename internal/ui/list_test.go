package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

func TestListShowsEveryWholeTitle(t *testing.T) {
	long := "既存テーブルの依存関係を全部洗い出して移行の順番を決める"
	b := &board.Board{
		Nodes: []board.Node{
			{ID: "a", Title: long, Row: 0, Col: 3},
			{ID: "b", Title: "設計", Row: 0, Col: 0, Done: true, DoneNote: "スキーマ確定"},
			{ID: "c", Title: "実装", Row: 2, Col: 1},
		},
		Groups: []board.Group{{ID: "g", Title: "移行", Members: []string{"a"}, Color: 1}},
	}
	b.SetBreak(1, "昼休み")
	m := New(t.TempDir(), b)
	n, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m = n.(Model)
	m = press(t, m, "L")
	if m.mode != modeList {
		t.Fatalf("L should open the list: %v", m.mode)
	}
	view := m.View()
	t.Log("\n" + view)
	for _, want := range []string{"決める", "スキーマ確定", "[移行]", "昼休み", "実装", "LIST"} {
		if !strings.Contains(view, want) {
			t.Errorf("list should show %q", want)
		}
	}
	body := view[strings.Index(view, "\n"):]
	if strings.Index(body, "設計") > strings.Index(body, "既存") {
		t.Error("reading order: 設計 (col 0) comes first")
	}
	m = press(t, m, "j", "j", "<enter>")
	if m.mode != modeNormal || m.selected() == nil || m.selected().ID != "c" {
		t.Errorf("enter should go back to the board on c: %v %v", m.mode, m.selected())
	}
}
