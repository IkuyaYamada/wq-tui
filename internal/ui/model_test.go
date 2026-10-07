package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/thread"
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
	m = press(t, m, "g", "g", "c", "j", "j", "l", "<enter>")
	if !hasEdge(m.b, "設計", "雑務") {
		t.Fatalf("connect failed: %s / edges %v", m.msg, m.b.Edges)
	}

	// Move 雑務 one column right; the edge keeps pointing down.
	m = press(t, m, "m", "l", "<enter>")
	if got := titles(m.b)["雑務"]; got != [2]int{2, 2} {
		t.Errorf("雑務 at %v after move", got)
	}

	m = press(t, m, "<space>", "<enter>")
	if n := m.selected(); !n.Done {
		t.Errorf("space did not complete %s", n.Title)
	}
	entries, _ := thread.List(board.NodeDir(dir, m.selected().ID))
	if len(entries) != 1 || entries[0].Summary() != "Completed" {
		t.Errorf("thread missing completion entry: %+v", entries)
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

func TestVisualSelectMovesNodesTogether(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	// 設計 → 実装 in column 0, 雑務 at (0,1).
	m = press(t, m, "a", "設計", "<enter>", "o", "実装", "<enter>", "g", "g", "l", "a", "雑務", "<enter>")

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
	m = press(t, m, "g", "g", "V", "j", "m", "j", "<enter>")
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
	m = press(t, m, "g", "g", "V", "d")
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

func TestIMEBackToASCIIOnFocusAndWideKeys(t *testing.T) {
	f := &fakeIME{current: "Japanese"}
	m := New(t.TempDir(), &board.Board{}, WithIME(f))

	// Another app turned the input method on; coming back switches it off.
	f.current = "Japanese"
	next, _ := m.Update(tea.FocusMsg{})
	m = next.(Model)
	if f.current != "ABC" {
		t.Errorf("focus: got %s", f.current)
	}

	// A full-width key on the board switches too, and still acts.
	f.current = "Japanese"
	m = press(t, m, "ａ")
	if f.current != "Japanese" || m.mode != modeInput {
		t.Fatalf("ａ should open the title prompt in the input method: %s %v", f.current, m.mode)
	}
	// While typing a title, neither focus nor wide keys switch.
	m = press(t, m, "全角")
	next, _ = m.Update(tea.FocusMsg{})
	m = next.(Model)
	if f.current != "Japanese" {
		t.Errorf("title prompt should keep the input method: %s", f.current)
	}
	m = press(t, m, "<enter>")
	f.current = "Japanese"
	m = press(t, m, "ｊ")
	if f.current != "ABC" {
		t.Errorf("wide key on the board: got %s", f.current)
	}
}

func TestFullWidthKeysWorkOnBoard(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	m = press(t, m, "ａ", "全角", "<enter>", "ｏ", "ｊｋ", "<enter>")
	if got := titles(m.b); got["全角"] != [2]int{0, 0} || got["ｊｋ"] != [2]int{1, 0} {
		t.Errorf("full-width a/o not recognised, or title was converted: %v", got)
	}
	m = press(t, m, "ｋ", "　", "<enter>")
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

func TestCompletingAsksForAComment(t *testing.T) {
	dir := t.TempDir()
	f := &fakeIME{current: "Japanese"}
	m := New(dir, &board.Board{}, WithIME(f))
	m = press(t, m, "a", "設計", "<enter>")
	id := m.selected().ID

	// Space opens the prompt in Japanese; Esc leaves the node open.
	m = press(t, m, "<space>")
	if m.mode != modeInput || f.current != "Japanese" || m.selected().Done {
		t.Fatalf("mode %v ime %s done %v", m.mode, f.current, m.selected().Done)
	}
	m = press(t, m, "<esc>")
	if m.selected().Done || f.current != "ABC" {
		t.Fatalf("esc should cancel: done %v ime %s", m.selected().Done, f.current)
	}

	m = press(t, m, "<space>", "スキーマ確定", "<enter>")
	if !m.selected().Done {
		t.Fatal("enter should complete")
	}
	entries, _ := thread.List(board.NodeDir(dir, id))
	if len(entries) != 1 || entries[0].Summary() != "Completed: スキーマ確定" {
		t.Errorf("entries %+v", entries)
	}

	// Reopening does not ask.
	m = press(t, m, "<space>")
	if m.mode != modeNormal || m.selected().Done {
		t.Errorf("reopen: mode %v done %v", m.mode, m.selected().Done)
	}
	if m.input.Prompt != "done> " {
		t.Errorf("prompt was %q", m.input.Prompt)
	}
	m = press(t, m, "i")
	if m.input.Prompt != "title> " {
		t.Errorf("rename prompt %q", m.input.Prompt)
	}
}

func TestIMESwitchesAgainAfterTheComment(t *testing.T) {
	f := &fakeIME{current: "Japanese"}
	m := New(t.TempDir(), &board.Board{}, WithIME(f))
	m = press(t, m, "a", "設計", "<enter>", "<space>", "完了", "<enter>")
	if f.current != "ABC" {
		t.Fatalf("after the comment: %s", f.current)
	}

	// The IME takes over again while finishing the conversion...
	f.current = "Japanese"
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if cmd != nil {
		t.Errorf("a plain board key should not schedule another switch")
	}
	m = next.(Model)

	// ...and the delayed second switch brings ASCII back.
	next, _ = m.Update(asciiAgainMsg{})
	m = next.(Model)
	if f.current != "ABC" {
		t.Errorf("second switch: %s", f.current)
	}

	// While a prompt is open the delayed switch must not fire.
	m = press(t, m, "i")
	next, _ = m.Update(asciiAgainMsg{})
	if f.current != "Japanese" {
		t.Errorf("delayed switch fired during input: %s", f.current)
	}
}

func TestLeavingInputSchedulesSecondSwitch(t *testing.T) {
	m := New(t.TempDir(), &board.Board{}, WithIME(&fakeIME{current: "Japanese"}))
	m = press(t, m, "a", "設計")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("leaving the prompt should schedule the second switch")
	}
}

func TestVisualEqualsOrganizes(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	// A at (0,0); B dropped far away at (3,4), then connected A → B.
	m = press(t, m, "a", "A", "<enter>", "j", "j", "j", "l", "l", "l", "l", "a", "B", "<enter>")
	m = press(t, m, "g", "g", "c", "j", "j", "j", "l", "l", "l", "l", "<enter>")
	if !hasEdge(m.b, "A", "B") {
		t.Fatalf("setup: %v", m.b.Edges)
	}
	m = press(t, m, "g", "g", "V", "j", "j", "j", "=")
	if got := titles(m.b)["B"]; got != [2]int{1, 0} {
		t.Errorf("B at %v after =", got)
	}
	if m.mode != modeNormal || !strings.HasPrefix(m.msg, "organized") {
		t.Errorf("mode %v msg %q", m.mode, m.msg)
	}
	m = press(t, m, "u")
	if got := titles(m.b)["B"]; got != [2]int{3, 4} {
		t.Errorf("undo: B at %v", got)
	}
}

func TestStartsOnFirstOpenNode(t *testing.T) {
	n := func(id string, row, col int, done bool) board.Node {
		return board.Node{ID: id, Title: id, Row: row, Col: col, Done: done}
	}
	b := &board.Board{Nodes: []board.Node{
		n("top", 0, 0, true), n("done2", 1, 0, true),
		n("right", 2, 4, false), n("left", 2, 1, false), n("later", 3, 0, false),
	}}
	m := New(t.TempDir(), b)
	if got := m.selected(); got == nil || got.ID != "left" {
		t.Errorf("cursor on %+v, want left", got)
	}

	for i := range b.Nodes {
		b.Nodes[i].Done = true
	}
	m = New(t.TempDir(), b)
	if m.row != 0 || m.col != 0 {
		t.Errorf("all done: cursor %d,%d", m.row, m.col)
	}
}

func TestGXOpensNodeURL(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	var opened []string
	m.openURL = func(u string) error { opened = append(opened, u); return nil }
	m = press(t, m, "a", "設計", "<enter>", "g", "x")
	if len(opened) != 0 || !strings.Contains(m.msg, "no url") {
		t.Errorf("without url: opened %v msg %q", opened, m.msg)
	}
	m.b.Nodes[0].URL = "https://example.com/doc"
	m = press(t, m, "g", "x")
	if len(opened) != 1 || opened[0] != "https://example.com/doc" {
		t.Errorf("opened %v", opened)
	}
	// gg still goes to the only (open) node; a lone g then another key does nothing.
	m = press(t, m, "j", "l", "g", "g")
	if m.row != 0 || m.col != 0 {
		t.Errorf("gg: %d,%d", m.row, m.col)
	}
	m = press(t, m, "g", "j")
	if m.row != 0 {
		t.Errorf("g then j should be swallowed: row %d", m.row)
	}
}

func TestURLEditedInVimIsAdopted(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	m = press(t, m, "a", "設計", "<enter>")
	n := *m.selected()
	strategy, err := strategyPath(dir, n)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(strategy); !strings.Contains(string(got), "\nurl:\n") {
		t.Errorf("blank url: field missing:\n%s", got)
	}
	os.WriteFile(strategy, []byte("---\ntitle: 設計\nurl: https://example.com\n---\n\n"), 0o644)
	next, _ := m.Update(editorDoneMsg{id: n.ID, strategy: strategy})
	m = next.(Model)
	if got := m.selected().URL; got != "https://example.com" {
		t.Fatalf("url %q", got)
	}
	m = press(t, m, "u")
	if got := m.selected().URL; got != "" {
		t.Errorf("undo should clear url, got %q", got)
	}
}

// writeElsewhere saves b the way a second wq would, nudging the mtime so the
// change is visible even on coarse-grained filesystems.
func writeElsewhere(t *testing.T, dir string, b *board.Board) {
	t.Helper()
	if err := board.Save(dir, b); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	os.Chtimes(filepath.Join(dir, "board.json"), later, later)
}

func TestStaleBoardRefusesEditsUntilReload(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	m = press(t, m, "a", "こちら", "<enter>")

	other, _ := board.Load(dir)
	other.Nodes = append(other.Nodes, board.Node{ID: "x", Title: "あちら", Row: 0, Col: 3})
	writeElsewhere(t, dir, other)

	// Looking around is fine; editing is refused and nothing is written.
	m = press(t, m, "l")
	m = press(t, m, "a")
	if m.mode != modeNormal || !m.stale || !strings.Contains(m.msg, "R to reload") {
		t.Fatalf("mode %v stale %v msg %q", m.mode, m.stale, m.msg)
	}
	if !strings.Contains(m.View(), "changed in another wq") {
		t.Error("header should flag the stale board")
	}
	if onDisk, _ := board.Load(dir); len(onDisk.Nodes) != 2 {
		t.Fatalf("other wq's node was overwritten: %d nodes", len(onDisk.Nodes))
	}

	m = press(t, m, "R")
	if m.stale || len(m.b.Nodes) != 2 || len(m.undo) != 0 {
		t.Fatalf("reload: stale %v nodes %d undo %d", m.stale, len(m.b.Nodes), len(m.undo))
	}
	m = press(t, m, "j", "a", "追加", "<enter>")
	if onDisk, _ := board.Load(dir); len(onDisk.Nodes) != 3 {
		t.Errorf("edit after reload not saved: %d nodes", len(onDisk.Nodes))
	}
}

func TestSaveRefusesWhenBoardChangedDuringInput(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	m = press(t, m, "a", "最初", "<enter>", "i")
	other, _ := board.Load(dir)
	other.Nodes[0].Title = "あちらで改名"
	writeElsewhere(t, dir, other)
	m = press(t, m, "こちらで改名", "<enter>")
	if onDisk, _ := board.Load(dir); onDisk.Nodes[0].Title != "あちらで改名" {
		t.Errorf("overwrote the other wq: %q", onDisk.Nodes[0].Title)
	}
	if !m.stale || !strings.HasPrefix(m.msg, "not saved") {
		t.Errorf("stale %v msg %q", m.stale, m.msg)
	}
}

func TestDoneNoteSetAndCleared(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	m = press(t, m, "a", "設計", "<enter>", "<space>", "スキーマ確定", "<enter>")
	if got := m.selected().DoneNote; got != "スキーマ確定" {
		t.Fatalf("note %q", got)
	}
	m = press(t, m, "<space>")
	if got := m.selected().DoneNote; got != "" {
		t.Errorf("reopen should clear the note, got %q", got)
	}
}

func TestBracketSpaceAddsEmptyRows(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	m = press(t, m, "a", "上", "<enter>", "o", "下", "<enter>")
	// [ Space on 下: empty row above it, cursor follows 下.
	m = press(t, m, "[", "<space>")
	if got := titles(m.b); got["上"] != [2]int{0, 0} || got["下"] != [2]int{2, 0} {
		t.Fatalf("after [ space: %v", got)
	}
	if n := m.selected(); n == nil || n.Title != "下" {
		t.Errorf("cursor left 下: %+v", n)
	}
	// ] Space on 上: empty row below it.
	m = press(t, m, "k", "k", "]", "<space>")
	if got := titles(m.b)["下"]; got != [2]int{3, 0} {
		t.Errorf("after ] space: 下 at %v", got)
	}
	// [ then another key does nothing.
	m = press(t, m, "[", "j")
	if got := titles(m.b)["下"]; got != [2]int{3, 0} || m.row != 0 {
		t.Errorf("[ j: 下 %v row %d", got, m.row)
	}
	m = press(t, m, "u", "u")
	if got := titles(m.b)["下"]; got != [2]int{1, 0} {
		t.Errorf("undo: %v", got)
	}
}

func TestOAndShiftOMakeRoomWhenBlocked(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	m = press(t, m, "a", "上", "<enter>", "o", "下", "<enter>")
	// o on 上: the cell below holds 下, so 下 moves down a row.
	m = press(t, m, "k", "o", "中", "<enter>")
	got := titles(m.b)
	if got["上"] != [2]int{0, 0} || got["中"] != [2]int{1, 0} || got["下"] != [2]int{2, 0} {
		t.Fatalf("o: %v", got)
	}
	if !hasEdge(m.b, "上", "中") || !hasEdge(m.b, "中", "下") {
		t.Errorf("o edges: %v", m.b.Edges)
	}
	// O on 上 (top row): everything moves down to make room.
	m = press(t, m, "k", "O", "最初", "<enter>")
	got = titles(m.b)
	if got["最初"] != [2]int{0, 0} || got["上"] != [2]int{1, 0} || got["下"] != [2]int{3, 0} {
		t.Errorf("O: %v", got)
	}
	m = press(t, m, "u")
	if got := titles(m.b); got["上"] != [2]int{0, 0} {
		t.Errorf("undo O: %v", got)
	}
}

func TestScrollKeys(t *testing.T) {
	b := &board.Board{}
	for r := 0; r < 30; r++ {
		b.Nodes = append(b.Nodes, board.Node{ID: fmt.Sprint(r), Title: fmt.Sprint(r), Row: r})
	}
	m := New(t.TempDir(), b)
	n, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 22}) // 20 body lines = 4 rows
	m = n.(Model)
	rowH := cardH + laneH

	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlD})
	if m.row != 2 || m.scroll != 2*rowH {
		t.Errorf("C-d: row %d scroll %d", m.row, m.scroll)
	}
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.row != 0 || m.scroll != 0 {
		t.Errorf("C-u: row %d scroll %d", m.row, m.scroll)
	}

	for i := 0; i < 10; i++ {
		m = press(t, m, "j")
	}
	m = press(t, m, "z", "t")
	if m.scroll != 10*rowH {
		t.Errorf("zt: scroll %d", m.scroll)
	}
	m = press(t, m, "z", "b")
	if want := 10*rowH + cardH + 1 - m.bodyHeight(); m.scroll != want {
		t.Errorf("zb: scroll %d want %d", m.scroll, want)
	}
	m = press(t, m, "z", "z")
	if want := 10*rowH + cardH/2 - m.bodyHeight()/2; m.scroll != want {
		t.Errorf("zz: scroll %d want %d", m.scroll, want)
	}
	if m.row != 10 {
		t.Errorf("z commands must not move the cursor: row %d", m.row)
	}
}

func pressKey(m Model, k tea.KeyMsg) Model {
	n, _ := m.Update(k)
	return n.(Model)
}

func TestDashTogglesSessionBreak(t *testing.T) {
	f := &fakeIME{current: "Japanese"}
	m := New(t.TempDir(), &board.Board{}, WithIME(f))
	n, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = n.(Model)
	m = press(t, m, "a", "設計", "<enter>", "o", "実装", "<enter>", "k")

	m = press(t, m, "-")
	if m.mode != modeInput || f.current != "Japanese" || m.input.Prompt != "break> " {
		t.Fatalf("mode %v ime %s prompt %q", m.mode, f.current, m.input.Prompt)
	}
	m = press(t, m, "今日はここまで", "<enter>")
	if br, ok := m.b.BreakAfter(0); !ok || br.Label != "今日はここまで" {
		t.Fatalf("break %+v %v", br, ok)
	}
	view := m.View()
	if !strings.Contains(view, "今日はここまで") || !strings.Contains(view, "┄") {
		t.Errorf("break not drawn:\n%s", view)
	}
	if got := titles(m.b)["実装"]; got != [2]int{1, 0} {
		t.Errorf("a break must not move nodes: %v", got)
	}

	// - again removes it; Esc on the prompt adds nothing.
	m = press(t, m, "-")
	if _, ok := m.b.BreakAfter(0); ok {
		t.Error("second - should remove the break")
	}
	m = press(t, m, "-", "<esc>")
	if len(m.b.Breaks) != 0 {
		t.Errorf("esc added a break: %+v", m.b.Breaks)
	}
	m = press(t, m, "u")
	if _, ok := m.b.BreakAfter(0); !ok {
		t.Error("undo should bring the break back")
	}
}

func TestCtrlECtrlYScrollWithoutMovingCursor(t *testing.T) {
	b := &board.Board{}
	for r := 0; r < 30; r++ {
		b.Nodes = append(b.Nodes, board.Node{ID: fmt.Sprint(r), Title: fmt.Sprint(r), Row: r})
	}
	m := New(t.TempDir(), b)
	n, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 22}) // 4 rows visible
	m = n.(Model)
	rowH := cardH + laneH
	m = press(t, m, "j", "j") // cursor on row 2, scroll 0

	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	if m.scroll != rowH || m.row != 2 {
		t.Errorf("C-e: scroll %d row %d", m.scroll, m.row)
	}
	// Two more: row 2 would scroll off the top, so the cursor moves down.
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	if m.scroll != 3*rowH || m.row != 3 {
		t.Errorf("C-e x3: scroll %d row %d", m.scroll, m.row)
	}
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlY})
	if m.scroll != 2*rowH || m.row != 3 {
		t.Errorf("C-y: scroll %d row %d", m.scroll, m.row)
	}
	// Scrolling up past the cursor pulls it back on screen from below.
	for i := 0; i < 2; i++ {
		m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlY})
	}
	if m.scroll != 0 || m.row != 3 {
		t.Errorf("C-y to top: scroll %d row %d", m.scroll, m.row)
	}
}

func TestMoveAndRelabelSessionBreak(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	n, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = n.(Model)
	m = press(t, m, "a", "A", "<enter>", "o", "B", "<enter>", "o", "C", "<enter>", "g", "g")
	m = press(t, m, "-", "昼", "<enter>")

	m = press(t, m, "j")
	if m = press(t, m, "M"); m.mode == modeMoveBreak || m.msg == "" {
		t.Fatal("M without a break under the row should refuse")
	}
	m = press(t, m, "k", "M", "j", "j")
	if m.mode != modeMoveBreak || m.row != 2 {
		t.Fatalf("mode %v row %d", m.mode, m.row)
	}
	m = press(t, m, "<enter>")
	if want := []board.Break{{After: 2, Label: "昼"}}; !reflect.DeepEqual(m.b.Breaks, want) {
		t.Fatalf("moved: %+v", m.b.Breaks)
	}
	if got := titles(m.b); got["B"] != [2]int{1, 0} || got["C"] != [2]int{2, 0} {
		t.Errorf("moving a break must not move nodes: %v", got)
	}

	// Esc puts it back where it was.
	m = press(t, m, "M", "k", "<esc>")
	if br, ok := m.b.BreakAfter(2); !ok || br.Label != "昼" || m.row != 2 {
		t.Fatalf("esc: %+v row %d", m.b.Breaks, m.row)
	}

	// i edits the label of the break being moved, wherever it lands.
	m = press(t, m, "M", "k", "i")
	if m.mode != modeInput || m.input.Value() != "昼" {
		t.Fatalf("mode %v value %q", m.mode, m.input.Value())
	}
	m.input.SetValue("今日はここまで")
	m = press(t, m, "<enter>")
	if want := []board.Break{{After: 1, Label: "今日はここまで"}}; !reflect.DeepEqual(m.b.Breaks, want) {
		t.Fatalf("relabel: %+v", m.b.Breaks)
	}

	// Undo takes back the label, then the move.
	m = press(t, m, "u")
	if want := []board.Break{{After: 1, Label: "昼"}}; !reflect.DeepEqual(m.b.Breaks, want) {
		t.Errorf("undo label: %+v", m.b.Breaks)
	}
	m = press(t, m, "u")
	if want := []board.Break{{After: 2, Label: "昼"}}; !reflect.DeepEqual(m.b.Breaks, want) {
		t.Errorf("undo move: %+v", m.b.Breaks)
	}
}

func TestGGGoesToFirstOpenNode(t *testing.T) {
	n := func(id string, row, col int, done bool) board.Node {
		return board.Node{ID: id, Title: id, Row: row, Col: col, Done: done}
	}
	b := &board.Board{Nodes: []board.Node{n("A", 0, 0, true), n("B", 1, 2, true), n("C", 1, 4, false), n("D", 2, 1, false)}}
	m := New(t.TempDir(), b)
	m = press(t, m, "G", "g", "g")
	if m.row != 1 || m.col != 4 {
		t.Errorf("gg: %d,%d, want C at 1,4", m.row, m.col)
	}
	for i := range m.b.Nodes {
		m.b.Nodes[i].Done = true
	}
	m = press(t, m, "g", "g")
	if m.row != 0 || m.col != 0 {
		t.Errorf("all done: %d,%d, want 0,0", m.row, m.col)
	}
}

func TestKPreviewsStrategyAndThread(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	n, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = n.(Model)
	m = press(t, m, "a", "設計", "<enter>", "l", "a", "実装", "<enter>", "h")
	id := m.selected().ID
	nodeDir := board.NodeDir(dir, id)
	os.MkdirAll(nodeDir, 0o755)
	os.WriteFile(filepath.Join(nodeDir, "strategy.md"), []byte("---\ntitle: 設計\n---\n\nスキーマから決める\n"), 0o644)
	at := time.Date(2026, 10, 1, 14, 3, 0, 0, time.Local)
	thread.Add(nodeDir, at.Add(time.Hour), "ログ見たら500多発\n")
	thread.Add(nodeDir, at, "クエリ流した\n")

	m = press(t, m, "K")
	view := m.View()
	t.Log("\n" + view)
	iS, iA, iB := strings.Index(view, "スキーマから決める"), strings.Index(view, "クエリ流した"), strings.Index(view, "ログ見たら500多発")
	if iS < 0 || iA < iS || iB < iA {
		t.Fatalf("want strategy, then entries oldest first: %d %d %d", iS, iA, iB)
	}
	if strings.Contains(view, "title: 設計") {
		t.Error("the header should be stripped")
	}

	// It follows the cursor, hides on empty cells and closes with Esc or K.
	m = press(t, m, "l")
	if view := m.View(); strings.Contains(view, "スキーマから決める") || !strings.Contains(view, "no strategy yet") {
		t.Errorf("preview should follow the cursor:\n%s", view)
	}
	m = press(t, m, "j")
	if strings.Contains(m.View(), "no strategy yet") {
		t.Error("no preview on an empty cell")
	}
	m = press(t, m, "k", "<esc>")
	if strings.Contains(m.View(), "no strategy yet") {
		t.Error("esc should close the preview")
	}
	m = press(t, m, "K", "K")
	if strings.Contains(m.View(), "no strategy yet") {
		t.Error("K again should close the preview")
	}
}

func TestBracketSpaceCarriesTheBreak(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	m = press(t, m, "a", "A", "<enter>", "o", "B", "<enter>", "k", "-", "<enter>")
	if _, ok := m.b.BreakAfter(0); !ok {
		t.Fatal("break missing")
	}
	// ] Space under A: the new row joins A's side, the break moves down.
	m = press(t, m, "]", "<space>")
	if _, ok := m.b.BreakAfter(1); !ok || titles(m.b)["B"] != [2]int{2, 0} {
		t.Errorf("] space: breaks %+v B %v", m.b.Breaks, titles(m.b)["B"])
	}
	// [ Space over B: the new row joins B's side, the break stays put.
	m = press(t, m, "j", "j", "[", "<space>")
	if _, ok := m.b.BreakAfter(1); !ok || titles(m.b)["B"] != [2]int{3, 0} {
		t.Errorf("[ space: breaks %+v B %v", m.b.Breaks, titles(m.b)["B"])
	}
	// ] Space on the last row still carries a break under it down.
	m = press(t, m, "-", "<enter>", "]", "<space>")
	if _, ok := m.b.BreakAfter(4); !ok {
		t.Errorf("] space on the last row: %+v", m.b.Breaks)
	}
}

func TestStartsWithFirstOpenNodeOnTheSecondRow(t *testing.T) {
	var nodes []board.Node
	for r := 0; r < 12; r++ {
		nodes = append(nodes, board.Node{ID: fmt.Sprint(r), Title: fmt.Sprint(r), Row: r, Done: r < 6})
	}
	b := &board.Board{Nodes: nodes}
	b.SetBreak(2, "")
	m := New(t.TempDir(), b)
	n, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = n.(Model)
	l := newLayout(m.b, m.width)
	if m.row != 6 || m.scroll != l.rowY[5] {
		t.Errorf("row %d scroll %d, want row 6 with row 5 (y=%d) at the top", m.row, m.scroll, l.rowY[5])
	}

	// The first row has nothing above it; a short board does not scroll.
	m = New(t.TempDir(), &board.Board{Nodes: []board.Node{{ID: "a", Title: "a"}}})
	n, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	if m = n.(Model); m.scroll != 0 {
		t.Errorf("scroll %d, want 0", m.scroll)
	}
}

func TestFrameNodesInVisualMode(t *testing.T) {
	n := func(id string, row, col int) board.Node { return board.Node{ID: id, Title: id, Row: row, Col: col} }
	b := &board.Board{
		Nodes: []board.Node{n("X", 0, 2), n("A", 1, 0), n("B", 1, 1), n("C", 2, 0), n("Y", 4, 3)},
		Edges: []board.Edge{{From: "X", To: "B"}, {From: "C", To: "Y"}, {From: "A", To: "C"}},
	}
	m := New(t.TempDir(), b)
	n2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = n2.(Model)
	m.row, m.col = 1, 0
	m = press(t, m, "v", "j", "l", "g", "設計", "<enter>")
	if len(m.b.Groups) != 1 || m.b.Groups[0].Title != "設計" || len(m.b.Groups[0].Members) != 3 || m.mode != modeNormal {
		t.Fatalf("groups %+v mode %v", m.b.Groups, m.mode)
	}
	view := m.View()
	t.Log("\n" + view)
	if strings.Count(view, "╭") < 1+len(m.b.Nodes)-1 || !strings.Contains(view, "╰") {
		t.Errorf("frame not drawn")
	}
	if len(m.routes) != 3 {
		t.Fatalf("routes %d", len(m.routes))
	}
	for _, r := range m.routes {
		if len(r.path) == 0 {
			t.Errorf("%s→%s not routed", r.from, r.to)
		}
	}

	// o inside the frame adds a member; the frame grows around it.
	m.row, m.col = 2, 0
	m = press(t, m, "o", "D", "<enter>")
	if m.b.GroupOf(m.selected().ID) != 0 {
		t.Errorf("o from a member should join its frame: %+v", m.b.Groups)
	}
	// u takes nodes out; an empty frame goes away. Undo brings it back.
	m.row, m.col = 1, 0
	m = press(t, m, "V", "j", "j", "u")
	if len(m.b.Groups) != 0 {
		t.Errorf("groups %+v", m.b.Groups)
	}
	m = press(t, m, "u")
	if len(m.b.Groups) != 1 {
		t.Errorf("undo: %+v", m.b.Groups)
	}

	// g on a selection reaching into the frame adds the rest to it, with
	// no title to ask for; one reaching into two frames is refused.
	m.row, m.col = 1, 1
	m = press(t, m, "v", "k", "l", "g")
	if g := m.b.Groups[0]; m.mode != modeNormal || g.Title != "設計" || m.b.GroupOf("X") != 0 || len(g.Members) != 5 {
		t.Fatalf("add to frame: mode %v %+v", m.mode, g)
	}
	m.row, m.col = 4, 3
	m = press(t, m, "v", "g", "別", "<enter>")
	m.row, m.col = 0, 0
	m = press(t, m, "V", "j", "j", "j", "j", "g")
	if len(m.b.Groups) != 2 || m.b.GroupOf("Y") != 1 || len(m.b.Groups[0].Members) != 5 || m.mode != modeNormal {
		t.Errorf("two frames: %+v", m.b.Groups)
	}
}

func TestFrameAsATarget(t *testing.T) {
	dir := t.TempDir()
	n := func(id string, row, col int) board.Node { return board.Node{ID: id, Title: id, Row: row, Col: col} }
	b := &board.Board{Nodes: []board.Node{n("X", 0, 0), n("A", 1, 0), n("B", 1, 1), n("C", 2, 0)}}
	m := New(dir, b)
	n2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = n2.(Model)
	m.row, m.col = 1, 0
	m = press(t, m, "v", "j", "l", "g", "<esc>")
	if len(m.b.Groups) != 0 {
		t.Fatalf("esc on the title should give up the frame: %+v", m.b.Groups)
	}
	m.row, m.col = 1, 0
	m = press(t, m, "v", "j", "l", "g", "設計", "<enter>")
	if len(m.b.Groups) != 1 {
		t.Fatalf("groups %+v", m.b.Groups)
	}
	id := m.b.Groups[0].ID

	// k from a member with the frame's line over it selects the frame; j
	// comes back, k k leaves upward.
	m.row, m.col = 2, 0
	m = press(t, m, "k")
	if m.frameSel != "" || m.row != 1 {
		t.Fatalf("k inside the frame moves as usual: sel %q row %d", m.frameSel, m.row)
	}
	m = press(t, m, "k")
	if m.frameSel != id {
		t.Fatalf("k under the frame's line selects the frame")
	}
	view := m.View()
	t.Log("\n" + view)
	if !strings.Contains(view, "┏") || !strings.Contains(view, "設計") || !strings.Contains(view, "0/3") || !strings.Contains(view, "FRAME") {
		t.Errorf("selected frame should be bold with title and progress")
	}
	m = press(t, m, "j")
	if m.frameSel != "" || m.row != 1 {
		t.Errorf("j: sel %q row %d", m.frameSel, m.row)
	}
	m = press(t, m, "k", "k")
	if m.frameSel != "" || m.row != 0 {
		t.Errorf("k k: sel %q row %d", m.frameSel, m.row)
	}

	// Space completes every member; again reopens them.
	m = press(t, m, "j", "k", "<space>")
	for _, id := range []string{"A", "B", "C"} {
		if !m.b.Node(id).Done {
			t.Errorf("%s not done", id)
		}
	}
	if !strings.Contains(m.View(), "3/3") {
		t.Error("progress should read 3/3")
	}
	m = press(t, m, "<space>")
	if m.b.Node("A").Done {
		t.Error("second space should reopen")
	}

	// c gives the frame the next color; u takes it back.
	color := m.b.Group(id).Color
	m = press(t, m, "c")
	if m.b.Group(id).Color != color%board.FrameColors+1 || m.frameSel != id {
		t.Errorf("c: color %d → %d, sel %q", color, m.b.Group(id).Color, m.frameSel)
	}
	m = press(t, m, "u") // lets the frame go, as keys not about frames do
	if m.b.Group(id).Color != color {
		t.Errorf("undo color: %d", m.b.Group(id).Color)
	}
	m = press(t, m, "k")

	// i renames, m moves the members together, x drops only the frame.
	m = press(t, m, "i")
	m.input.SetValue("実装")
	m = press(t, m, "<enter>")
	if g := m.b.Group(id); g == nil || g.Title != "実装" {
		t.Errorf("rename: %+v", g)
	}
	m = press(t, m, "m", "l", "<enter>")
	if got := titles(m.b); got["A"] != [2]int{1, 1} || got["B"] != [2]int{1, 2} || got["C"] != [2]int{2, 1} {
		t.Errorf("m should move the members together: %v", got)
	}
	m = press(t, m, "x")
	if len(m.b.Groups) != 0 || len(m.b.Nodes) != 4 {
		t.Errorf("x: groups %+v nodes %d", m.b.Groups, len(m.b.Nodes))
	}
}

func TestSplitNodeIntoFrame(t *testing.T) {
	dir := t.TempDir()
	b := &board.Board{
		Nodes: []board.Node{{ID: "X", Title: "X"}, {ID: "A", Title: "大きい", Row: 1}, {ID: "Y", Title: "Y", Row: 2}},
		Edges: []board.Edge{{From: "X", To: "A"}, {From: "A", To: "Y"}},
	}
	m := New(dir, b)
	m.row = 1
	m = press(t, m, "g", "s", "<esc>")
	if m.b.Node("A") == nil || len(m.b.Groups) != 0 {
		t.Fatalf("esc should undo the split: %+v", m.b.Groups)
	}
	m = press(t, m, "g", "s", "小1", "<enter>")
	g := m.b.Group("A")
	child := m.b.At(1, 0)
	if g == nil || g.Title != "大きい" || child == nil || child.Title != "小1" || g.Members[0] != child.ID {
		t.Fatalf("frame %+v child %+v", g, child)
	}
	if !m.b.HasEdge("X", child.ID) || !m.b.HasEdge(child.ID, "Y") {
		t.Errorf("edges should move to the child: %+v", m.b.Edges)
	}
	// o from the child adds a second member; K on the frame shows its notes.
	m = press(t, m, "o", "小2", "<enter>")
	if len(m.b.Group("A").Members) != 2 {
		t.Errorf("members %+v", m.b.Group("A").Members)
	}
	nodeDir := board.NodeDir(dir, "A")
	os.MkdirAll(nodeDir, 0o755)
	os.WriteFile(filepath.Join(nodeDir, "strategy.md"), []byte("---\ntitle: 大きい\n---\n\n全体の方針\n"), 0o644)
	m = press(t, m, "k", "k", "K")
	if m.frameSel != "A" || !strings.Contains(m.View(), "全体の方針") {
		t.Errorf("K on the frame should show its own strategy:\n%s", m.View())
	}
}

func TestHeaderNamesFrameAndNodeInFull(t *testing.T) {
	long := "とても長い名前の枠をここに書いておくための見出し"
	b := &board.Board{Nodes: []board.Node{{ID: "A", Title: "中身"}, {ID: "B", Title: "外", Col: 3}}}
	b.Frame(board.Group{ID: "F", Title: long}, []string{"A"})
	m := New(t.TempDir(), b)
	n, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = n.(Model)
	m.row, m.col = 0, 0
	header := strings.SplitN(m.View(), "\n", 2)[0]
	if !strings.Contains(header, long+" › 中身") {
		t.Errorf("header %q", header)
	}
	m = press(t, m, "k")
	if header := strings.SplitN(m.View(), "\n", 2)[0]; !strings.Contains(header, "▸ "+long+" 0/1") {
		t.Errorf("frame selected: %q", header)
	}
	m = press(t, m, "j", "l", "l", "l")
	if header := strings.SplitN(m.View(), "\n", 2)[0]; !strings.HasSuffix(strings.TrimSpace(header), "外") || strings.Contains(header, "›") {
		t.Errorf("outside a frame: %q", header)
	}
	// A narrow screen cuts the frame's name before the node's.
	n, _ = m.Update(tea.WindowSizeMsg{Width: 50, Height: 30})
	m = n.(Model)
	m.row, m.col = 0, 0
	if header := strings.SplitN(m.View(), "\n", 2)[0]; !strings.Contains(header, "… › 中身") {
		t.Errorf("narrow: %q", header)
	}
}

func TestYankAndPasteCopiesANode(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	n, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = n.(Model)
	m = press(t, m, "a", "設計", "<enter>")
	body := func(m Model) string { // the board alone: no breadcrumb, no footer message
		lines := strings.Split(m.View(), "\n")
		return strings.Join(lines[1:len(lines)-1], "\n")
	}
	src := m.selected()
	src.URL = "https://example.com/x"
	if err := writeStrategyBody(dir, *src, "まず読む"); err != nil {
		t.Fatal(err)
	}

	if m = press(t, m, "p"); len(m.b.Nodes) != 1 {
		t.Fatal("p with nothing yanked should do nothing")
	}

	// yy shows a ghost in the empty cell nearest the cursor; it follows
	// the cursor, and p pastes there at once.
	m = press(t, m, "y", "y")
	if v := body(m); strings.Count(v, "設計") != 2 {
		t.Fatalf("want the node and its ghost:\n%s", v)
	}
	m = press(t, m, "l", "l", "p")
	if m.mode != modeNormal || len(m.b.Nodes) != 2 {
		t.Fatalf("p should paste at once: mode %v, %d nodes", m.mode, len(m.b.Nodes))
	}
	cp := m.b.At(0, 2)
	if cp == nil || cp.Title != "設計" || cp.URL != src.URL || cp.ID == src.ID || cp.Done {
		t.Fatalf("copy %+v; board %v", cp, titles(m.b))
	}
	if got := strategyBody(dir, *cp); got != "まず読む" {
		t.Errorf("copy's strategy %q", got)
	}
	if m.row != 0 || m.col != 2 {
		t.Errorf("cursor at %d,%d, want on the copy", m.row, m.col)
	}

	// The yank stays held: p again lands next to the copy.
	if m = press(t, m, "p"); len(m.b.Nodes) != 3 || m.b.At(0, 3) == nil {
		t.Errorf("second p: %v", titles(m.b))
	}

	// One undo takes one paste back.
	if m = press(t, m, "u"); len(m.b.Nodes) != 2 {
		t.Errorf("undo left %d nodes", len(m.b.Nodes))
	}

	// Esc lets the yank go: no ghost, and p does nothing.
	m = press(t, m, "<esc>")
	if v := body(m); strings.Count(v, "設計") != 2 {
		t.Errorf("the ghost should be gone:\n%s", v)
	}
	if m = press(t, m, "p"); len(m.b.Nodes) != 2 {
		t.Errorf("p after esc pasted")
	}
}

func TestDeleteSessionBreak(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	m = press(t, m, "a", "A", "<enter>", "o", "B", "<enter>", "o", "C", "<enter>", "g", "g")
	m = press(t, m, "-", "昼", "<enter>")

	// M then x: gone after moving it, and one undo brings it back where it
	// was before M.
	m = press(t, m, "M", "j", "x")
	if m.mode != modeNormal || len(m.b.Breaks) != 0 {
		t.Fatalf("mode %v breaks %+v", m.mode, m.b.Breaks)
	}
	m = press(t, m, "u")
	if want := []board.Break{{After: 0, Label: "昼"}}; !reflect.DeepEqual(m.b.Breaks, want) {
		t.Errorf("undo: %+v", m.b.Breaks)
	}

	// - on the row above a break also removes it.
	m = press(t, m, "g", "g", "-")
	if len(m.b.Breaks) != 0 || m.mode != modeNormal {
		t.Errorf("- should remove: mode %v breaks %+v", m.mode, m.b.Breaks)
	}
}

func TestCtrlJKInVimOpensNeighbour(t *testing.T) {
	m := New(t.TempDir(), &board.Board{})
	m = press(t, m, "a", "一", "<enter>", "o", "二", "<enter>")
	second := *m.selected()
	m = press(t, m, "k")
	first := *m.selected()

	next, cmd := m.Update(editorDoneMsg{id: first.ID, step: 1})
	m = next.(Model)
	if m.selected().ID != second.ID || cmd == nil {
		t.Fatalf("C-j should move to and open %s, at %v cmd %v", second.Title, m.selected(), cmd != nil)
	}
	next, _ = m.Update(editorDoneMsg{id: second.ID, step: 1})
	m = next.(Model)
	if m.selected().ID != second.ID || m.msg != "no next node" {
		t.Errorf("past the last node should stay on the board: %q", m.msg)
	}
	next, cmd = m.Update(editorDoneMsg{id: second.ID, step: -1})
	m = next.(Model)
	if m.selected().ID != first.ID || cmd == nil {
		t.Errorf("C-k should move to and open %s", first.Title)
	}
}

func TestYPCopiesStrategyPath(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, &board.Board{})
	var copied []string
	m.copyText = func(s string) error { copied = append(copied, s); return nil }
	m = press(t, m, "a", "設計", "<enter>", "y", "p")
	want := filepath.Join(board.NodeDir(dir, m.b.Nodes[0].ID), "strategy.md")
	if len(copied) != 1 || copied[0] != want {
		t.Fatalf("copied %v, want %s", copied, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("yp should write strategy.md so the path opens: %v", err)
	}
	if m.yank != nil {
		t.Errorf("yp is not yy: nothing should be held for p")
	}
	// On an empty cell there is nothing to copy.
	m = press(t, m, "l", "y", "p")
	if len(copied) != 1 {
		t.Errorf("empty cell copied %v", copied)
	}
}
