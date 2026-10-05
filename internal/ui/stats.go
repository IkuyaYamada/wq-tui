package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/thread"
)

// nodeStats is how much has been written on a node: characters in the
// strategy body and thread entries written by hand, and the strategy's
// task list: checkboxes, and how many of them are ticked.
type nodeStats struct {
	chars   int
	entries int
	checks  int
	checked int
}

// readStats counts a node's writing without touching its files (no thread
// migration): non-space characters of the strategy body, and thread entries
// other than the Completed / Reopened lines wq logs itself.
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
	files, _ := filepath.Glob(filepath.Join(thread.Dir(nodeDir), "*.md"))
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil || isLogEntry(string(body)) || strings.TrimSpace(string(body)) == "" {
			continue
		}
		s.entries++
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

// isLogEntry reports whether body is a line wq wrote on completing or
// reopening a node ("Completed", "Completed: <comment>", "Reopened").
func isLogEntry(body string) bool {
	body = strings.TrimSpace(body)
	for _, ev := range []string{"Completed", "Reopened"} {
		if body == ev || (strings.HasPrefix(body, ev+": ") && !strings.Contains(body, "\n")) {
			return true
		}
	}
	return false
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

var (
	charSteps  = [3]int{100, 400, 1000} // strategy characters
	entrySteps = [3]int{2, 5, 10}       // thread entries
)

// meters is the two-bar maturity gauge on a card's bottom border: strategy
// length, then thread entries.
func (s nodeStats) meters() [2]rune {
	return [2]rune{meterRune(s.chars, charSteps), meterRune(s.entries, entrySteps)}
}
