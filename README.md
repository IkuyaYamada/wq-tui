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
| `o` / `O` | Insert a node in the free cell directly below / above, taking over the outgoing / incoming edges (A → B becomes A → new → B). Nothing else moves; if that cell is taken, off the board, or an edge would turn sideways, it reports an error instead |
| `i` | Rename (or edit `title:` in vim) |
| `m` | Move mode: `hjkl` slides to the next empty cell, `Enter` places, `Esc` cancels |
| `v` / `V` | Visual mode: select a block of cells / whole rows; `m` moves the selection together, `d` deletes it |
| `c` | Connect mode: `hjkl` picks a target, `Enter` toggles the edge (either end works; edges always point down) |
| `Space` | Complete: asks for a comment (`Enter` completes, `Esc` cancels) and logs `Completed: <comment>` to the thread. On a done node it reopens right away (`Reopened`) |
| `Enter` | Open the node in vim (strategy, thread index, entry) |
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

## Opening a node

`Enter` on a node opens it straight in vim, laid out as

```
 strategy.md            │ ▸ 10/01 15:20  ログ見たら500多発   ← thread index
                        │   10/01 14:03  クエリ流した
                        │   10/01 16:43  Completed
                        ├──────────────────────────────────
                        │ (the entry picked in the index)
```

`C-w w` cycles through the three windows, and `-` jumps back to the index
from anywhere (reopening its window if it was closed). The index window is
pinned to its buffer (`winfixbuf`), so buffer-switching maps such as `:bnext`
cannot replace it. Strategy and thread split the width half and half. In the
index:

| Key | Action |
| --- | --- |
| `Enter` / `o` | Open the entry under the cursor below |
| `a` | New entry below, in insert mode (never written if left untouched) |
| `D` | Delete the entry (after a prompt; moved to `thread/.trash/`) |

`q` in normal mode saves everything and returns to the board; entries saved
empty are removed. The layout comes from `internal/ui/wq.vim`, embedded in the
binary and sourced with `vim -S`, so it only applies to these sessions.
`WQ_VIM` overrides the vim binary.

`strategy.md` starts with a small header that carries the node's title:

```markdown
---
title: 設計
---

(strategy)
```

Edit the `title:` line and the node is renamed when you leave vim (undoable
with `u`; a blank title is ignored). `board.json` stays the
source of truth: the header is rewritten with the current title every time
the node opens. Other `key: value` lines you add to the header are kept.

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
