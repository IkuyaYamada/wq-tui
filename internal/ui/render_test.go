package ui

import (
	"strings"
	"testing"

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
