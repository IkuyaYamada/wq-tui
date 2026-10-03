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
	line := key + ":" // no trailing space for an empty value
	if value != "" {
		line += " " + value
	}
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

// syncStrategyHeader writes title and url into strategy.md's header,
// creating the file or the header as needed; url: is always present so
// there is a blank to fill in. A leading "# <title>" heading from older
// files is folded into the header.
func syncStrategyHeader(path, title, url string) error {
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
	fm.set("url", url)
	out := fm.render(body)
	if out == string(cur) {
		return nil
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// readStrategyHeader returns the title and url from strategy.md's header;
// ok is false when there is no header.
func readStrategyHeader(path string) (title, url string, ok bool) {
	cur, err := os.ReadFile(path)
	if err != nil {
		return "", "", false
	}
	fm, _, ok := splitFrontmatter(string(cur))
	if !ok {
		return "", "", false
	}
	title, _ = fm.get("title")
	url, _ = fm.get("url")
	return title, url, true
}
