// Package thread stores a node's thread as one Markdown file per entry under
// nodes/<id>/thread/, named by creation time so a listing is chronological.
package thread

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const stampLayout = "20060102-150405"

type Entry struct {
	Path string
	Time time.Time
	Body string
}

// Summary is the first non-blank line, for one-line listings.
func (e Entry) Summary() string {
	for _, l := range strings.Split(e.Body, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

func Dir(nodeDir string) string { return filepath.Join(nodeDir, "thread") }

func trashDir(nodeDir string) string { return filepath.Join(Dir(nodeDir), ".trash") }

// List returns the entries oldest first.
func List(nodeDir string) ([]Entry, error) {
	if err := migrate(nodeDir); err != nil {
		return nil, err
	}
	files, err := os.ReadDir(Dir(nodeDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		path := filepath.Join(Dir(nodeDir), f.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out = append(out, Entry{Path: path, Time: parseTime(f.Name()), Body: string(body)})
	}
	// Compare stems: "…-152003" must sort before "…-152003-02".
	stem := func(e Entry) string { return strings.TrimSuffix(filepath.Base(e.Path), ".md") }
	sort.SliceStable(out, func(i, j int) bool { return stem(out[i]) < stem(out[j]) })
	return out, nil
}

func parseTime(name string) time.Time {
	stamp := strings.TrimSuffix(name, ".md")
	if len(stamp) >= len(stampLayout) {
		stamp = stamp[:len(stampLayout)]
	}
	t, _ := time.ParseInLocation(stampLayout, stamp, time.Local)
	return t
}

// Add writes a new entry stamped with now; a clash within the same second
// gets a numeric suffix so ordering is kept.
func Add(nodeDir string, now time.Time, body string) (Entry, error) {
	if err := os.MkdirAll(Dir(nodeDir), 0o755); err != nil {
		return Entry{}, err
	}
	base := now.Format(stampLayout)
	for i := 1; ; i++ {
		name := base + ".md"
		if i > 1 {
			name = fmt.Sprintf("%s-%02d.md", base, i)
		}
		path := filepath.Join(Dir(nodeDir), name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return Entry{}, err
		}
		_, err = f.WriteString(body)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return Entry{Path: path, Time: now, Body: body}, err
	}
}

// Trash moves an entry aside so it can be restored; it returns the trashed
// path.
func Trash(nodeDir string, e Entry) (string, error) {
	if err := os.MkdirAll(trashDir(nodeDir), 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(trashDir(nodeDir), filepath.Base(e.Path))
	return dst, os.Rename(e.Path, dst)
}

// Restore moves a trashed entry back into the thread.
func Restore(nodeDir, trashed string) error {
	return os.Rename(trashed, filepath.Join(Dir(nodeDir), filepath.Base(trashed)))
}

// migrate splits a legacy single-file thread.md into entries: each
// "## YYYY-MM-DD HH:MM" heading starts one, and each "- YYYY-MM-DD HH:MM
// Completed" style log line becomes its own. The old file is kept as
// thread.md.migrated.
func migrate(nodeDir string) error {
	legacy := filepath.Join(nodeDir, "thread.md")
	data, err := os.ReadFile(legacy)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	info, err := os.Stat(legacy)
	if err != nil {
		return err
	}
	type draft struct {
		t     time.Time
		lines []string
	}
	var drafts []*draft
	var cur *draft
	for _, line := range strings.Split(string(data), "\n") {
		if t, ok := cutStamp(line, "## "); ok {
			cur = &draft{t: t}
			drafts = append(drafts, cur)
			continue
		}
		if t, ok := cutStamp(line, "- "); ok {
			event := strings.TrimSpace(line[len("- 2006-01-02 15:04"):])
			drafts = append(drafts, &draft{t: t, lines: []string{event}})
			cur = nil
			continue
		}
		if cur == nil {
			if strings.TrimSpace(line) == "" {
				continue
			}
			cur = &draft{t: info.ModTime()}
			drafts = append(drafts, cur)
		}
		cur.lines = append(cur.lines, line)
	}
	for _, d := range drafts {
		body := strings.TrimSpace(strings.Join(d.lines, "\n"))
		if body == "" {
			continue
		}
		if _, err := Add(nodeDir, d.t, body+"\n"); err != nil {
			return err
		}
	}
	return os.Rename(legacy, legacy+".migrated")
}

func cutStamp(line, prefix string) (time.Time, bool) {
	rest, ok := strings.CutPrefix(line, prefix)
	if !ok || len(rest) < len("2006-01-02 15:04") {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", rest[:len("2006-01-02 15:04")], time.Local)
	return t, err == nil
}
