" wq.vim — sourced by wq when a node opens its strategy.md, full screen.
"
" ZZ (or :wq) saves and returns to wq. q is left alone: a stray q, say at
" a hit-enter prompt, used to throw you out of vim. C-j / C-k save too and
" open the next / previous node in focus (F on the board), going round;
" a node opened from outside it joins the round until you are back on the
" board. ZF puts this node in focus or takes it out.
"
" wq tells this script, through the environment: WQ_FOCUS, the node's
" place in focus (0 when out of it); WQ_FOCUS_OTHERS, how many other
" nodes C-j / C-k can go to; WQ_FOCUS_FILE, where ZF leaves its answer
" (1 in focus, 0 out) for wq to pick up when vim ends.

let s:focus = str2nr($WQ_FOCUS)
let s:on = s:focus > 0
let s:others = str2nr($WQ_FOCUS_OTHERS)

function! WqFocusLabel() abort
  if !s:on
    return ''
  endif
  " The same ①, ②, … the board shows.
  return s:focus > 0 && s:focus <= 20 ? 'focus ' . nr2char(0x2460 + s:focus - 1) . ' · ' : 'focus · '
endfunction

" The exit codes tell wq which way to go; keep them in step with editor.go.
function! s:Step(code) abort
  if s:others == 0
    echo 'no other node in focus — ZF or F on the board adds one'
    return
  endif
  wall
  execute 'cquit' a:code
endfunction

function! s:ToggleFocus() abort
  if $WQ_FOCUS_FILE ==# ''
    return
  endif
  let s:on = !s:on
  let s:focus = 0 " its place is only known once wq has it
  call writefile([s:on ? '1' : '0'], $WQ_FOCUS_FILE)
  redrawstatus
  echo s:on ? 'in focus' : 'out of focus'
endfunction

nnoremap <silent> ZZ :wqa<CR>
nnoremap <silent> <C-j> :call <SID>Step(3)<CR>
nnoremap <silent> <C-k> :call <SID>Step(4)<CR>
nnoremap <silent> ZF :call <SID>ToggleFocus()<CR>
setlocal statusline=\ strategy%m%=%{WqFocusLabel()}C-j/C-k\ next/prev\ focus\ ·\ ZF\ focus\ on/off\ ·\ ZZ\ save\ &\ back\ 
