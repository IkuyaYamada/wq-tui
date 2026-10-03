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

// OpenRowBelow opens an empty row under row that belongs with it: a break
// under row moves down to sit under the new row, as the rows below do.
func (b *Board) OpenRowBelow(row int) {
	b.InsertRow(row + 1)
	for i := range b.Breaks {
		if b.Breaks[i].After == row {
			b.Breaks[i].After = row + 1
		}
	}
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

// MoveBreak slides the break under row to the next free gap up (step -1) or
// down (step 1), hopping over gaps that already have one and staying under
// rows 0..last. It returns where the break landed, or false if it could not
// move.
func (b *Board) MoveBreak(row, step, last int) (int, bool) {
	br, ok := b.BreakAfter(row)
	if !ok {
		return row, false
	}
	to := row + step
	for to >= 0 && to <= last {
		if _, taken := b.BreakAfter(to); !taken {
			b.RemoveBreak(row)
			b.SetBreak(to, br.Label)
			return to, true
		}
		to += step
	}
	return row, false
}
