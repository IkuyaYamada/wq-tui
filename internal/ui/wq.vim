" wq.vim — sourced by wq when a node opens. It lays the node out as
"
"   strategy.md │ thread index, one line per entry
"               ├────────────────────────────────
"               │ the entry picked in the index
"
" C-w w cycles through the three windows; q saves everything and returns to
" wq. g:wq_thread_dir (the node's thread/ directory) must be set first.

if !exists('g:wq_thread_dir')
  finish
endif
let s:dir = fnamemodify(g:wq_thread_dir, ':p:s?/$??')
set hidden
nnoremap <silent> q :wqa<CR>

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

function! s:current() abort
  let nr = win_id2win(s:entry_win)
  return nr ? fnamemodify(bufname(winbufnr(nr)), ':p') : ''
endfunction

function! s:render() abort
  let s:files = s:entries()
  let cur = s:current()
  let lines = map(copy(s:files), {_, f -> (f ==# cur ? '▸ ' : '  ') . s:label(f)})
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

" open shows path in the entry window and moves there. A new entry is not
" written until saved, so an untouched one never reaches the disk.
function! s:open(path, insert) abort
  call win_gotoid(s:entry_win)
  silent execute 'edit ' . fnameescape(a:path)
  call s:render()
  if a:insert
    startinsert
  endif
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
  if s:current() ==# fnamemodify(f, ':p')
    let rest = s:entries()
    call s:open(empty(rest) ? s:newname() : rest[-1], 0)
  endif
  silent! execute 'bwipeout! ' . bufnr(f)
  call win_gotoid(s:index_win)
  call s:render()
endfunction

" ── layout ───────────────────────────────────────────────────────────────
let s:strategy_win = win_getid()
setlocal statusline=\ strategy%=%m\

rightbelow vnew
let s:index = bufnr('%')
let s:index_win = win_getid()
silent file [thread]
setlocal buftype=nofile bufhidden=wipe noswapfile nobuflisted
setlocal nonumber norelativenumber nowrap cursorline nomodifiable winfixheight
setlocal statusline=\ thread\ \ ⏎\ open\ ·\ a\ new\ ·\ D\ delete\ ·\ q\ back
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

belowright new
let s:entry_win = win_getid()
let s:files = s:entries()
call s:open(empty(s:files) ? s:newname() : s:files[-1], 0)

call win_gotoid(s:strategy_win)
execute 'vertical resize ' . (&columns * 2 / 5)
call win_gotoid(s:index_win)
execute 'resize ' . max([3, min([len(s:files) + 1, &lines / 3])])
normal! G

augroup wq
  autocmd!
  execute 'autocmd BufWritePost,BufEnter ' . fnameescape(s:dir) . '/*.md call s:render()'
augroup END
