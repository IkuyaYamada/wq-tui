package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/IkuyaYamada/wq-tui/internal/board"
)

// nodeStats is how much has been written on a node: characters in the
// strategy body, and the strategy's task list: checkboxes, and how many of
// them are ticked.
type nodeStats struct {
	chars   int
	checks  int
	checked int
}

// readStats counts a node's writing: non-space characters of the strategy
// body and its task list.
func readStats(dir, id string) nodeStats {
	nodeDir := board.NodeDir(dir, id)
	var s nodeStats
	if doc, err := os.ReadFile(filepath.Join(nodeDir, "strategy.md")); err == nil {
		body := string(doc)
		if _, b, ok := splitFrontmatter(body); ok {
			body = b
		}
		for _, r := range body {
			if !unicode.IsSpace(r) {
				s.chars++
			}
		}
		s.checks, s.checked = countChecks(body)
	}
	return s
}

// checkItem is a Markdown task list item: "- [ ]", "* [x]", "1. [X]", at
// any depth.
var checkItem = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+\[([ xX])\]`)

// countChecks counts the task list items in a Markdown body and the ticked
// ones among them, leaving out fenced code blocks.
func countChecks(body string) (checks, checked int) {
	fenced := false
	for _, line := range strings.Split(body, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if m := checkItem.FindStringSubmatch(line); m != nil {
			checks++
			if m[1] != " " {
				checked++
			}
		}
	}
	return checks, checked
}

// progress is the task list's " done/total " label, or "" without one.
func (s nodeStats) progress() string {
	if s.checks == 0 {
		return ""
	}
	return fmt.Sprintf(" %d/%d ", s.checked, s.checks)
}

func readAllStats(dir string, b *board.Board) map[string]nodeStats {
	out := make(map[string]nodeStats, len(b.Nodes))
	for _, n := range b.Nodes {
		out[n.ID] = readStats(dir, n.ID)
	}
	return out
}

// meterRune turns a count into a bar a quarter to fully high, or 0 for
// nothing written. steps are the counts where the bar grows.
func meterRune(n int, steps [3]int) rune {
	switch {
	case n <= 0:
		return 0
	case n < steps[0]:
		return '▂'
	case n < steps[1]:
		return '▄'
	case n < steps[2]:
		return '▆'
	}
	return '█'
}

// charSteps are the strategy lengths where the meter grows.
var charSteps = [3]int{100, 400, 1000}

// meter is the maturity gauge on a card's bottom border: how long the
// strategy is, or 0 when nothing is written.
func (s nodeStats) meter() rune { return meterRune(s.chars, charSteps) }
