package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/thread"
)

func TestStatsCountWritingNotLogs(t *testing.T) {
	dir := t.TempDir()
	nodeDir := board.NodeDir(dir, "a")
	os.MkdirAll(nodeDir, 0o755)
	os.WriteFile(filepath.Join(nodeDir, "strategy.md"), []byte("---\ntitle: 長いタイトル\n---\n\nスキーマ から\n決める\n"), 0o644)
	at := time.Date(2026, 10, 1, 14, 0, 0, 0, time.Local)
	thread.Add(nodeDir, at, "クエリ流した\n")
	thread.Add(nodeDir, at.Add(time.Minute), "Completed: スキーマ確定\n")
	thread.Add(nodeDir, at.Add(2*time.Minute), "Reopened\n")
	thread.Add(nodeDir, at.Add(3*time.Minute), "Completed: 見直し\n続きはここに\n")

	got := readStats(dir, "a")
	if got.chars != 9 || got.entries != 2 {
		t.Errorf("stats %+v, want 9 chars (header and spaces left out) and 2 entries", got)
	}
	if m := (nodeStats{}).meters(); m != [2]rune{0, 0} {
		t.Errorf("nothing written: %q", m)
	}
	if m := (nodeStats{chars: 1000, entries: 3}).meters(); m != [2]rune{'█', '▄'} {
		t.Errorf("meters %q", m)
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
	if !strings.Contains(m.View(), "strategy 150字 · thread 0件") {
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

	// a writes a new thread entry, several lines allowed.
	m = press(t, m, "a", "一行目", "<enter>", "二行目")
	m = pressKey(m, ctrlS)
	entries, _ := thread.List(nodeDir)
	if len(entries) != 1 || entries[0].Body != "一行目\n二行目\n" {
		t.Fatalf("entries %+v", entries)
	}
	if !strings.Contains(m.View(), "thread 1件") {
		t.Errorf("stats should be updated:\n%s", m.View())
	}
	// An empty entry is not written.
	m = press(t, m, "a")
	m = pressKey(m, ctrlS)
	if entries, _ := thread.List(nodeDir); len(entries) != 1 {
		t.Errorf("empty entry written: %d", len(entries))
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
