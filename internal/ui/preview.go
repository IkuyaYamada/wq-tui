package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/thread"
)

// pline is one line of the preview before wrapping.
type pline struct {
	text string
	st   style
}

// previewLines reads a node the way vim shows it, as one page: the strategy
// body (header stripped) and then every thread entry, oldest first.
func previewLines(dir string, n board.Node) []pline {
	nodeDir := board.NodeDir(dir, n.ID)
	var out []pline
	body := ""
	if doc, err := os.ReadFile(filepath.Join(nodeDir, "strategy.md")); err == nil {
		body = string(doc)
		if _, b, ok := splitFrontmatter(body); ok {
			body = b
		}
	}
	if body = strings.TrimSpace(body); body == "" {
		out = append(out, pline{"no strategy yet", stPreviewDim})
	}
	for _, l := range strings.Split(body, "\n") {
		if body != "" {
			out = append(out, pline{l, stPlain})
		}
	}

	entries, err := thread.List(nodeDir)
	out = append(out, pline{"", stPlain}, pline{"── thread", stPreviewDim})
	switch {
	case err != nil:
		out = append(out, pline{"thread: " + err.Error(), stPreviewDim})
	case len(entries) == 0:
		out = append(out, pline{"no entries yet", stPreviewDim})
	}
	for _, e := range entries {
		out = append(out, pline{e.Time.Format("01/02 15:04"), stPreviewWhen})
		for _, l := range strings.Split(strings.TrimSpace(e.Body), "\n") {
			out = append(out, pline{"  " + l, stPlain})
		}
	}
	return out
}

// wrapLines breaks each line to at most w cells, keeping its style.
func wrapLines(lines []pline, w int) []pline {
	var out []pline
	for _, l := range lines {
		text := strings.ReplaceAll(l.text, "\t", "    ")
		for runewidth.StringWidth(text) > w {
			cut := runewidth.Truncate(text, w, "")
			if cut == "" { // a wide rune wider than w
				break
			}
			out = append(out, pline{cut, l.st})
			text = text[len(cut):]
		}
		out = append(out, pline{text, l.st})
	}
	return out
}

// previewBox is where the preview floats on a w×h screen: beside the
// cursor's card (cardX is its left edge, top its first line on screen), on
// the right unless that runs off the screen.
type previewBox struct {
	x, y, w, h int
	text       []pline // wrapped to the box
}

func placePreview(w, h int, lines []pline, cardX, cardW, top int) (previewBox, bool) {
	pw := min(max(w/2, 30), w-2)
	if pw-4 < 4 || h < 3 {
		return previewBox{}, false
	}
	x := cardX + cardW + 1
	if x+pw > w {
		x = max(cardX-pw-1, 0)
	}
	text := wrapLines(lines, pw-4)
	ph := min(len(text)+2, h)
	return previewBox{x: x, y: max(0, min(top, h-ph)), w: pw, h: ph, text: text}, true
}

// maxScroll is how far the text can scroll before its end is on screen.
func (p previewBox) maxScroll() int { return max(0, len(p.text)-(p.h-2)) }

// draw paints the box over cv, text scrolled by scroll lines.
func (p previewBox) draw(cv *canvas, title string, scroll int) {
	x0, y0, x1, bottom := p.x, p.y, p.x+p.w-1, p.y+p.h-1
	for y := y0; y <= bottom; y++ {
		cv.clearSpan(x0, x1+1, y)
	}
	for x := x0 + 1; x < x1; x++ {
		cv.set(x, y0, '─', stPreviewBorder)
		cv.set(x, bottom, '─', stPreviewBorder)
	}
	for y := y0 + 1; y < bottom; y++ {
		cv.set(x0, y, '│', stPreviewBorder)
		cv.set(x1, y, '│', stPreviewBorder)
	}
	cv.set(x0, y0, '╭', stPreviewBorder)
	cv.set(x1, y0, '╮', stPreviewBorder)
	cv.set(x0, bottom, '╰', stPreviewBorder)
	cv.set(x1, bottom, '╯', stPreviewBorder)
	cv.text(x0+2, y0, " "+runewidth.Truncate(title, p.w-6, "…")+" ", stTitleSel)

	scroll = max(0, min(scroll, p.maxScroll()))
	rows := p.h - 2
	for i := 0; i < rows && scroll+i < len(p.text); i++ {
		l := p.text[scroll+i]
		cv.text(x0+2, y0+1+i, l.text, l.st)
	}
	if p.maxScroll() > 0 {
		pos := fmt.Sprintf(" %d/%d ^d/^u ", scroll+rows, len(p.text))
		cv.text(x1-1-runewidth.StringWidth(pos), bottom, pos, stPreviewDim)
	}
}

// clearSpan blanks cells x0..x1-1 on line y, also blanking any wide rune
// cut in half at either end.
func (c *canvas) clearSpan(x0, x1, y int) {
	if y < 0 || y >= c.h {
		return
	}
	x0, x1 = max(x0, 0), min(x1, c.w)
	if x0 > 0 && c.cells[y*c.w+x0].cont {
		c.set(x0-1, y, ' ', stPlain)
	}
	if x1 < c.w && c.cells[y*c.w+x1].cont {
		c.set(x1, y, ' ', stPlain)
	}
	for x := x0; x < x1; x++ {
		c.set(x, y, ' ', stPlain)
	}
}
