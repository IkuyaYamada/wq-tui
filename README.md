# wq-tui

A keyboard-driven six-column board for the terminal. Everything you are
working on lives on one grid; each node opens to its **strategy** (left) and a
**thread** of short timestamped entries (right), all edited in vim.

Successor to the web-based `workflow-queue`, keeping its grid rules and keymap.

## Run

```sh
mise install        # Go toolchain (see mise.toml)
go build -o wq . && ./wq   # mise.toml sets CGO_ENABLED=0
```

Data lives in `~/wq` (override with `WQ_DIR`):

```
~/wq/board.json                 # nodes (title, row, col, done) and edges
~/wq/nodes/<id>/strategy.md
~/wq/nodes/<id>/thread/20261001-152003.md   # one file per thread entry
```

Plain files, so the directory can be a git repo of its own.

## Keys

| Key | Action |
| --- | --- |
| `hjkl` / arrows | Move the cursor one cell (empty cells included) |
| `w` / `b` | Jump to the next / previous node |
| `g` / `G` | Top / last row with nodes |
| `a` / `n` | Add a node on the cursor cell (or the nearest empty cell if taken) |
| `o` / `O` | Insert a node below / above, taking over the outgoing / incoming edges (A → B becomes A → new → B). `O` only fills the free cell above and never moves other rows; it reports an error when that cell is taken, on the top row, or when an incoming node sits on that row |
| `i` | Rename (or edit `title:` in vim) |
| `m` | Move mode: `hjkl` slides to the next empty cell, `Enter` places, `Esc` cancels |
| `v` / `V` | Visual mode: select a block of cells / whole rows; `m` moves the selection together, `d` deletes it |
| `c` | Connect mode: `hjkl` picks a target, `Enter` toggles the edge (either end works; edges always point down) |
| `Space` | Toggle done (logged to the thread) |
| `Enter` | Open the node's detail screen |
| `x` | Delete the node under the cursor (A → B → C is bridged to A → C) |
| `dd` / `D` | Delete the cursor's row, nodes included, pulling the rows below up |
| `u` / `Ctrl+r` | Undo / redo |
| `q` | Quit |

New nodes ask for a title right away; `Esc` on that prompt discards the node.

## Rules

- Six columns, rows grow downward, one node per cell.
- Three empty rows always follow the lowest node, so there is room to walk
  into and drop new nodes.
- Edges always point to a lower row. Connecting two nodes on the same row
  pushes the target down a row; upward edges are refused.

## Node detail screen

`Enter` on a node shows its strategy on the left and its thread on the right:
the entries, oldest first, with the selected one's full text below.

| Key | Action |
| --- | --- |
| `Tab` / `h` / `l` | Switch focus between the strategy and thread panes (the focused one is marked ▸) |
| `j` / `k`, `g` / `G` | Select an entry, or scroll the strategy when it has focus |
| `a` / `o` | New entry, opened beside the strategy in insert mode (left empty, it is discarded) |
| `Enter` / `e` / `i` | Open vim split like the screen: strategy left, selected entry right, cursor on the focused side |
| `s` | Same split, cursor on the strategy |
| `x` / `d` | Delete the entry (moved to `thread/.trash/`) |
| `u` | Restore the last entry deleted on this visit |
| `Space` | Toggle done (logged as a `Completed` / `Reopened` entry) |
| `Esc` / `q` | Back to the board |

vim mirrors the screen: `vim -O strategy.md <entry>.md` (just the strategy
while the thread is empty), so `C-w w` moves between the two. `WQ_VIM`
overrides the binary. For that session only, `q` in normal mode saves both
and returns.

`strategy.md` starts with a small header that carries the node's title:

```markdown
---
title: 設計
---

(strategy)
```

Edit the `title:` line and the node is renamed when you leave vim (undoable
with `u` on the board; a blank title is ignored). `board.json` stays the
source of truth: the header is rewritten with the current title every time
the strategy opens. Other `key: value` lines you add to the header are kept.

An older single-file `thread.md` is split into entries (one per `## time`
heading or `- time Completed` line) the first time the node is opened, and
kept as `thread.md.migrated`.

## Input method (macOS)

Board keys need ASCII, so wq switches the keyboard to the ASCII input source
(e.g. ABC) whenever you are on the board, including after returning from vim.
While typing a title it switches back to the input method you were using
(e.g. Japanese). Full-width keys such as `ｈｊｋｌ` are also understood on the
board. Set `WQ_IME=off` to leave the input source alone.

The switch goes through the Carbon Text Input Sources API, loaded at runtime
with purego, so no cgo is needed. `WQ_IME_LIVE=1 go test ./internal/ime`
checks it against the real input source (and restores it).
