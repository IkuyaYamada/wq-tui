package ui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/thread"
)

// editKind is what the in-preview editor writes to.
type editKind int

const (
	editStrategy editKind = iota // the strategy body (i)
	editEntry                    // a new thread entry (a)
)

// startEdit turns the preview into an editor for the cursor's node, for
// quick fixes without leaving the board.
func (m *Model) startEdit(kind editKind) tea.Cmd {
	n := m.focus()
	if n == nil {
		return nil
	}
	text := ""
	if kind == editStrategy {
		text = strategyBody(m.dir, *n)
	}
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.SetValue(text)
	// Keep at least the room the preview had, so the box does not shrink
	// under the cursor when editing starts.
	m.editRows = 0
	if p, ok := m.previewBox(); ok {
		m.editRows = p.h - 2
	}
	m.edit, m.editKind, m.editOrig, m.editWarned = ta, kind, text, false
	m.mode = modeEdit
	m.sizeEditor()
	m.ime.Select(m.imePrev)
	cmd := m.edit.Focus()
	// Start at the top, as vim does. Not by stepping CursorUp: on a
	// soft-wrapped line without spaces (Japanese) it can cycle forever.
	m.edit, _ = m.edit.Update(tea.KeyMsg{Type: tea.KeyCtrlHome})
	return cmd
}

// strategyBody is strategy.md without its header, trimmed.
func strategyBody(dir string, n board.Node) string {
	doc, err := os.ReadFile(filepath.Join(board.NodeDir(dir, n.ID), "strategy.md"))
	if err != nil {
		return ""
	}
	body := string(doc)
	if _, b, ok := splitFrontmatter(body); ok {
		body = b
	}
	return strings.Trim(body, "\n")
}

// editBox is the preview box sized for the editor: as tall as the preview
// was (10 lines at least), taller as the wrapped text grows, never past the
// screen.
func (m *Model) editBox() (previewBox, bool) {
	n := m.focus()
	if n == nil {
		return previewBox{}, false
	}
	l := newLayout(m.b, m.width)
	place := func(rows int) (previewBox, bool) {
		return placePreview(l.width, m.bodyHeight(), make([]pline, rows), l.colX(m.col), l.cardW, l.rowY[m.row]-m.scroll)
	}
	p, ok := place(1)
	if !ok {
		return p, false
	}
	var lines []pline
	for _, s := range strings.Split(m.edit.Value(), "\n") {
		lines = append(lines, pline{s, stPlain})
	}
	wrapped := len(wrapLines(lines, p.w-4)) + 1 // a spare line to type into
	return place(max(wrapped, m.editRows, 10))
}

func (m *Model) sizeEditor() {
	if p, ok := m.editBox(); ok {
		m.edit.SetWidth(p.w - 4)
		m.edit.SetHeight(p.h - 2)
	}
}

// keyEdit types into the editor. ^s saves; Esc leaves, but asks once
// before throwing away changes.
func (m *Model) keyEdit(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "ctrl+s":
		m.saveEdit()
		m.leaveEdit()
		return nil
	case "esc", "ctrl+c":
		if m.edit.Value() != m.editOrig && !m.editWarned {
			m.editWarned = true
			m.msg = "unsaved changes — ^s save · esc again to discard"
			return nil
		}
		m.leaveEdit()
		return nil
	}
	m.editWarned = false
	var cmd tea.Cmd
	m.edit, cmd = m.edit.Update(k)
	return cmd
}

func (m *Model) leaveEdit() {
	m.edit.Blur()
	m.mode = modeNormal
	m.previewID = "" // reread what was written
	m.toASCII()
}

func (m *Model) saveEdit() {
	n := m.focus()
	text := strings.Trim(m.edit.Value(), "\n")
	if n == nil || m.edit.Value() == m.editOrig {
		return
	}
	var err error
	switch m.editKind {
	case editStrategy:
		err = writeStrategyBody(m.dir, *n, text)
	case editEntry:
		if strings.TrimSpace(text) == "" {
			return
		}
		_, err = thread.Add(board.NodeDir(m.dir, n.ID), time.Now(), text+"\n")
	}
	if err != nil {
		m.msg = "save: " + err.Error()
		return
	}
	m.msg = "saved"
}

// writeStrategyBody replaces strategy.md's body, keeping its header.
func writeStrategyBody(dir string, n board.Node, body string) error {
	path, err := strategyPath(dir, n) // makes sure the header is there
	if err != nil {
		return err
	}
	doc, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fm, _, _ := splitFrontmatter(string(doc))
	if body != "" {
		body += "\n"
	}
	return os.WriteFile(path, []byte(fm.render("\n"+body)), 0o644)
}

// drawEditor paints the editor's box on screen and returns the screen
// lines, with the textarea's own rendering spliced into the box.
func (m *Model) drawEditor(screen *canvas) []string {
	out := make([]string, screen.h)
	p, ok := m.editBox()
	if !ok {
		for i := range out {
			out[i] = screen.line(i)
		}
		return out
	}
	title := m.focus().Title + " · strategy"
	if m.editKind == editEntry {
		title = m.focus().Title + " · new entry"
	}
	p.text = nil
	p.draw(screen, title, 0)
	rows := strings.Split(m.edit.View(), "\n")
	inner := p.w - 4
	for y := range out {
		i := y - p.y - 1
		if i < 0 || i >= p.h-2 {
			out[y] = screen.line(y)
			continue
		}
		row := ""
		if i < len(rows) {
			row = rows[i]
		}
		if w := lipgloss.Width(row); w < inner {
			row += strings.Repeat(" ", inner-w)
		}
		out[y] = screen.spliced(y, p.x+2, p.x+2+inner, row)
	}
	return out
}
