package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

// press feeds keys the way a terminal would: named keys in <>, text as runes.
func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "<enter>":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "<esc>":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "<space>":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		case "<c-r>":
			msg = tea.KeyMsg{Type: tea.KeyCtrlR}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func titles(b *board.Board) map[string][2]int {
	out := map[string][2]int{}
	for _, n := range b.Nodes {
		out[n.Title] = [2]int{n.Row, n.Col}
	}
	return out
}

func TestRhythmicAddConnectComplete(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	m, _ = func() (Model, tea.Cmd) {
		n, c := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
		return n.(Model), c
	}()

	// a: first node; o, o: a chain below it; a: a sibling off to the side.
	m = press(t, m, "a", "設計", "<enter>", "o", "実装", "<enter>", "o", "リリース", "<enter>")
	m = press(t, m, "a", "雑務", "<enter>")
	got := titles(m.b)
	want := map[string][2]int{"設計": {0, 0}, "実装": {1, 0}, "リリース": {2, 0}, "雑務": {2, 1}}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s at %v, want %v", k, got[k], v)
		}
	}
	if len(m.b.Edges) != 2 {
		t.Errorf("edges %v", m.b.Edges)
	}

	// Esc on a fresh node's title discards the node entirely.
	m = press(t, m, "o", "<esc>")
	if len(m.b.Nodes) != 4 {
		t.Errorf("discarded node still present: %d nodes", len(m.b.Nodes))
	}

	// Connect 設計 → 雑務: select 設計, c, move to 雑務, Enter.
	m = press(t, m, "g", "c", "j", "j", "l", "<enter>")
	if !hasEdge(m.b, "設計", "雑務") {
		t.Fatalf("connect failed: %s / edges %v", m.msg, m.b.Edges)
	}

	// Move 雑務 one column right; the edge keeps pointing down.
	m = press(t, m, "m", "l", "<enter>")
	if got := titles(m.b)["雑務"]; got != [2]int{2, 2} {
		t.Errorf("雑務 at %v after move", got)
	}

	m = press(t, m, "<space>")
	if n := m.selected(); !n.Done {
		t.Errorf("space did not complete %s", n.Title)
	}
	thread, _ := os.ReadFile(filepath.Join(board.NodeDir(dir, m.selected().ID), "thread.md"))
	if !strings.Contains(string(thread), "Completed") {
		t.Errorf("thread missing completion log: %q", thread)
	}

	m = press(t, m, "u", "u")
	if got := titles(m.b)["雑務"]; got != [2]int{2, 1} || m.b.At(2, 1).Done {
		t.Errorf("undo x2 should restore position and done flag: %v", got)
	}
	m = press(t, m, "<c-r>")
	if got := titles(m.b)["雑務"]; got != [2]int{2, 2} {
		t.Errorf("redo: %v", got)
	}

	saved, err := board.Load(dir)
	if err != nil || len(saved.Nodes) != 4 || len(saved.Edges) != 3 {
		t.Errorf("board.json: %v nodes=%d edges=%d", err, len(saved.Nodes), len(saved.Edges))
	}
	t.Log("\n" + m.View())
}

func TestBufferRowsAcceptNewNodes(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})

	// An empty board still offers bufferRows rows to walk into.
	m = press(t, m, "j", "j", "j", "j", "l", "l")
	if m.row != bufferRows-1 || m.col != 2 {
		t.Fatalf("cursor at %d,%d", m.row, m.col)
	}
	m = press(t, m, "a", "深いところ", "<enter>")
	if got := titles(m.b)["深いところ"]; got != [2]int{2, 2} {
		t.Fatalf("added at %v", got)
	}

	// The buffer follows the lowest node down.
	m = press(t, m, "j", "j", "j", "j", "j")
	if m.row != 2+bufferRows {
		t.Errorf("cursor stopped at row %d", m.row)
	}
	m = press(t, m, "h", "n", "もっと下", "<enter>")
	if got := titles(m.b)["もっと下"]; got != [2]int{5, 1} {
		t.Errorf("added at %v", got)
	}

	// Deleting leaves the cursor on the now-empty cell.
	m = press(t, m, "x")
	if m.row != 5 || m.col != 1 || m.selected() != nil {
		t.Errorf("after delete cursor %d,%d on %v", m.row, m.col, m.selected())
	}

	// Connecting to an empty cell is refused without leaving connect mode.
	m = press(t, m, "k", "k", "k", "l", "c", "j", "<enter>")
	if m.mode != modeConnect || m.msg == "" {
		t.Errorf("mode=%v msg=%q", m.mode, m.msg)
	}
}

func hasEdge(b *board.Board, from, to string) bool {
	id := map[string]string{}
	for _, n := range b.Nodes {
		id[n.Title] = n.ID
	}
	return b.HasEdge(id[from], id[to])
}

func TestUnusedThreadStampIsDropped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thread.md")
	os.WriteFile(path, []byte("old note"), 0o644)
	add, err := appendThread(path, "## 2026-10-01 10:00\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := dropUnused(path, add); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "old note" {
		t.Errorf("got %q", got)
	}
}

func TestVisualSelectMovesNodesTogether(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	// 設計 → 実装 in column 0, 雑務 at (0,1).
	m = press(t, m, "a", "設計", "<enter>", "o", "実装", "<enter>", "g", "l", "a", "雑務", "<enter>")

	// Block-select column 0 only (rows 0-1), then slide it right: 雑務 is
	// in the way on row 0, so the pair hops to column 2.
	m = press(t, m, "h", "v", "j", "m", "l", "<enter>")
	got := titles(m.b)
	if got["設計"] != [2]int{0, 2} || got["実装"] != [2]int{1, 2} || got["雑務"] != [2]int{0, 1} {
		t.Fatalf("after block move: %v", got)
	}
	if m.row != 1 || m.col != 2 {
		t.Errorf("cursor should ride along, at %d,%d", m.row, m.col)
	}

	// V selects whole rows: push both rows down one.
	m = press(t, m, "g", "V", "j", "m", "j", "<enter>")
	got = titles(m.b)
	if got["設計"] != [2]int{1, 2} || got["雑務"] != [2]int{1, 1} || got["実装"] != [2]int{2, 2} {
		t.Fatalf("after row move: %v", got)
	}

	// One undo reverts the whole group move.
	m = press(t, m, "u")
	if got := titles(m.b); got["設計"] != [2]int{0, 2} || got["雑務"] != [2]int{0, 1} {
		t.Errorf("undo: %v", got)
	}

	// Visual d deletes every selected node.
	m = press(t, m, "g", "V", "d")
	if len(m.b.Nodes) != 1 {
		t.Errorf("visual delete left %d nodes", len(m.b.Nodes))
	}
}

func TestDisconnectFromLowerNode(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	m = press(t, m, "a", "上", "<enter>", "o", "下", "<enter>")
	if len(m.b.Edges) != 1 {
		t.Fatalf("edges %v", m.b.Edges)
	}
	// Cursor is on 下; connect mode, pick 上, Enter removes the edge.
	m = press(t, m, "c", "k", "<enter>")
	if len(m.b.Edges) != 0 || m.msg != "disconnected" {
		t.Errorf("edges %v msg %q", m.b.Edges, m.msg)
	}
	// The cursor followed to 上; connecting back down restores the edge.
	m = press(t, m, "c", "j", "<enter>")
	if !hasEdge(m.b, "上", "下") {
		t.Errorf("reconnect: %v", m.b.Edges)
	}
}

type fakeIME struct {
	current string
	log     []string
}

func (f *fakeIME) ASCII() string {
	f.log = append(f.log, "ascii")
	if f.current == "ABC" {
		return ""
	}
	prev := f.current
	f.current = "ABC"
	return prev
}

func (f *fakeIME) Select(id string) {
	f.log = append(f.log, "select "+id)
	if id != "" {
		f.current = id
	}
}

func TestIMESwitchesAroundTitleInput(t *testing.T) {
	f := &fakeIME{current: "Japanese"}
	m := New(t.TempDir(), &board.Board{}, WithIME(f))
	if f.current != "ABC" {
		t.Fatalf("board should start in ASCII, got %s", f.current)
	}
	m = press(t, m, "a")
	if f.current != "Japanese" {
		t.Errorf("title input should restore Japanese, got %s", f.current)
	}
	m = press(t, m, "設計", "<enter>")
	if f.current != "ABC" {
		t.Errorf("back on the board should be ASCII, got %s", f.current)
	}
	next, _ := m.Update(editorDoneMsg{})
	m = next.(Model)
	if f.current != "ABC" {
		t.Errorf("after vim should be ASCII, got %s", f.current)
	}
}

func TestFullWidthKeysWorkOnBoard(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	m = press(t, m, "ａ", "全角", "<enter>", "ｏ", "ｊｋ", "<enter>")
	if got := titles(m.b); got["全角"] != [2]int{0, 0} || got["ｊｋ"] != [2]int{1, 0} {
		t.Errorf("full-width a/o not recognised, or title was converted: %v", got)
	}
	m = press(t, m, "ｋ", "　")
	if n := m.selected(); n == nil || n.Title != "全角" || !n.Done {
		t.Errorf("full-width k / ideographic space not recognised: %+v", n)
	}
}

func TestDeleteRowWithDDAndD(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	m = press(t, m, "a", "上", "<enter>", "o", "中", "<enter>", "j", "j", "a", "下", "<enter>")
	// Rows: 上 0, 中 1, (2 empty), 下 3. dd on the empty row closes the gap.
	m = press(t, m, "k", "d", "d")
	if got := titles(m.b)["下"]; got != [2]int{2, 0} {
		t.Fatalf("dd on empty row: 下 at %v", got)
	}
	// D on 中's row deletes 中 with it.
	m = press(t, m, "k", "D")
	got := titles(m.b)
	if _, ok := got["中"]; ok || got["下"] != [2]int{1, 0} {
		t.Fatalf("D on occupied row: %v", got)
	}
	// A lone d followed by another key does nothing; x deletes one node.
	m = press(t, m, "d", "j")
	if len(m.b.Nodes) != 2 || m.row != 1 {
		t.Errorf("d then j: %d nodes, cursor row %d", len(m.b.Nodes), m.row)
	}
	m = press(t, m, "x")
	if len(m.b.Nodes) != 1 {
		t.Errorf("x: %d nodes", len(m.b.Nodes))
	}
	m = press(t, m, "u", "u")
	if _, ok := titles(m.b)["中"]; !ok {
		t.Errorf("undo should bring 中 back: %v", titles(m.b))
	}
}

func TestOReportsWhenThereIsNoRoomAbove(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	m = press(t, m, "a", "一番上", "<enter>", "O")
	if m.mode != modeNormal || len(m.b.Nodes) != 1 || m.msg == "" {
		t.Errorf("mode %v nodes %d msg %q", m.mode, len(m.b.Nodes), m.msg)
	}
	if len(m.undo) != 1 {
		t.Errorf("refused O should not leave an undo step: %d", len(m.undo))
	}
}
