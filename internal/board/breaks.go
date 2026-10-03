package board

import "sort"

// Break is a session divider drawn in the gap under row After ("break for
// lunch", "done for today"). It marks a stopping point between phases and
// never constrains nodes or edges.
type Break struct {
	After int    `json:"after"`
	Label string `json:"label,omitempty"`
}

// BreakAfter returns the break under row, if any.
func (b *Board) BreakAfter(row int) (Break, bool) {
	for _, br := range b.Breaks {
		if br.After == row {
			return br, true
		}
	}
	return Break{}, false
}

// SetBreak draws a break under row, replacing one already there.
func (b *Board) SetBreak(row int, label string) {
	b.RemoveBreak(row)
	b.Breaks = append(b.Breaks, Break{After: row, Label: label})
	sort.Slice(b.Breaks, func(i, j int) bool { return b.Breaks[i].After < b.Breaks[j].After })
}

// RemoveBreak erases the break under row.
func (b *Board) RemoveBreak(row int) {
	out := b.Breaks[:0]
	for _, br := range b.Breaks {
		if br.After != row {
			out = append(out, br)
		}
	}
	b.Breaks = out
}

// insertBreakRow keeps breaks in their gaps when an empty row opens at row:
// gaps below it move down with their rows.
func (b *Board) insertBreakRow(row int) {
	for i := range b.Breaks {
		if b.Breaks[i].After >= row {
			b.Breaks[i].After++
		}
	}
}

// deleteBreakRow keeps breaks in their gaps when row is removed. A break
// under the removed row now sits under the row above it; one that would
// land on an existing break, or above the top row, is dropped.
func (b *Board) deleteBreakRow(row int) {
	out := b.Breaks[:0]
	seen := map[int]bool{}
	for _, br := range b.Breaks {
		if br.After >= row {
			br.After--
		}
		if br.After < 0 || seen[br.After] {
			continue
		}
		seen[br.After] = true
		out = append(out, br)
	}
	b.Breaks = out
}
