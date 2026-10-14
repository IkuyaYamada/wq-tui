package ui

import (
	"github.com/IkuyaYamada/wq-tui/internal/board"
)

// FAST mode (s) moves from node to node instead of cell to cell, so empty
// cells never stop the cursor: j / k go to the nearest node in the next
// row down / up that has one, whatever its column, and h / l to the
// nearest node left / right in the same row.

// startFast enters FAST mode, remembering where the cursor was so Esc can
// put it back.
func (m *Model) startFast() {
	m.fastOrig = [2]int{m.row, m.col}
	m.mode = modeFast
}

func (m *Model) keyFast(key string) {
	switch key {
	case "j", "down":
		m.fastStep(board.Down)
	case "k", "up":
		m.fastStep(board.Up)
	case "h", "left":
		m.fastStep(board.Left)
	case "l", "right":
		m.fastStep(board.Right)
	case "enter", "s":
		m.mode = modeNormal
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.row, m.col = m.fastOrig[0], m.fastOrig[1]
	}
}

// fastStep moves the cursor to the nearest node in direction d: for up and
// down the closest row that has a node, then the closest column in it
// (the left one on a tie); for left and right the closest node in the
// cursor's row.
func (m *Model) fastStep(d board.Dir) {
	var best *board.Node
	better := func(n *board.Node) bool {
		if best == nil {
			return true
		}
		dr, br := abs(n.Row-m.row), abs(best.Row-m.row)
		if dr != br {
			return dr < br
		}
		dc, bc := abs(n.Col-m.col), abs(best.Col-m.col)
		if dc != bc {
			return dc < bc
		}
		return n.Col < best.Col
	}
	for i := range m.b.Nodes {
		n := &m.b.Nodes[i]
		var ok bool
		switch d {
		case board.Down:
			ok = n.Row > m.row
		case board.Up:
			ok = n.Row < m.row
		case board.Left:
			ok = n.Row == m.row && n.Col < m.col
		case board.Right:
			ok = n.Row == m.row && n.Col > m.col
		}
		if ok && better(n) {
			best = n
		}
	}
	if best == nil {
		m.msg = "no node that way"
		return
	}
	m.frameSel = ""
	m.cursorTo(best)
}
