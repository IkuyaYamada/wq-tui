package board

// FocusIndex is id's place in the focus list, or -1 when it is not there.
func (b *Board) FocusIndex(id string) int {
	for i, f := range b.Focus {
		if f == id {
			return i
		}
	}
	return -1
}

// ToggleFocus adds id to the end of the focus list, or takes it out when it
// is already there, and reports whether it is in now.
func (b *Board) ToggleFocus(id string) bool {
	if b.FocusIndex(id) >= 0 {
		b.Unfocus(id)
		return false
	}
	b.Focus = append(b.Focus, id)
	return true
}

// Unfocus takes id out of the focus list.
func (b *Board) Unfocus(id string) {
	out := b.Focus[:0]
	for _, f := range b.Focus {
		if f != id {
			out = append(out, f)
		}
	}
	b.Focus = out
	if len(b.Focus) == 0 {
		b.Focus = nil
	}
}

// Focused lists the focus nodes that are on the board, in order. A done
// node stays: it may still be on your mind.
func (b *Board) Focused() []string {
	var out []string
	for _, id := range b.Focus {
		if b.Node(id) != nil {
			out = append(out, id)
		}
	}
	return out
}
