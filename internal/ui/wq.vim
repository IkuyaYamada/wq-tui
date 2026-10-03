" wq.vim — sourced by wq when a node opens. Two panes:
"
"   strategy.md │ thread: the entry index, or one entry
"
" In the index, Enter opens the entry under the cursor in the same pane and
" Esc in that entry (normal mode) saves it and comes back to the index. a adds
" an entry, D deletes one, - shows the index from anywhere, C-w w switches
" panes, and q saves everything and returns to wq. g:wq_thread_dir (the
" node's thread/ directory) must be set first.

if !exists('g:wq_thread_dir')
  finish
endif
let s:dir = fnamemodify(g:wq_thread_dir, ':p:s?/$??')
let s:last = ''  " entry most recently opened; marked ▸ in the index
set hidden
nnoremap <silent> q :wqa<CR>
nnoremap <silent> - :call <SID>show_index()<CR>

" Entries sort by file stem so "…-152003" comes before "…-152003-02".
function! s:bystem(a, b) abort
  let [x, y] = [fnamemodify(a:a, ':t:r'), fnamemodify(a:b, ':t:r')]
  return x ==# y ? 0 : x ># y ? 1 : -1
endfunction

function! s:entries() abort
  return sort(glob(s:dir . '/*.md', 1, 1), function('s:bystem'))
endfunction

" 20261001-152003.md → "10/01 15:20  <first non-blank line>"
function! s:label(path) abort
  let n = fnamemodify(a:path, ':t:r')
  let when = n[4:5] . '/' . n[6:7] . ' ' . n[9:10] . ':' . n[11:12]
  let text = filereadable(a:path) ? filter(readfile(a:path, '', 50), 'v:val =~# ''\S''') : []
  return when . '  ' . (empty(text) ? '…' : trim(text[0]))
endfunction

function! s:render() abort
  let s:files = s:entries()
  let lines = map(copy(s:files), {_, f -> (f ==# s:last ? '▸ ' : '  ') . s:label(f)})
  if empty(lines)
    let lines = ['  no entries yet — a to add one']
  endif
  call setbufvar(s:index, '&modifiable', 1)
  silent call deletebufline(s:index, 1, '$')
  call setbufline(s:index, 1, lines)
  call setbufvar(s:index, '&modifiable', 0)
  call setbufvar(s:index, '&modified', 0)
endfunction

function! s:newname() abort
  let base = s:dir . '/' . strftime('%Y%m%d-%H%M%S')
  let path = base . '.md'
  let i = 2
  while filereadable(path) || bufexists(path)
    let path = printf('%s-%02d.md', base, i)
    let i += 1
  endwhile
  return path
endfunction

" goto_thread_win moves to the right pane, reopening it beside the strategy
" if it was closed.
function! s:goto_thread_win() abort
  if !win_id2win(s:thread_win)
    if win_id2win(s:strategy_win)
      call win_gotoid(s:strategy_win)
    endif
    rightbelow vsplit
    let s:thread_win = win_getid()
    call s:style_thread_window()
  endif
  call win_gotoid(s:thread_win)
endfunction

function! s:style_thread_window() abort
  setlocal nonumber norelativenumber cursorline
  setlocal statusline=%{WqThreadStatus()}
endfunction

function! WqThreadStatus() abort
  if bufnr('%') == s:index
    return ' thread   ⏎ open · a new · D delete · q done'
  endif
  return ' ' . s:label(expand('%:p')) . '   esc index · q done'
endfunction

" show_index puts the index in the right pane, cursor on the last entry
" opened (or the newest).
function! s:show_index() abort
  call s:goto_thread_win()
  execute 'buffer ' . s:index
  call s:render()
  let i = index(s:files, s:last)
  call cursor(i >= 0 ? i + 1 : line('$'), 1)
endfunction

" open shows an entry in the right pane. A new entry is not written until
" saved, so an untouched one never reaches the disk.
function! s:open(path, insert) abort
  call s:goto_thread_win()
  silent execute 'edit ' . fnameescape(a:path)
  let s:last = fnamemodify(a:path, ':p')
  " <nowait>: do not wait to see whether this Esc starts a longer global
  " mapping such as <Esc><Esc>.
  nnoremap <buffer> <nowait> <silent> <Esc> :call <SID>back()<CR>
  if a:insert
    startinsert
  endif
endfunction

function! s:back() abort
  if &modified
    silent update
  endif
  call s:show_index()
endfunction

function! s:open_under_cursor() abort
  let i = line('.') - 1
  if i < len(s:files)
    call s:open(s:files[i], 0)
  endif
endfunction

function! s:trash_under_cursor() abort
  let i = line('.') - 1
  if i >= len(s:files)
    return
  endif
  let f = s:files[i]
  if confirm('Delete "' . s:label(f) . '"?', "&Yes\n&No", 2) != 1
    return
  endif
  call mkdir(s:dir . '/.trash', 'p')
  call rename(f, s:dir . '/.trash/' . fnamemodify(f, ':t'))
  silent! execute 'bwipeout! ' . bufnr(f)
  if s:last ==# f
    let s:last = ''
  endif
  call s:render()
  call cursor(min([i + 1, line('$')]), 1)
endfunction

" ── layout ───────────────────────────────────────────────────────────────
let s:strategy_win = win_getid()
setlocal statusline=\ strategy%=%m

rightbelow vnew
let s:thread_win = win_getid()
let s:index = bufnr('%')
silent file [thread]
setlocal buftype=nofile bufhidden=hide noswapfile nobuflisted nowrap nomodifiable
call s:style_thread_window()
syntax match wqWhen /\d\d\/\d\d \d\d:\d\d/
syntax match wqMark /^▸/
syntax match wqLog /\v  \zs(Completed|Reopened)/
syntax match wqHint /^  no entries yet.*/
highlight default link wqWhen Comment
highlight default link wqMark Title
highlight default link wqLog Special
highlight default link wqHint Comment
nnoremap <buffer> <silent> <CR> :call <SID>open_under_cursor()<CR>
nnoremap <buffer> <silent> o :call <SID>open_under_cursor()<CR>
nnoremap <buffer> <silent> a :call <SID>open(<SID>newname(), 1)<CR>
nnoremap <buffer> <silent> D :call <SID>trash_under_cursor()<CR>

call win_gotoid(s:strategy_win)
execute 'vertical resize ' . (&columns / 2)
call s:show_index()
" Start in the strategy; the index keeps its cursor on the newest entry.
call win_gotoid(s:strategy_win)

augroup wq
  autocmd!
  execute 'autocmd BufWritePost ' . fnameescape(s:dir) . '/*.md call s:render()'
augroup END
