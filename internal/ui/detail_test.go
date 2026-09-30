package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/thread"
)

func TestDetailScreenThread(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	n, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = n.(Model)
	m = press(t, m, "a", "設計", "<enter>")
	id := m.selected().ID
	nodeDir := board.NodeDir(dir, id)
	base := time.Date(2026, 10, 1, 14, 3, 0, 0, time.Local)
	thread.Add(nodeDir, base, "クエリ流した\n結果OK\n")
	thread.Add(nodeDir, base.Add(time.Hour), "ログ見たら500多発\n")

	m = press(t, m, "<enter>")
	if m.mode != modeDetail || len(m.entries) != 2 || m.entrySel != 1 {
		t.Fatalf("mode %v entries %d sel %d", m.mode, len(m.entries), m.entrySel)
	}
	view := m.View()
	for _, want := range []string{"設計", "10/01 15:03", "ログ見たら500多発", "strategy"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}

	// x trashes the selected entry, u brings it back selected.
	m = press(t, m, "k", "x")
	if len(m.entries) != 1 || m.entries[0].Summary() != "ログ見たら500多発" {
		t.Fatalf("after x: %+v", m.entries)
	}
	m = press(t, m, "u")
	if len(m.entries) != 2 || m.entrySel != 0 {
		t.Errorf("after u: %d entries, sel %d", len(m.entries), m.entrySel)
	}

	// A new entry left blank in vim is dropped; a written one is kept and selected.
	blank, _ := thread.Add(nodeDir, base.Add(2*time.Hour), "")
	n, _ = m.Update(editorDoneMsg{entry: blank.Path, isNew: true})
	m = n.(Model)
	if len(m.entries) != 2 {
		t.Errorf("blank entry kept: %d", len(m.entries))
	}
	written, _ := thread.Add(nodeDir, base.Add(3*time.Hour), "")
	os.WriteFile(written.Path, []byte("書いた\n"), 0o644)
	n, _ = m.Update(editorDoneMsg{entry: written.Path, isNew: true})
	m = n.(Model)
	if len(m.entries) != 3 || m.entries[m.entrySel].Summary() != "書いた" {
		t.Errorf("written entry: %d entries, selected %q", len(m.entries), m.entries[m.entrySel].Summary())
	}

	// Space completes the node and the log entry shows up selected.
	m = press(t, m, "<space>")
	if !m.detailNode().Done || m.entries[m.entrySel].Summary() != "Completed" {
		t.Errorf("space: done=%v selected %q", m.detailNode().Done, m.entries[m.entrySel].Summary())
	}

	m = press(t, m, "<esc>")
	if m.mode != modeNormal || m.selected() == nil || m.selected().ID != id {
		t.Errorf("esc should return to the board on the node")
	}
}

func TestWrapBreaksJapanese(t *testing.T) {
	got := wrap("あいうえおかきくけこ\n\nabc", 6)
	want := []string{"あいう", "えおか", "きくけ", "こ", "", "abc"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q", got)
	}
}
