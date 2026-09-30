package ui

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

// frontmatter is the "---" delimited header at the top of strategy.md. It is
// kept as raw lines so fields added by hand survive a rewrite; only simple
// "key: value" lines are understood.
type frontmatter struct {
	lines []string
}

// splitFrontmatter separates the header from the body. ok is false when the
// document has no header.
func splitFrontmatter(doc string) (fm frontmatter, body string, ok bool) {
	rest, found := strings.CutPrefix(doc, "---\n")
	if !found {
		return frontmatter{}, doc, false
	}
	head, body, found := strings.Cut(rest, "\n---\n")
	if !found {
		head, found = strings.CutSuffix(rest, "\n---")
		if !found {
			return frontmatter{}, doc, false
		}
	}
	if head != "" {
		fm.lines = strings.Split(head, "\n")
	}
	return fm, body, true
}

func (fm frontmatter) get(key string) (string, bool) {
	for _, l := range fm.lines {
		if v, found := strings.CutPrefix(l, key+":"); found {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

func (fm *frontmatter) set(key, value string) {
	line := key + ": " + value
	for i, l := range fm.lines {
		if strings.HasPrefix(l, key+":") {
			fm.lines[i] = line
			return
		}
	}
	fm.lines = append(fm.lines, line)
}

func (fm frontmatter) render(body string) string {
	return "---\n" + strings.Join(fm.lines, "\n") + "\n---\n" + body
}

// syncStrategyTitle writes title into strategy.md's header, creating the
// file or the header as needed. A leading "# <title>" heading from older
// files is folded into the header.
func syncStrategyTitle(path, title string) error {
	cur, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	fm, body, ok := splitFrontmatter(string(cur))
	if !ok {
		legacy := strings.TrimPrefix(body, "# "+title+"\n")
		body = "\n" + strings.TrimLeft(legacy, "\n")
	}
	fm.set("title", title)
	out := fm.render(body)
	if out == string(cur) {
		return nil
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// readStrategyTitle returns the non-empty title from strategy.md's header.
func readStrategyTitle(path string) (string, bool) {
	cur, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	fm, _, ok := splitFrontmatter(string(cur))
	if !ok {
		return "", false
	}
	title, ok := fm.get("title")
	return title, ok && title != ""
}
