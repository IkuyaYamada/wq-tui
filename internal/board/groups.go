package board

import (
	"errors"
	"time"
)

// Group is a frame drawn around some nodes to show they belong together.
// It marks a meaningful cluster: it takes no cell of its own, never
// constrains moves or edges, and its frame is the smallest rectangle around
// its members, so it follows them wherever they go. Like a node it has an
// id, so it keeps its own strategy and thread under nodes/<id>/.
type Group struct {
	ID        string    `json:"id,omitempty"`
	Title     string    `json:"title,omitempty"`
	URL       string    `json:"url,omitempty"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	Members   []string  `json:"members"`
}

var ErrNested = errors.New("already in a frame — frames do not nest")

func cloneGroups(gs []Group) []Group {
	if gs == nil {
		return nil
	}
	out := make([]Group, len(gs))
	for i, g := range gs {
		out[i] = g
		out[i].Members = append([]string(nil), g.Members...)
	}
	return out
}

// Group returns a pointer into b.Groups; it is invalidated by any change
// to the groups.
func (b *Board) Group(id string) *Group {
	for i := range b.Groups {
		if b.Groups[i].ID == id {
			return &b.Groups[i]
		}
	}
	return nil
}

// EnsureGroupIDs gives an id to frames made before frames had one.
func (b *Board) EnsureGroupIDs(now time.Time) {
	for i := range b.Groups {
		if b.Groups[i].ID == "" {
			b.Groups[i].ID = NewID(now.Add(time.Duration(i) * time.Second))
			b.Groups[i].CreatedAt = now
		}
	}
}

// Frame frames ids as group g (its members are set from ids), taking them
// out of any group they were in.
func (b *Board) Frame(g Group, ids []string) {
	in := map[string]bool{}
	var members []string
	for _, id := range ids {
		if b.Node(id) != nil && !in[id] {
			in[id] = true
			members = append(members, id)
		}
	}
	if len(members) == 0 {
		return
	}
	b.leaveGroups(in)
	g.Members = members
	b.Groups = append(b.Groups, g)
}

// RemoveGroup erases the frame id; its members stay where they are.
func (b *Board) RemoveGroup(id string) {
	out := b.Groups[:0]
	for _, g := range b.Groups {
		if g.ID != id {
			out = append(out, g)
		}
	}
	b.Groups = out
}

// Decompose breaks node id up into a frame: the frame takes over its id,
// title, url and creation time (and so its strategy and thread), and child
// takes its cell and its edges as the frame's first member.
func (b *Board) Decompose(id string, child Node) error {
	n := b.Node(id)
	if n == nil {
		return ErrNotFound
	}
	if b.GroupOf(id) >= 0 {
		return ErrNested
	}
	g := Group{ID: n.ID, Title: n.Title, URL: n.URL, CreatedAt: n.CreatedAt, Members: []string{child.ID}}
	child.Row, child.Col = n.Row, n.Col
	*n = child
	for i := range b.Edges {
		if b.Edges[i].From == id {
			b.Edges[i].From = child.ID
		}
		if b.Edges[i].To == id {
			b.Edges[i].To = child.ID
		}
	}
	b.Groups = append(b.Groups, g)
	return nil
}

// GroupNodes frames ids as a new, untitled group.
func (b *Board) GroupNodes(ids []string) { b.Frame(Group{}, ids) }

// Ungroup takes ids out of their groups; a group left empty is removed.
func (b *Board) Ungroup(ids []string) {
	in := map[string]bool{}
	for _, id := range ids {
		in[id] = true
	}
	b.leaveGroups(in)
}

func (b *Board) leaveGroups(ids map[string]bool) {
	out := b.Groups[:0]
	for _, g := range b.Groups {
		kept := g.Members[:0]
		for _, id := range g.Members {
			if !ids[id] {
				kept = append(kept, id)
			}
		}
		if len(kept) > 0 {
			g.Members = kept
			out = append(out, g)
		}
	}
	b.Groups = out
}

// GroupOf is the index of id's group, or -1.
func (b *Board) GroupOf(id string) int {
	for i, g := range b.Groups {
		for _, m := range g.Members {
			if m == id {
				return i
			}
		}
	}
	return -1
}

// JoinGroupOf puts id in the group of other, if other has one.
func (b *Board) JoinGroupOf(id, other string) {
	if g := b.GroupOf(other); g >= 0 && b.GroupOf(id) < 0 {
		b.Groups[g].Members = append(b.Groups[g].Members, id)
	}
}

// Bounds is the cell rectangle around a group's members.
func (b *Board) Bounds(g Group) (r0, c0, r1, c1 int, ok bool) {
	for _, id := range g.Members {
		n := b.Node(id)
		if n == nil {
			continue
		}
		if !ok {
			r0, c0, r1, c1, ok = n.Row, n.Col, n.Row, n.Col, true
			continue
		}
		r0, c0 = min(r0, n.Row), min(c0, n.Col)
		r1, c1 = max(r1, n.Row), max(c1, n.Col)
	}
	return
}

// GroupAt is the index of the group whose frame covers the cell, or -1.
func (b *Board) GroupAt(row, col int) int {
	for i, g := range b.Groups {
		if r0, c0, r1, c1, ok := b.Bounds(g); ok && row >= r0 && row <= r1 && col >= c0 && col <= c1 {
			return i
		}
	}
	return -1
}
