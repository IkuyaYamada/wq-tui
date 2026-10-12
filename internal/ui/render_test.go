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

func TestBlockRowsGrowToShowWholeTitles(t *testing.T) {
	long := "既存テーブルの依存関係を全部洗い出して移行の順番を決める"
	b := &board.Board{Nodes: []board.Node{
		{ID: "a", Title: long, Row: 0, Col: 0, Done: true, DoneNote: "スキーマ確定"},
		{ID: "b", Title: "短い", Row: 1, Col: 0},
	}}
	full, blocks := newLayout(b, 120), makeLayout(b, 120, densityBlocks)
	if full.h(0) != fullCardH {
		t.Errorf("full rows keep their height: %d", full.h(0))
	}
	rows, note := blockLines(b.Nodes[0], blocks.cardW)
	if blocks.h(0) != len(rows)+1 || note == "" || blocks.h(1) != compactCardH {
		t.Errorf("a row of blocks fits its longest title and comment: %v", blocks.rowH[:2])
	}
	out := plain(renderBoard(b, blocks, nil, view{cursorRow: -1}))
	for _, want := range append(rows, note) {
		if !strings.Contains(out, want) {
			t.Errorf("%q should show:\n%s", want, out)
		}
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
	l := newLayout(b, 120).withFrames(b)
	cv := renderBoard(b, l, routeEdges(b, l), view{cursorRow: -1})
	out := plain(cv)
	t.Log("\n" + out)
	for y := 0; y < cv.h; y++ {
		if w := runewidthOf(strings.Split(out, "\n")[y]); w != cv.w {
			t.Errorf("line %d is %d cells wide, want %d", y, w, cv.w)
		}
	}
	if !strings.Contains(out, " レビュー準") {
		t.Errorf("frame title missing or cut by an edge")
	}
}

func TestFrameHugsMembersAndJoinsPieces(t *testing.T) {
	n := func(id, title string, row, col int) board.Node {
		return board.Node{ID: id, Title: title, Row: row, Col: col}
	}
	b := &board.Board{
		Nodes: []board.Node{
			n("a", "スキーマ", 0, 0), n("b", "API", 0, 1), n("x", "他", 0, 2),
			n("c", "移行", 1, 0), n("y", "他2", 1, 1),
			n("d", "遠い子", 3, 3),
		},
		Edges:  []board.Edge{{From: "a", To: "c"}, {From: "x", To: "d"}},
		Groups: []board.Group{{ID: "g", Title: "設計", Color: 2, Members: []string{"a", "b", "c", "d"}}},
	}
	l := newLayout(b, 100).withFrames(b)
	cv := renderBoard(b, l, routeEdges(b, l), view{cursorRow: -1})
	out := plain(cv)
	t.Log("\n" + out)
	if strings.Count(out, "╭") < 3 {
		t.Error("the outline should take the L shape and round the far piece")
	}
	if !strings.ContainsAny(out, "┬┴") {
		t.Error("a neck should join the far piece")
	}
}

func TestBlocksOfShortTitlesAreOneLine(t *testing.T) {
	b := sample()
	b.Nodes[1].Done = true
	b.Nodes[2].URL = "https://example.com"
	full, compact := newLayout(b, 120), makeLayout(b, 120, densityBlocks)
	if compact.height >= full.height*3/4 {
		t.Errorf("blocks %d lines, full %d: should be well under", compact.height, full.height)
	}
	if compact.rowY[1]-compact.rowY[0] != compactCardH+compact.lane {
		t.Errorf("a row of short titles should take %d lines: %v", compactCardH+compact.lane, compact.rowY)
	}
	routes := routeEdges(b, compact)
	cv := renderBoard(b, compact, routes, view{cursorRow: 1, cursorCol: 0, cursorSt: stBorderSel, lit: "d"})
	out := plain(cv)
	t.Log("\n" + out)
	lines := strings.Split(out, "\n")
	for _, rt := range routes {
		end := rt.path[len(rt.path)-1]
		if c := cv.cells[end.y*cv.w+end.x]; c.r != '▼' {
			t.Errorf("%s→%s should end in an arrowhead over its target, got %q", rt.from, rt.to, c.r)
		}
	}
	row0 := lines[compact.rowY[0]]
	if !strings.Contains(row0, " 要件を洗い出す") || !strings.Contains(row0, "✓ DB調査") || !strings.Contains(row0, "↗") {
		t.Errorf("titles, done mark and link should share the card's line: %q", row0)
	}
	if strings.ContainsAny(out, "╭╰┬") {
		t.Error("blocks have no borders or ports")
	}
	if c := cv.cells[compact.rowY[0]*cv.w+compact.colX(2)+1]; c.r != ' ' {
		t.Errorf("an empty cell in a row with nodes should be blank, got %q", c.r)
	}
	if c := cv.cells[compact.rowY[1]*cv.w+compact.colX(0)]; c.st != stCardSel {
		t.Errorf("cursor card style %v", c.st)
	}
	if c := cv.cells[compact.rowY[0]*cv.w+compact.colX(1)+compact.cardW-2]; c.st != stCardDoneBand {
		t.Errorf("a done card's padding should not be struck through: %v", c.st)
	}
}

func TestEdgesPassOutsideAFrameOnTheEdgeColumn(t *testing.T) {
	n := func(id string, row, col int) board.Node { return board.Node{ID: id, Title: id, Row: row, Col: col} }
	b := &board.Board{
		Nodes:  []board.Node{n("a", 0, 5), n("b", 1, 5), n("c", 2, 5), n("d", 3, 5), n("x", 1, 4), n("y", 2, 4), n("z", 1, 3), n("w", 2, 3)},
		Edges:  []board.Edge{{From: "a", To: "d"}},
		Groups: []board.Group{{ID: "g", Title: "枠", Members: []string{"b", "c", "x", "y"}}},
	}
	for _, w := range []int{80, 100, 121, 160, 203} {
		for _, d := range []density{densityFull, densityBlocks} {
			l := makeLayout(b, w, d).withFrames(b)
			routes := routeEdges(b, l)
			right, frameRight := 0, 0
			for _, p := range routes[0].path {
				right = max(right, p.x)
			}
			for p := range l.frames[0].lines {
				frameRight = max(frameRight, p.x)
			}
			if right <= frameRight {
				t.Errorf("width %d %s: the edge should run outside the frame (x>%d), reached x=%d\n%s",
					w, densityNames[d], frameRight, right, plain(renderBoard(b, l, routes, view{cursorRow: -1})))
			}
		}
	}
}

func TestFrameLinesWinOverUnlitEdges(t *testing.T) {
	n := func(id string, row, col int) board.Node { return board.Node{ID: id, Title: id, Row: row, Col: col} }
	b := &board.Board{
		Nodes:  []board.Node{n("a", 0, 0), n("m1", 1, 0), n("m2", 1, 1)},
		Edges:  []board.Edge{{From: "a", To: "m2"}},
		Groups: []board.Group{{ID: "g", Title: "枠", Members: []string{"m1", "m2"}}},
	}
	l := newLayout(b, 100).withFrames(b)
	routes := routeEdges(b, l)
	var cross point
	found := false
	for _, p := range routes[0].path {
		if l.frames[0].lines[p] != 0 {
			cross, found = p, true
		}
	}
	if !found {
		t.Fatal("the edge should cross the frame's top line")
	}
	at := func(v view) rune {
		cv := renderBoard(b, l, routes, v)
		return cv.cells[cross.y*cv.w+cross.x].r
	}
	if r := at(view{cursorRow: -1}); r != '─' {
		t.Errorf("unlit: the frame's line should stay whole, got %q", r)
	}
	if r := at(view{cursorRow: 0, cursorCol: 0, cursorSt: stBorderSel, lit: "a"}); r != '┃' {
		t.Errorf("lit: the edge should win, got %q", r)
	}
}

func TestBoardSitsInTheMiddle(t *testing.T) {
	for w := 90; w < 130; w++ {
		l := newLayout(&board.Board{}, w)
		left := l.colX(0)
		right := w - (l.colX(board.Cols-1) + l.cardW)
		if right-left < 0 || right-left > 1 {
			t.Errorf("width %d: margins %d left, %d right", w, left, right)
		}
	}
}

func TestOnlyEmptyRowsAmongBlocksAreDotted(t *testing.T) {
	b := &board.Board{Nodes: []board.Node{
		{ID: "a", Title: "下書きを書く", Row: 0, Col: 0},
		{ID: "b", Title: "見直す", Row: 2, Col: 1},
	}}
	l := makeLayout(b, 120, densityBlocks)
	out := strings.Split(plain(renderBoard(b, l, routeEdges(b, l), view{cursorRow: 0, cursorCol: 0, cursorSt: stBorderSel})), "\n")
	t.Log("\n" + strings.Join(out, "\n"))
	for r, want := range map[int]bool{0: false, 1: true, 2: false, 3: true} {
		if got := strings.Count(out[l.rowY[r]], "·") > board.Cols*l.cardW/3; got != want {
			t.Errorf("row %d dotted across = %v, want %v: %q", r, got, want, out[l.rowY[r]])
		}
	}
	if strings.Contains(out[l.rowY[0]], "·") {
		t.Errorf("a row with nodes should have no dots: %q", out[l.rowY[0]])
	}
}
