# wq-tui

A keyboard-driven six-column board for the terminal. Everything you are
working on lives on one grid; each node opens in vim as a **strategy** (left)
and a free-form **thread** (right).

Successor to the web-based `workflow-queue`, keeping its grid rules and keymap.

## Run

```sh
mise install        # Go toolchain (see mise.toml)
go build -o wq . && ./wq
```

Data lives in `~/wq` (override with `WQ_DIR`):

```
~/wq/board.json                 # nodes (title, row, col, done) and edges
~/wq/nodes/<id>/strategy.md
~/wq/nodes/<id>/thread.md
```

Plain files, so the directory can be a git repo of its own.

## Keys

| Key | Action |
| --- | --- |
| `hjkl` / arrows | Move the cursor one cell (empty cells included) |
| `w` / `b` | Jump to the next / previous node |
| `g` / `G` | Top / last row with nodes |
| `a` / `n` | Add a node on the cursor cell (or the nearest empty cell if taken) |
| `o` / `O` | Insert a node below / above, taking over the outgoing / incoming edges (A → B becomes A → new → B) |
| `i` | Rename |
| `m` | Move mode: `hjkl` slides to the next empty cell, `Enter` places, `Esc` cancels |
| `c` | Connect mode: `hjkl` picks a target, `Enter` toggles the edge |
| `Space` | Toggle done (logged to the thread) |
| `Enter` | Open in vim |
| `d` / `x` | Delete (A → B → C is bridged to A → C) |
| `u` / `Ctrl+r` | Undo / redo |
| `q` | Quit |

New nodes ask for a title right away; `Esc` on that prompt discards the node.

## Rules

- Six columns, rows grow downward, one node per cell.
- Three empty rows always follow the lowest node, so there is room to walk
  into and drop new nodes.
- Edges always point to a lower row. Connecting two nodes on the same row
  pushes the target down a row; upward edges are refused.

## Editing in vim

`Enter` runs `vim -O strategy.md thread.md` (set `WQ_VIM` to use another
vim-compatible binary). A timestamp heading is appended to the thread and the
cursor starts under it; the heading is removed again if you write nothing.
For this session only, `q` in normal mode saves both files and returns.
