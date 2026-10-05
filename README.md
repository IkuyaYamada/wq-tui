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

### Windows

Run it in WSL (Ubuntu or similar), from Windows Terminal:

```sh
sudo apt install -y git vim curl
curl https://mise.run | sh          # then follow its hint to activate mise
git clone https://github.com/IkuyaYamada/wq-tui.git && cd wq-tui
mise trust && mise install           # apt's Go is too old for go.mod
go build -o wq . && ./wq
```

Keep the data in the Linux home (`~/wq`), not under `/mnt/c`: it is much
faster and the atomic save of board.json behaves as on macOS. `gx` opens the
link in the Windows browser. Switching the input method to ASCII on the board
is macOS only, so turn the Japanese IME off yourself before using board keys
(full-width ｈｊｋｌ are still accepted).

A native build also works (`winget install GoLang.Go Git.Git vim.vim`, then
`go build -o wq.exe .` in PowerShell), but is untested; an exe built on the
machine itself does not trigger the SmartScreen warning a downloaded one does.

Data lives in `~/wq` (override with `WQ_DIR`):

```
~/wq/board.json                 # nodes (title, row, col, done) and edges
~/wq/nodes/<id>/strategy.md
~/wq/nodes/<id>/thread/20261001-152003.md   # one file per thread entry
```

Plain files, so the directory can be a git repo of its own; see
[Backup and other machines](#backup-and-other-machines).

## Keys

| Key | Action |
| --- | --- |
| `hjkl` / arrows | Move the cursor one cell (empty cells included) |
| `w` / `b` | Jump to the next / previous node |
| `Ctrl+d` / `Ctrl+u` | Scroll half a screen down / up, moving the cursor with it |
| `Ctrl+e` / `Ctrl+y` | Scroll one row down / up; the cursor stays unless it would leave the screen |
| `zz` / `zt` / `zb` | Put the cursor's row at the middle / top / bottom of the screen |
| `gg` / `G` | The topmost node not yet done (leftmost first; the top-left cell when all are done) / last row with nodes |
| `gx` | Open the node's `url:` in the browser (nodes with a link show ↗ on their frame) |
| `a` / `n` | Add a node on the cursor cell (or the nearest empty cell if taken) |
| `o` / `O` | Insert a node directly below / above, taking over the outgoing / incoming edges (A → B becomes A → new → B). A free cell is used as is; if it is taken, off the board, or an edge would turn sideways, an empty row is opened first (like `]` / `[` `Space`) |
| `i` | Rename (or edit `title:` in vim) |
| `m` | Move mode: `hjkl` slides to the next empty cell, `Enter` places, `Esc` cancels |
| `yy` / `p` | Copy the node under the cursor: a dashed ghost then shows where it would go (the empty cell nearest the cursor) and follows the cursor; `p` pastes a copy there (title, url and strategy; open, no edges, its own thread) and moves onto it. The copy stays held for more `p`; `Esc` lets it go |
| `v` / `V` | Visual mode: select a block of cells / whole rows; `m` moves the selection together, `d` deletes it, `=` organizes it |
| `c` | Connect mode: `hjkl` picks a target, `Enter` toggles the edge (either end works; edges always point down) |
| `Space` | Complete: asks for a comment (`Enter` completes, `Esc` cancels), logs `Completed: <comment>` to the thread and shows the comment in green on the card's second line (the title shrinks to one line; the card keeps its size). On a done node it reopens right away (`Reopened`) and drops the comment |
| `Enter` | Open the node in vim (strategy, thread index, entry) |
| `K` | Preview the node beside its card: strategy, then the thread oldest first, read-only. It follows the cursor; `Ctrl+d` / `Ctrl+u` scroll it, `K` / `Esc` close it. For quick fixes without vim, `i` edits the strategy body in place and `a` writes a new thread entry: `Enter` is a new line, `Esc` (or `Ctrl+s`) saves and closes, `Ctrl+c` throws the changes away (asking once if anything changed). Older entries are edited in vim |
| `x` | Delete the node under the cursor (A → B → C is bridged to A → C) |
| `dd` / `D` | Delete the cursor's row, nodes included, pulling the rows below up |
| `[` `Space` / `]` `Space` | Open an empty row above / below the cursor's row (rows below move down; the cursor stays on its node) |
| `-` | Draw a session break under the cursor's row, with an optional label ("今日はここまで"); `-` on a row that has one removes it |
| `M` | Pick up the session break under the cursor's row: `j` / `k` move it to another gap (hopping over taken ones), `i` edits its label, `x` deletes it, `Enter` places it, `Esc` puts it back |
| `R` | Reload board.json after another wq changed it |
| `u` / `Ctrl+r` | Undo / redo |
| `q` | Quit |

New nodes ask for a title right away; `Esc` on that prompt discards the node.

## Session breaks

A session break is a dotted line in the gap under a row — a stopping point
between phases ("break here", "done for today"). It lives in board.json as
`breaks`, never moves nodes or constrains edges, and stays in its gap when
rows are opened or deleted, and `M` moves it to another gap or relabels it.
A row opened next to a row stays on that row's side of a break: `o` and
`] Space` push a break under the row down along with the rows below, `O` and
`[ Space` leave a break over the row where it is, and `o` / `O` never drop the
new node into a free cell across a break.
The break gets a line of its own below the edge lane, so edges only cross
it straight down and never run along it; the label is placed where no edge
crosses.

## Frames

A frame is a line drawn around some nodes to mark them as one meaningful
cluster, say the pieces a node that grew too big was broken into. It has a
title and, like a node, its own strategy and thread (`nodes/<id>/`). Frames
live in board.json as `groups` and do not nest.

```
╭─ 設計 ───────────────── 1/3 ─╮
│╭──────────╮  ╭──────────╮    │
││ スキーマ │  │ API      │    │
│╰──────────╯  ╰──────────╯    │
│╭──────────╮                  │
││ 移行     │                  │
│╰──────────╯                  │
╰──────────────────────────────╯
```

| Key | Action |
| --- | --- |
| `v` / `V` … `g` | Frame the selection; asks for a title (`Esc` gives up) |
| `v` / `V` … `u` | Take the selection out of its frames; an empty frame disappears |
| `gs` | Split the node into a frame: the frame keeps its title, url and notes, a first child takes its cell and edges and asks for a title (`Esc` undoes the split) |
| `k` on a frame's top row | Select the frame (bold); `j` goes back in, `k` again leaves upward |

With a frame selected: `m` moves its members together, `x` removes the frame
(members stay), `Space` completes every member (or reopens them all when
all are done), `i` renames it, `K` previews and `Enter` opens its own
notes in vim. Edges stay between nodes: `c` does nothing on a frame.
The header line names where the cursor is in full, frame › node, as frame
lines and cards cut long titles short.

A frame is the smallest rectangle around its members and follows them as
they move; it never adds columns or constrains moves and edges. `o` / `O`
from a member, and `a` on a member or an empty cell inside a frame, add the
new node to that frame. Edges cross frames but never run along them. A node
that is not a member can still sit inside a frame's rectangle: move it out,
or frame it too.

## Maturity meters

Two small bars on the left end of a card's bottom border show how much has
been written on it, read when wq starts, on `R` and when vim closes:

```
╰▄▆──────────────╯   strategy ▄ , thread ▆
```

| Bar | Strategy (non-space characters, header excluded) | Thread (entries) |
| --- | --- | --- |
| none | 0 | 0 |
| `▂` | 1–99 | 1 |
| `▄` | 100–399 | 2–4 |
| `▆` | 400–999 | 5–9 |
| `█` | 1000+ | 10+ |

The Completed / Reopened lines wq logs itself do not count. `K` gives the
exact numbers at the top of the preview.

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

## Backup and other machines

Everything wq knows is in the data directory (`~/wq`, or `WQ_DIR`):
`board.json` and `nodes/`. Nothing else needs copying; the vim layout script
in the user cache directory is rewritten every time a node opens. Copy the
directory and you have a backup; share it and another machine picks up where
you left off.

A private git repository works well: the files are small plain text, so
history and diffs are readable. The board is your own work, so keep the
repository **private**.

First machine, once:

```sh
cd ~/wq
git init
printf 'board.json.tmp\n.DS_Store\n' > .gitignore   # board.json.tmp only exists mid-save
git add -A && git commit -m "wq board"
gh repo create wq-data --private --source=. --push
```

Another machine, once: build wq as in [Run](#run), then

```sh
gh repo clone <you>/wq-data ~/wq   # or anywhere, with WQ_DIR pointing there
```

and set the same `WQ_VIM` / `WQ_IME` there if you use them.

Each session, on whichever machine:

```sh
cd ~/wq && git pull           # before starting wq
wq
cd ~/wq && git add -A && git commit -m sync && git push   # after quitting it
```

Work on one machine at a time. The `⟳ changed in another wq` check only sees
other wq processes on the same disk; it cannot see a commit waiting on
another machine. If you forget to pull and both sides changed `board.json`,
git reports a conflict on pull: keep one side (`git checkout --theirs
board.json` or `--ours`) and redo the few moves by hand, since a hand-merged
board.json can easily end up with two nodes in one cell. Strategy and thread
files rarely conflict, as each entry is its own file.

A synced folder (iCloud Drive, Dropbox) also works for one person, but on a
conflict it quietly writes a "conflicted copy" of board.json that wq never
reads, so changes can go missing unnoticed; git makes the conflict visible.

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
back to the index. This is the one place `Esc` keeps what you typed: on the
board's prompts (`title>`, `done>`, `break>`) it cancels. The status lines
say which.

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
| `C-j` / `C-k` | anywhere | Save everything and open the next / previous node (the one `w` / `b` would pick) |

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

## Pushing (private data check)

This repository is public, while wq's data lives in `~/wq`. A pre-push hook
in `.githooks/` keeps the two apart. Enable it once per clone:

```sh
git config core.hooksPath .githooks
```

On every push it looks at the added lines and commit messages being pushed:

1. a mechanical pass for text taken from the wq data directory (titles,
   completion comments, break labels, urls, strategy and thread lines), local
   home paths, e-mail addresses and credential-looking strings;
2. a review by `claude -p` (sonnet, no tools) that answers OK or NG for
   anything that reads like private information.

Either one stops the push with the reason. `WQ_SKIP_REVIEW=1 git push` skips
the Claude review; `git push --no-verify` skips both.
`.githooks/pre-push --check origin/main..HEAD` runs the check without pushing.

