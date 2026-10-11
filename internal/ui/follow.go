package ui

import (
	"fmt"
	"sort"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

// follow is where FOLLOW mode (f) has been: the node it started on, and
// the last step's choices, so h / l can switch to another branch.
type follow struct {
	start   string
	from    string   // the node the last step left
	choices []string // where that step could have gone, left to right
	at      int      // index of the cursor's node in choices
}

// startFollow enters FOLLOW mode on the node under the cursor.
func (m *Model) startFollow() {
	n := m.selected()
	if n == nil {
		return
	}
	m.follow = follow{start: n.ID}
	m.mode = modeFollow
}

func (m *Model) keyFollow(key string) {
	switch key {
	case "j", "down":
		m.followStep(true)
	case "k", "up":
		m.followStep(false)
	case "h", "left":
		m.followSwitch(-1)
	case "l", "right":
		m.followSwitch(1)
	case "enter":
		m.mode = modeNormal
	case "esc", "ctrl+c", "f":
		m.mode = modeNormal
		m.cursorTo(m.b.Node(m.follow.start))
	}
}

// followStep moves along an edge from the node under the cursor: down to
// a successor or up to a predecessor, the leftmost when there are several.
func (m *Model) followStep(down bool) {
	n := m.selected()
	if n == nil {
		return
	}
	var ids []string
	for _, e := range m.b.Edges {
		switch {
		case down && e.From == n.ID:
			ids = append(ids, e.To)
		case !down && e.To == n.ID:
			ids = append(ids, e.From)
		}
	}
	var choices []*board.Node
	for _, id := range ids {
		if c := m.b.Node(id); c != nil {
			choices = append(choices, c)
		}
	}
	if len(choices) == 0 {
		if down {
			m.msg = "no edge goes on from here"
		} else {
			m.msg = "no edge comes into here"
		}
		return
	}
	sort.SliceStable(choices, func(i, j int) bool {
		if choices[i].Col != choices[j].Col {
			return choices[i].Col < choices[j].Col
		}
		return choices[i].Row < choices[j].Row
	})
	m.follow.from = n.ID
	m.follow.choices = m.follow.choices[:0]
	for _, c := range choices {
		m.follow.choices = append(m.follow.choices, c.ID)
	}
	m.follow.at = 0
	m.cursorTo(choices[0])
	m.followCount()
}

// followSwitch moves to the previous (-1) or next (1) of the last step's
// choices: another branch from the same node.
func (m *Model) followSwitch(step int) {
	f := &m.follow
	if len(f.choices) < 2 {
		if f.from == "" {
			m.msg = "j / k follow an edge first"
		} else {
			m.msg = "no other branch here"
		}
		return
	}
	f.at = (f.at + step + len(f.choices)) % len(f.choices)
	m.cursorTo(m.b.Node(f.choices[f.at]))
	m.followCount()
}

// followCount says which branch the cursor is on when there are several.
func (m *Model) followCount() {
	if k := len(m.follow.choices); k > 1 {
		m.msg = fmt.Sprintf("branch %d/%d · h/l other branches", m.follow.at+1, k)
	}
}
