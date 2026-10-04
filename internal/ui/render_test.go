package ui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

func plain(cv *canvas) string {
	var sb strings.Builder
	for y := 0; y < cv.h; y++ {
		for x := 0; x < cv.w; x++ {
			c := cv.cells[y*cv.w+x]
			if !c.cont {
				sb.WriteRune(c.r)
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func sample() *board.Board {
	n := func(id, title string, row, col int) board.Node {
		return board.Node{ID: id, Title: title, Row: row, Col: col}
	}
	e := func(from, to string) board.Edge { return board.Edge{From: from, To: to} }
	return &board.Board{
		Nodes: []board.Node{
			n("a", "要件を洗い出す", 0, 0), n("b", "DB調査", 0, 1), n("c", "環境構築", 0, 3),
			n("d", "スキーマ設計", 1, 0), n("e", "雑務", 1, 4),
			n("f", "API実装", 2, 1), n("g", "リリース", 3, 3),
		},
		Edges: []board.Edge{e("a", "d"), e("b", "d"), e("d", "f"), e("c", "g"), e("f", "g"), e("a", "g")},
	}
}

func TestRenderRoutesEveryEdge(t *testing.T) {
	b := sample()
	l := newLayout(b, 120)
	routes := routeEdges(b, l)
	if len(routes) != len(b.Edges) {
		t.Fatalf("routed %d of %d edges", len(routes), len(b.Edges))
	}
	for _, r := range routes {
		if len(r.path) == 0 {
			t.Errorf("%s→%s has no path", r.from, r.to)
		}
	}
	out := plain(renderBoard(b, l, routes, view{cursorRow: 1, cursorCol: 0, cursorSt: stBorderSel, lit: "d"}))
	t.Log("\n" + out)
	if got := strings.Count(out, "▼"); got < 4 {
		t.Errorf("expected arrowheads on every target, got %d", got)
	}
}

func TestRowsStayPutWhenEdgesChange(t *testing.T) {
	b := sample()
	before := newLayout(b, 120).rowY
	b.Edges = nil
	after := newLayout(b, 120).rowY
	for r := range before {
		if before[r] != after[r] {
			t.Fatalf("row %d moved from y=%d to y=%d", r, before[r], after[r])
		}
	}
}

func TestTitleRows(t *testing.T) {
	cases := []struct {
		title string
		want  []string
	}{
		{"設計", []string{"設計"}},
		{"既存テーブルの依存", []string{"既存テーブ", "ルの依存"}},
		{"既存テーブルの依存関係を全部洗い出す", []string{"既存テーブ", "ルの依存…"}},
	}
	for _, c := range cases {
		got := titleRows(c.title, 10)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%q: got %q want %q", c.title, got, c.want)
		}
	}
}

func TestSelectedDoneNodeStillLooksDone(t *testing.T) {
	b := &board.Board{Nodes: []board.Node{{ID: "a", Title: "済", Row: 0, Col: 0, Done: true}}}
	l := newLayout(b, 120)
	cv := renderBoard(b, l, nil, view{cursorRow: 0, cursorCol: 0, cursorSt: stBorderSel})
	x, y := l.colX(0)+2, l.rowY[0]+1
	if c := cv.cells[y*cv.w+x]; c.r != '✓' || c.st != stTitleDoneSel {
		t.Errorf("title cell %q style %v", c.r, c.st)
	}
	if c := cv.cells[l.rowY[0]*cv.w+l.colX(0)]; c.st != stBorderSel {
		t.Errorf("cursor border should still show, style %v", c.st)
	}
}

func TestDoneNoteOnSecondLine(t *testing.T) {
	b := &board.Board{Nodes: []board.Node{
		{ID: "a", Title: "既存テーブルの依存関係を洗う", Row: 0, Col: 0, Done: true, DoneNote: "スキーマ確定"},
		{ID: "b", Title: "未着手", Row: 1, Col: 0},
	}}
	l := newLayout(b, 120)
	before := l.rowY[1]
	cv := renderBoard(b, l, nil, view{cursorRow: -1})
	out := plain(cv)
	if !strings.Contains(out, "スキーマ確定") {
		t.Errorf("note missing:\n%s", out)
	}
	x, y := l.colX(0)+2, l.rowY[0]+2
	if c := cv.cells[y*cv.w+x]; c.st != stDoneNote {
		t.Errorf("note style %v", c.st)
	}
	if newLayout(b, 120).rowY[1] != before {
		t.Error("the note must not change the layout")
	}
}

func TestBreakGetsItsOwnLineEdgesOnlyCross(t *testing.T) {
	b := sample()
	b.SetBreak(0, "昼休み")
	b.SetBreak(1, "")
	l := newLayout(b, 120)
	if plainRow1 := newLayout(sample(), 120).rowY[1]; l.rowY[1] != plainRow1+1 {
		t.Errorf("a break should add one line: row 1 at %d, want %d", l.rowY[1], plainRow1+1)
	}
	routes := routeEdges(b, l)
	for _, r := range routes {
		if len(r.path) == 0 {
			t.Fatalf("%s→%s has no path", r.from, r.to)
		}
		for i := 1; i < len(r.path); i++ {
			if p, q := r.path[i-1], r.path[i]; p.y == q.y && l.breakY[p.y] {
				t.Errorf("%s→%s runs along the break line at y=%d", r.from, r.to, p.y)
			}
		}
	}
	cv := renderBoard(b, l, routes, view{cursorRow: -1})
	t.Log("\n" + plain(cv))
	for y := range l.breakY {
		for x := 0; x < cv.w; x++ {
			if r := cv.cells[y*cv.w+x].r; strings.ContainsRune("─┌┐└┘├┤┬┴┼", r) {
				t.Errorf("edge turns on the break line at (%d,%d): %c", x, y, r)
			}
		}
	}
	if !strings.Contains(plain(cv), "昼休み") {
		t.Error("label missing")
	}
}

func TestSetOverWideRuneKeepsLineWidth(t *testing.T) {
	for _, x := range []int{2, 3} { // left half, then right half of "シ"
		cv := newCanvas(10, 1)
		cv.text(0, 0, "マシン", stPlain)
		cv.set(x, 0, '│', stPlain)
		if got := runewidthOf(plain(cv)); got != 10 {
			t.Errorf("x=%d: line is %d cells wide, want 10: %q", x, got, plain(cv))
		}
	}
}

func runewidthOf(s string) int {
	return runewidth.StringWidth(strings.TrimSuffix(s, "\n"))
}

func TestFrameTitleDodgesEdges(t *testing.T) {
	n := func(id, title string, row, col int) board.Node {
		return board.Node{ID: id, Title: title, Row: row, Col: col}
	}
	b := &board.Board{
		Nodes: []board.Node{
			n("a", "前提A", 0, 0), n("b", "前提B", 0, 1),
			n("c", "設計書のレビューが終わっている", 1, 0),
		},
		Edges:  []board.Edge{{From: "a", To: "c"}, {From: "b", To: "c"}},
		Groups: []board.Group{{ID: "g", Title: "レビュー準備作業", Members: []string{"c"}}},
	}
	l := newLayout(b, 120)
	cv := renderBoard(b, l, routeEdges(b, l), view{cursorRow: -1})
	out := plain(cv)
	t.Log("\n" + out)
	for y := 0; y < cv.h; y++ {
		if w := runewidthOf(strings.Split(out, "\n")[y]); w != cv.w {
			t.Errorf("line %d is %d cells wide, want %d", y, w, cv.w)
		}
	}
	f := l.frames[0]
	lines := strings.Split(out, "\n")
	if !strings.Contains(lines[f.y0]+lines[f.y1], " レビュー準") {
		t.Errorf("frame title missing or cut by an edge")
	}
}
