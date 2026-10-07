package board

import (
	"errors"
	"time"
)

// Group is a frame drawn around some nodes to show they belong together.
// It marks a meaningful cluster: it takes no cell of its own, never
// constrains moves or edges, and its frame hugs its members, so it follows
// them wherever they go. Like a node it has an id, so it keeps its own
// strategy and thread under nodes/<id>/.
type Group struct {
	ID        string    `json:"id,omitempty"`
	Title     string    `json:"title,omitempty"`
	URL       string    `json:"url,omitempty"`
	Color     int       `json:"color,omitempty"` // 1..FrameColors; 0 is not given yet
	CreatedAt time.Time `json:"created_at,omitzero"`
	Members   []string  `json:"members"`
}

// FrameColors is how many colors frames take turns in.
const FrameColors = 4

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

// EnsureGroupColors gives a color to frames made before frames had one,
// in turn so neighbours in the list differ.
func (b *Board) EnsureGroupColors() {
	for i := range b.Groups {
		if b.Groups[i].Color == 0 {
			b.Groups[i].Color = b.nextColor()
		}
	}
}

// nextColor is the color fewest frames have, the first of them on a tie,
// so new frames go round the palette.
func (b *Board) nextColor() int {
	var used [FrameColors + 1]int
	for _, g := range b.Groups {
		if g.Color >= 1 && g.Color <= FrameColors {
			used[g.Color]++
		}
	}
	best := 1
	for c := 2; c <= FrameColors; c++ {
		if used[c] < used[best] {
			best = c
		}
	}
	return best
}

// CycleColor gives frame id the next color of the palette.
func (b *Board) CycleColor(id string) {
	if g := b.Group(id); g != nil {
		g.Color = g.Color%FrameColors + 1
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
	if g.Color == 0 {
		g.Color = b.nextColor()
	}
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
	g := Group{ID: n.ID, Title: n.Title, URL: n.URL, Color: b.nextColor(), CreatedAt: n.CreatedAt, Members: []string{child.ID}}
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

// FramesOf lists the ids of the frames some of ids are members of, in the
// order of b.Groups.
func (b *Board) FramesOf(ids []string) []string {
	in := map[string]bool{}
	for _, id := range ids {
		in[id] = true
	}
	var out []string
	for _, g := range b.Groups {
		for _, m := range g.Members {
			if in[m] {
				out = append(out, g.ID)
				break
			}
		}
	}
	return out
}

// AddToGroup puts ids in frame id, taking them out of any other frame;
// ids already in it stay put. It reports how many joined.
func (b *Board) AddToGroup(id string, ids []string) int {
	g := b.Group(id)
	if g == nil {
		return 0
	}
	have := map[string]bool{}
	for _, m := range g.Members {
		have[m] = true
	}
	var add []string
	for _, n := range ids {
		if b.Node(n) != nil && !have[n] {
			have[n] = true
			add = append(add, n)
		}
	}
	if len(add) == 0 {
		return 0
	}
	leave := map[string]bool{}
	for _, n := range add {
		leave[n] = true
	}
	b.leaveGroups(leave)
	g = b.Group(id)
	g.Members = append(g.Members, add...)
	return len(add)
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

// GroupAt is the index of the group the cell belongs to, or -1: the group
// of the node on it, or else the frame that walls the cell in, as members
// all round it do.
func (b *Board) GroupAt(row, col int) int {
	if n := b.At(row, col); n != nil {
		if g := b.GroupOf(n.ID); g >= 0 {
			return g
		}
	}
	for i := range b.Groups {
		if b.enclosed(b.Groups[i], row, col) {
			return i
		}
	}
	return -1
}

// enclosed reports whether a cell that is not one of g's members is walled
// in by them: no path of non-member cells, diagonal steps included, leads
// from it out of the members' bounding box. A diagonal gap is a way out,
// as the frame does not close there either.
func (b *Board) enclosed(g Group, row, col int) bool {
	in := map[[2]int]bool{}
	r0, c0, r1, c1 := row, col, row, col
	for _, id := range g.Members {
		if n := b.Node(id); n != nil {
			in[[2]int{n.Row, n.Col}] = true
			r0, c0 = min(r0, n.Row), min(c0, n.Col)
			r1, c1 = max(r1, n.Row), max(c1, n.Col)
		}
	}
	start := [2]int{row, col}
	if in[start] {
		return false
	}
	seen := map[[2]int]bool{start: true}
	queue := [][2]int{start}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if p[0] == r0 || p[0] == r1 || p[1] == c0 || p[1] == c1 {
			return false
		}
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				q := [2]int{p[0] + dr, p[1] + dc}
				if !in[q] && !seen[q] {
					seen[q] = true
					queue = append(queue, q)
				}
			}
		}
	}
	return true
}
