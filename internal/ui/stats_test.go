package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

func TestStatsCountStrategyCharacters(t *testing.T) {
	dir := t.TempDir()
	nodeDir := board.NodeDir(dir, "a")
	os.MkdirAll(nodeDir, 0o755)
	os.WriteFile(filepath.Join(nodeDir, "strategy.md"), []byte("---\ntitle: 長いタイトル\n---\n\nスキーマ から\n決める\n"), 0o644)

	if got := readStats(dir, "a"); got.chars != 9 {
		t.Errorf("stats %+v, want 9 chars (header and spaces left out)", got)
	}
	if m := (nodeStats{}).meter(); m != 0 {
		t.Errorf("nothing written: %q", m)
	}
	if m := (nodeStats{chars: 1000}).meter(); m != '█' {
		t.Errorf("meter %q", m)
	}
}

func TestMetersOnCards(t *testing.T) {
	dir := t.TempDir()
	b := &board.Board{Nodes: []board.Node{{ID: "a", Title: "設計"}, {ID: "b", Title: "実装", Col: 1}}}
	nodeDir := board.NodeDir(dir, "a")
	os.MkdirAll(nodeDir, 0o755)
	os.WriteFile(filepath.Join(nodeDir, "strategy.md"), []byte(strings.Repeat("あ", 150)), 0o644)
	m := New(dir, b)
	n, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = n.(Model)
	view := m.View()
	t.Log("\n" + view)
	if !strings.Contains(view, "┗▄━") {
		t.Errorf("a's card should show a half-height strategy bar")
	}
	if !strings.Contains(view, "╰────") {
		t.Errorf("b's card has nothing written and no bars")
	}
	m = press(t, m, "K")
	if !strings.Contains(m.View(), "strategy 150字") || strings.Contains(m.View(), "thread") {
		t.Errorf("preview should give the counts:\n%s", m.View())
	}
}

func TestEditInPreview(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	n, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = n.(Model)
	m = press(t, m, "a", "設計", "<enter>")
	id := m.selected().ID
	nodeDir := board.NodeDir(dir, id)
	os.MkdirAll(nodeDir, 0o755)
	strategy := filepath.Join(nodeDir, "strategy.md")
	os.WriteFile(strategy, []byte("---\ntitle: 設計\nurl:\nowner: me\n---\n\nスキーマから決める\n"), 0o644)

	ctrlS := tea.KeyMsg{Type: tea.KeyCtrlS}
	m = press(t, m, "K", "i")
	if m.mode != modeEdit || m.edit.Value() != "スキーマから決める" {
		t.Fatalf("mode %v value %q", m.mode, m.edit.Value())
	}
	m = press(t, m, "★")
	if !strings.Contains(m.View(), "★スキーマから決める") {
		t.Errorf("editor not drawn:\n%s", m.View())
	}

	// ^c on a change warns first; a second ^c throws it away.
	ctrlC := tea.KeyMsg{Type: tea.KeyCtrlC}
	m = pressKey(m, ctrlC)
	if m.mode != modeEdit || !strings.Contains(m.msg, "unsaved") {
		t.Fatalf("first ^c should warn: mode %v msg %q", m.mode, m.msg)
	}
	m = pressKey(m, ctrlC)
	if b, _ := os.ReadFile(strategy); m.mode != modeNormal || strings.Contains(string(b), "★") {
		t.Fatalf("second ^c should discard: mode %v\n%s", m.mode, b)
	}

	// Esc saves the body and keeps the header, hand-added fields included.
	m = press(t, m, "i", "★", "<esc>")
	want := "---\ntitle: 設計\nurl:\nowner: me\n---\n\n★スキーマから決める\n"
	if b, _ := os.ReadFile(strategy); string(b) != want {
		t.Errorf("strategy:\n%q\nwant\n%q", b, want)
	}
	if m.mode != modeNormal || !m.preview || !strings.Contains(m.View(), "★スキーマから決める") {
		t.Errorf("back in the refreshed preview:\n%s", m.View())
	}

	// ^s saves too.
	m = press(t, m, "i", "☆")
	m = pressKey(m, ctrlS)
	if b, _ := os.ReadFile(strategy); !strings.Contains(string(b), "☆★スキーマから決める") || m.mode != modeNormal {
		t.Errorf("^s should save: mode %v\n%s", m.mode, b)
	}
}

// Soft-wrapped bullet lines in Japanese once made the editor's walk to the
// top (CursorUp until line 0) cycle forever on i, freezing wq.
func TestEditStartsAtTopOverWrappedJapanese(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	n, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = n.(Model)
	m = press(t, m, "a", "設計", "<enter>")
	nodeDir := board.NodeDir(dir, m.selected().ID)
	os.MkdirAll(nodeDir, 0o755)
	body := "memo\n- あいabcdeかきくけこさしすせそたちつて\n- あいうえおかきくけこさしすせそた\n"
	os.WriteFile(filepath.Join(nodeDir, "strategy.md"), []byte("---\ntitle: 設計\n---\n\n"+body), 0o644)

	done := make(chan Model, 1)
	go func() { done <- press(t, m, "K", "i", "★") }()
	select {
	case m = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("i hung")
	}
	if !strings.HasPrefix(m.edit.Value(), "★memo") {
		t.Errorf("cursor should start at the top: %q", m.edit.Value())
	}
}

func TestEditBoxKeepsThePreviewsHeight(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	n, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = n.(Model)
	m = press(t, m, "a", "設計", "<enter>")
	nodeDir := board.NodeDir(dir, m.selected().ID)
	os.MkdirAll(nodeDir, 0o755)
	// Few line breaks, but long lines that wrap into many rows.
	body := strings.Repeat(strings.Repeat("長い説明の文です。", 12)+"\n", 4)
	os.WriteFile(filepath.Join(nodeDir, "strategy.md"), []byte("---\ntitle: 設計\n---\n\n"+body), 0o644)

	m = press(t, m, "K")
	before, _ := m.previewBox()
	m = press(t, m, "i")
	after, _ := m.editBox()
	if after.h < before.h {
		t.Errorf("editor box %d lines, preview was %d", after.h, before.h)
	}
	if after.h-2 < len(wrapLines([]pline{{strings.Repeat("長い説明の文です。", 12), stPlain}}, after.w-4))*4 {
		t.Errorf("editor box %d lines cannot show the wrapped text", after.h)
	}
}

func TestCtrlSStillSavesTheEditor(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	n, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = n.(Model)
	m = press(t, m, "a", "設計", "<enter>", "K", "i", "メモ")
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if got := strategyBody(dir, *m.selected()); m.mode != modeNormal || got != "メモ" {
		t.Errorf("mode %v strategy %q", m.mode, got)
	}
}

func TestCountChecks(t *testing.T) {
	body := "方針\n- [ ] 読む\n- [x] 書く\n  * [X] 入れ子\n1. [ ] 番号付き\n-[ ] 空白なし\n```\n- [x] コードの中\n```\n- 普通の項目\n"
	if c, d := countChecks(body); c != 4 || d != 2 {
		t.Errorf("countChecks = %d/%d, want 2 ticked of 4", d, c)
	}
	if p := (nodeStats{}).progress(); p != "" {
		t.Errorf("no task list should give no label: %q", p)
	}
}

func TestChecksOnCards(t *testing.T) {
	dir := t.TempDir()
	b := &board.Board{Nodes: []board.Node{{ID: "a", Title: "設計"}, {ID: "b", Title: "実装", Col: 1, URL: "https://example.com"}, {ID: "c", Title: "確認", Col: 2}}}
	for id, body := range map[string]string{"a": "- [x] 読む\n- [ ] 書く\n", "b": "- [x] 読む\n"} {
		nodeDir := board.NodeDir(dir, id)
		os.MkdirAll(nodeDir, 0o755)
		os.WriteFile(filepath.Join(nodeDir, "strategy.md"), []byte("---\ntitle: x\n---\n\n"+body), 0o644)
	}
	m := New(dir, b)
	n, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = n.(Model)
	view := m.View()
	t.Log("\n" + view)
	if !strings.Contains(view, " 1/2 ━┓") {
		t.Errorf("a's card (under the cursor) should show 1/2 at its top right")
	}
	if !strings.Contains(view, " 1/1 ↗─╮") {
		t.Errorf("b's card should show 1/1 before its link mark")
	}
	if !strings.Contains(view, "↗─╮  ╭────────────────╮") {
		t.Errorf("c has no task list and no label")
	}
}
