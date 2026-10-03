package ui

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/IkuyaYamada/wq-tui/internal/board"
	"github.com/IkuyaYamada/wq-tui/internal/thread"
)

// nodeStats is how much has been written on a node: characters in the
// strategy body and thread entries written by hand.
type nodeStats struct {
	chars   int
	entries int
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
