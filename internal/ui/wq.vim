" wq.vim — sourced by wq when a node opens its strategy.md, full screen.
"
" ZZ (or :wq) saves and returns to wq. q is left alone: a stray q, say at
" a hit-enter prompt, used to throw you out of vim. C-j / C-k save and
" open the next / previous node in focus (F on the board), going round,
" without leaving vim; a node opened from outside focus is in the round
" too, in front. Any other file opened meanwhile (:e) joins the round as
" a detour, right after where you were, until vim ends. ZF puts the node
" in focus or takes it out; the board catches up once vim ends, the round
" here stays as it began.
"
" wq passes, through the environment: WQ_START, the id of the node vim
" opens on; WQ_FOCUS, its place in focus (0 when out of it); WQ_ROUND, a
" file of the round, one "id<Tab>place<Tab>path" a line; WQ_RESULT, a file
" to leave what happened in, for wq to read once vim ends (see
" readVimResult in editor.go).

set hidden " a node left with C-j keeps its undo history

let s:round = []
if $WQ_ROUND !=# '' && filereadable($WQ_ROUND)
  for s:line in readfile($WQ_ROUND)
    let s:f = split(s:line, "\t", 1)
    if len(s:f) >= 3
      call add(s:round, {'id': s:f[0], 'place': str2nr(s:f[1]), 'path': fnamemodify(join(s:f[2:], "\t"), ':p')})
    endif
  endfor
endif

let s:cur = $WQ_START  " the round entry in view: a node's id, or file:<path>
let s:last = s:cur     " the last node in view, for the board's cursor
let s:place = {s:cur : str2nr($WQ_FOCUS)} " id -> place in focus, 0 out, -1 in at an unknown place
let s:paths = {fnamemodify(expand('%'), ':p') : s:cur}
for s:e in s:round
  let s:place[s:e.id] = s:e.place
  let s:paths[s:e.path] = s:e.id
endfor
let s:visited = [s:cur]
let s:changes = [] " [id, on] in the order ZF was pressed

function! s:At() abort
  for i in range(len(s:round))
    if s:round[i].id ==# s:cur
      return i
    endif
  endfor
  return -1
endfunction

function! WqFocusLabel() abort
  let p = get(s:place, s:cur, 0)
  if p == 0
    return ''
  endif
  " The same ①, ②, … the board shows.
  return p > 0 && p <= 20 ? 'focus ' . nr2char(0x2460 + p - 1) . ' · ' : 'focus · '
endfunction

function! s:Status() abort
  let &l:statusline = ' strategy%m%=%{WqFocusLabel()}C-j/C-k next/prev focus · ZF focus on/off · ZZ save & back '
endfunction

" Written after every change, so it holds whichever way vim ends.
function! s:Save() abort
  if $WQ_RESULT ==# ''
    return
  endif
  let lines = ["last\t" . s:last]
  call extend(lines, map(copy(s:visited), '"visit\t" . v:val'))
  call extend(lines, map(copy(s:changes), '"focus\t" . v:val[0] . "\t" . v:val[1]'))
  call writefile(lines, $WQ_RESULT)
endfunction

" IsDetour reports whether id is a file visited on the way, not a node.
function! s:IsDetour(id) abort
  return a:id =~# '^file:'
endfunction

" A strategy or detour coming into view, by C-j / C-k or any other way
" (C-^, C-o, :e), makes it the current entry. A plain file not in the
" round yet goes in right after where you were; help, quickfix, a
" directory listing and the like stay out.
function! s:Enter() abort
  let path = fnamemodify(expand('%'), ':p')
  let id = get(s:paths, path, '')
  if id ==# ''
    if &buftype !=# '' || expand('%') ==# '' || isdirectory(path)
      call s:Status()
      return
    endif
    let id = 'file:' . path
    let s:paths[path] = id
    call insert(s:round, {'id': id, 'place': 0, 'path': path}, s:At() + 1)
  endif
  let s:cur = id
  if !s:IsDetour(id)
    let s:last = id
    if index(s:visited, id) < 0
      call add(s:visited, id)
    endif
  endif
  call s:Save()
  call s:Status()
endfunction

function! s:Step(dir) abort
  let n = len(s:round)
  let at = s:At()
  if n - (at >= 0 ? 1 : 0) <= 0
    echo 'no other node in focus — ZF or F on the board adds one'
    return
  endif
  let next = at < 0 ? (a:dir > 0 ? 0 : n - 1) : (at + a:dir + n) % n
  wall
  execute 'edit' fnameescape(s:round[next].path)
endfunction

function! s:ToggleFocus() abort
  if s:IsDetour(s:cur)
    echo 'not a node: only nodes go in focus'
    return
  endif
  let on = get(s:place, s:cur, 0) == 0
  let s:place[s:cur] = on ? -1 : 0 " its place is only known once wq has it
  call filter(s:changes, 'v:val[0] !=# s:cur')
  call add(s:changes, [s:cur, on ? 1 : 0])
  call s:Save()
  redrawstatus
  echo on ? 'in focus' : 'out of focus'
endfunction

augroup wq
  autocmd!
  autocmd BufEnter * call s:Enter()
augroup END

nnoremap <silent> ZZ :wqa<CR>
nnoremap <silent> <C-j> :call <SID>Step(1)<CR>
nnoremap <silent> <C-k> :call <SID>Step(-1)<CR>
nnoremap <silent> ZF :call <SID>ToggleFocus()<CR>
call s:Status()
call s:Save()
