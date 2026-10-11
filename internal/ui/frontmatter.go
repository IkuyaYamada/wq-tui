package ui

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"time"
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

func (fm *frontmatter) del(key string) {
	for i, l := range fm.lines {
		if strings.HasPrefix(l, key+":") {
			fm.lines = append(fm.lines[:i], fm.lines[i+1:]...)
			return
		}
	}
}

func (fm frontmatter) render(body string) string {
	return "---\n" + strings.Join(fm.lines, "\n") + "\n---\n" + body
}

// strategyHeader is what wq keeps in strategy.md's header.
type strategyHeader struct {
	title, url string
	done       *string // the completion comment of a done node; nil when open
	session    string  // the agent session working on the node, if any
}

// syncStrategyHeader writes h into strategy.md's header, creating the file
// or the header as needed. url: is always present so there is a blank to
// fill in; done: only on a done node, session: only once there is one. A
// leading "# <title>" heading from older files is folded into the header.
func syncStrategyHeader(path string, h strategyHeader) error {
	cur, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	fm, body, ok := splitFrontmatter(string(cur))
	if !ok {
		legacy := strings.TrimPrefix(body, "# "+h.title+"\n")
		body = "\n" + strings.TrimLeft(legacy, "\n")
	}
	fm.set("title", h.title)
	fm.set("url", h.url)
	if h.done != nil {
		fm.set("done", *h.done)
	} else {
		fm.del("done")
	}
	if h.session != "" {
		fm.set("session", h.session)
	} else {
		fm.del("session")
	}
	out := fm.render(body)
	if out == string(cur) {
		return nil
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// readStrategyHeader returns what strategy.md's header holds; ok is false
// when there is no header. done is set whenever a done: line is there.
func readStrategyHeader(path string) (h strategyHeader, ok bool) {
	cur, err := os.ReadFile(path)
	if err != nil {
		return h, false
	}
	fm, _, ok := splitFrontmatter(string(cur))
	if !ok {
		return h, false
	}
	h.title, _ = fm.get("title")
	h.url, _ = fm.get("url")
	if d, found := fm.get("done"); found {
		h.done = &d
	}
	h.session, _ = fm.get("session")
	return h, true
}

// appendDoneLog adds a line recording a completion to the bottom of
// strategy.md, so the comment survives the node being reopened and done
// again.
func appendDoneLog(path string, at time.Time, note string) error {
	cur, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	line := "✓ done " + at.Format("2006-01-02 15:04")
	if note != "" {
		line += " — " + note
	}
	s := string(cur)
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return os.WriteFile(path, []byte(s+"\n"+line+"\n"), 0o644)
}
