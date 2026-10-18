" wq.vim — sourced by wq when a node opens its strategy.md, full screen.
"
" ZZ (or :wq) saves and returns to wq. q is left alone: a stray q, say at
" a hit-enter prompt, used to throw you out of vim. C-j / C-k save too and
" open the next / previous node, as w / b would pick it on the board.

nnoremap <silent> ZZ :wqa<CR>
" The exit codes tell wq which way to go; keep them in step with editor.go.
nnoremap <silent> <C-j> :wa <Bar> cquit 3<CR>
nnoremap <silent> <C-k> :wa <Bar> cquit 4<CR>
setlocal statusline=\ strategy%m%=C-j/C-k\ next/prev\ ·\ ZZ\ save\ &\ back\
