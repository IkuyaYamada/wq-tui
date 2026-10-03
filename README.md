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
| `gg` / `G` | Top / last row with nodes |
| `gx` | Open the node's `url:` in the browser (nodes with a link show ↗ on their frame) |
| `a` / `n` | Add a node on the cursor cell (or the nearest empty cell if taken) |
| `o` / `O` | Insert a node in the free cell directly below / above, taking over the outgoing / incoming edges (A → B becomes A → new → B). Nothing else moves; if that cell is taken, off the board, or an edge would turn sideways, it reports an error instead |
| `i` | Rename (or edit `title:` in vim) |
| `m` | Move mode: `hjkl` slides to the next empty cell, `Enter` places, `Esc` cancels |
| `v` / `V` | Visual mode: select a block of cells / whole rows; `m` moves the selection together, `d` deletes it, `=` organizes it |
| `c` | Connect mode: `hjkl` picks a target, `Enter` toggles the edge (either end works; edges always point down) |
| `Space` | Complete: asks for a comment (`Enter` completes, `Esc` cancels), logs `Completed: <comment>` to the thread and shows the comment in green on the card's second line (the title shrinks to one line; the card keeps its size). On a done node it reopens right away (`Reopened`) and drops the comment |
| `Enter` | Open the node in vim (strategy, thread index, entry) |
| `x` | Delete the node under the cursor (A → B → C is bridged to A → C) |
| `dd` / `D` | Delete the cursor's row, nodes included, pulling the rows below up |
| `[` `Space` / `]` `Space` | Open an empty row above / below the cursor's row (rows below move down; the cursor stays on its node) |
| `R` | Reload board.json after another wq changed it |
| `u` / `Ctrl+r` | Undo / redo |
| `q` | Quit |

New nodes ask for a title right away; `Esc` on that prompt discards the node.

## Organizing

`=` on a visual selection tidies it up without touching any edge. Linked
nodes are re-layered from the selection's top row, each as high as its
predecessors allow (a child lands right under its parent, empty rows close
up), and every row is ordered by the mean column of the predecessors, which
lines chains up vertically and undoes crossings. Unlinked nodes in the
selection and everything outside it stay put. If a full row would push a node
level with or below one of its successors outside the selection, nothing
changes and an error is shown. One `u` undoes it.

## Several wq at once

Running wq in more than one terminal is safe. Each one remembers the
board.json it last read or wrote; when another wq has written it since, the
header shows `⟳ changed in another wq — R to reload` and every key that would
change the board is refused (moving around, `gx`, opening a node and `q` still
work). A save that would overwrite the other wq's change is not written
either. `R` loads the current board.json and clears the undo history, so undo
can never roll back the other wq's work. Thread entries and strategy files
are separate per node and are not affected; vim's own swap-file warning
covers the same file being open twice.

wq is meant to grow while in use, so an older build is often still open next
to a newer one. Fields in board.json that a build does not know are kept and
written back unchanged, so an older wq never strips what a newer one added.
(Builds from before this change do drop them; restart those once.)

## Rules

- Six columns, rows grow downward, one node per cell.
- Three empty rows always follow the lowest node, so there is room to walk
  into and drop new nodes.
- Edges always point to a lower row. Connecting two nodes on the same row
  pushes the target down a row; upward edges are refused.

## Opening a node

`Enter` on a node opens it straight in vim as two panes, cursor in the strategy on the
left, the thread on the right. The thread pane shows the entry index; `Enter`
opens an entry in that same pane and `Esc` (normal mode) saves it and goes
back to the index.

```
 strategy.md            │  10/01 14:03  クエリ流した
                        │▸ 10/01 15:20  ログ見たら500多発   ← Enter opens it here
                        │  10/01 16:43  Completed: スキーマ確定
```

| Key | Where | Action |
| --- | --- | --- |
| `Enter` / `o` | index | Open the entry under the cursor |
| `a` | index | New entry, in insert mode (never written if left untouched) |
| `D` | index | Delete the entry (after a prompt; moved to `thread/.trash/`) |
| `Esc` | entry | Save and return to the index (`<nowait>`, so an `<Esc><Esc>` map does not delay it) |
| `-` | anywhere | Show the index in the thread pane |
| `C-w w` | anywhere | Switch panes |
| `q` | anywhere | Save everything and return to the board |

Entries saved empty are removed on the way out. The layout comes from
`internal/ui/wq.vim`, embedded in the binary and sourced with `vim -S`, so it
only applies to these sessions. `WQ_VIM` overrides the vim binary.

`strategy.md` starts with a small header that carries the node's title and
a link, blank until you fill it in:

```markdown
---
title: 設計
url: https://example.com/design-doc
---

(strategy)
```

Edit `title:` or `url:` and the node picks them up when you leave vim, as one
undoable change (a blank title is ignored; a blank url clears the link).
`gx` on the board then opens the url (`https://` is added to a bare host). `board.json` stays the
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
