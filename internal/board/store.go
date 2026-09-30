package board

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Load reads dir/board.json; a missing file is an empty board.
func Load(dir string) (*Board, error) {
	data, err := os.ReadFile(filepath.Join(dir, "board.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return &Board{}, nil
	}
	if err != nil {
		return nil, err
	}
	var b Board
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// Save writes dir/board.json atomically.
func Save(dir string, b *Board) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	out := b.Clone()
	out.Sort()
	if out.Nodes == nil {
		out.Nodes = []Node{}
	}
	if out.Edges == nil {
		out.Edges = []Edge{}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "board.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// NodeDir holds a node's strategy.md and thread.md.
func NodeDir(dir, id string) string {
	return filepath.Join(dir, "nodes", id)
}

// NewID is sortable by creation time and safe to use as a directory name.
func NewID(now time.Time) string {
	var r [2]byte
	_, _ = rand.Read(r[:])
	return now.Format("20060102-150405") + "-" + hex.EncodeToString(r[:])
}
