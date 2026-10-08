" wq.vim — sourced by wq when a node opens its strategy.md, full screen.
"
" q saves and returns to wq. C-j / C-k save too and open the next /
" previous node, as w / b would pick it on the board.

nnoremap <silent> q :wqa<CR>
" The exit codes tell wq which way to go; keep them in step with editor.go.
nnoremap <silent> <C-j> :wa <Bar> cquit 3<CR>
nnoremap <silent> <C-k> :wa <Bar> cquit 4<CR>
setlocal statusline=\ strategy%m%=C-j/C-k\ next/prev\ ·\ q\ save\ &\ back\
