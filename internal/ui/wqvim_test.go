package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestNodeVimScript drives wq.vim in a headless vim: the strategy alone on
// screen, and ZZ saving it.
func TestNodeVimScript(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim not installed")
	}
	dir := t.TempDir()
	strategy := filepath.Join(dir, "strategy.md")
	os.WriteFile(strategy, []byte("---\ntitle: 設計\n---\n\n方針\n"), 0o644)
	script := filepath.Join(dir, "wq.vim")
	os.WriteFile(script, nodeVimScript, 0o644)
	out := filepath.Join(dir, "out.txt")
	check := filepath.Join(dir, "check.vim")
	os.WriteFile(check, []byte(`redir! > `+out+`
echo "wins=" . winnr("$")
redir END
call setline(6, "追記")
normal ZZ
`), 0o644)
	if b, err := exec.Command("vim", "-N", "-u", "NONE", "-es", "-S", script, "-S", check, strategy).CombinedOutput(); err != nil {
		t.Fatalf("vim: %v\n%s", err, b)
	}
	if got, _ := os.ReadFile(out); string(got) != "\nwins=1" {
		t.Errorf("one window, the strategy: %q", got)
	}
	if body, _ := os.ReadFile(strategy); string(body) != "---\ntitle: 設計\n---\n\n方針\n追記\n" {
		t.Errorf("ZZ should save the strategy: %q", body)
	}
}

// runRound runs wq.vim headless on node start of A, B and C, each with
// its own strategy, going round round ("id:place" each), then check.vim's
// lines. It returns what wq would read back and each strategy's text.
func runRound(t *testing.T, start string, round []string, check string) (vimResult, map[string]string) {
	t.Helper()
	dir := t.TempDir()
	path := func(id string) string { return filepath.Join(dir, id, "strategy.md") }
	place := map[string]string{}
	var lines []string
	for _, r := range round {
		id, p, _ := strings.Cut(r, ":")
		place[id] = p
		lines = append(lines, id+"\t"+p+"\t"+path(id))
	}
	for _, id := range []string{"A", "B", "C"} {
		os.MkdirAll(filepath.Join(dir, id), 0o755)
		os.WriteFile(path(id), []byte(id+"\n"), 0o644)
	}
	roundFile, result := filepath.Join(dir, "round"), filepath.Join(dir, "result")
	os.WriteFile(roundFile, []byte(strings.Join(lines, "\n")), 0o644)
	script := filepath.Join(dir, "wq.vim")
	os.WriteFile(script, nodeVimScript, 0o644)
	checkPath := filepath.Join(dir, "check.vim")
	os.WriteFile(checkPath, []byte(check+"\nqall!\n"), 0o644)
	if place[start] == "" {
		place[start] = "0"
	}
	cmd := exec.Command("vim", "-N", "-u", "NONE", "-es", "-S", script, "-S", checkPath, path(start))
	cmd.Env = append(os.Environ(), "WQ_START="+start, "WQ_FOCUS="+place[start], "WQ_ROUND="+roundFile, "WQ_RESULT="+result)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("vim: %v\n%s", err, out)
	}
	bodies := map[string]string{}
	for _, id := range []string{"A", "B", "C"} {
		b, _ := os.ReadFile(path(id))
		bodies[id] = string(b)
	}
	return readVimResult(result), bodies
}

// TestNodeVimGoesRound checks that C-j / C-k open the next / previous
// node in the round inside vim, going round, saving the one left.
func TestNodeVimGoesRound(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim not installed")
	}
	r, bodies := runRound(t, "A", []string{"A:1", "C:2"}, `call setline(1, "A2")
execute "normal \<C-j>"
call setline(1, "C2")
execute "normal \<C-j>"
execute "normal \<C-k>"`)
	if r.last != "C" || !reflect.DeepEqual(r.visited, []string{"A", "C"}) {
		t.Errorf("A, C, A, C: last %q visited %v", r.last, r.visited)
	}
	if bodies["A"] != "A2\n" || bodies["C"] != "C2\n" || bodies["B"] != "B\n" {
		t.Errorf("each left node should be saved, B untouched: %q", bodies)
	}

	// A node from outside focus is in front: C-k from it goes to the last.
	r, _ = runRound(t, "B", []string{"B:0", "A:1", "C:2"}, `execute "normal \<C-k>"
execute "normal \<C-j>"`)
	if r.last != "B" || !reflect.DeepEqual(r.visited, []string{"B", "C"}) {
		t.Errorf("B, C, B: last %q visited %v", r.last, r.visited)
	}
}

// TestNodeVimDetour checks that a file opened with :e joins the round
// right after the node it was opened from, until vim ends, and that ZF
// leaves it alone.
func TestNodeVimDetour(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim not installed")
	}
	// A, :e a scratch file, C-j to C, C-k back to the file, ZF there,
	// C-k to A, C-k to C (round: A, file, C).
	r, _ := runRound(t, "A", []string{"A:1", "C:2"}, `execute 'edit' tempname()
call setline(1, "寄り道")
execute "normal \<C-j>"
execute "normal \<C-k>"
normal ZF
execute "normal \<C-k>"
execute "normal \<C-k>"`)
	// Had the file stayed out of the round, C-k from C would have gone to
	// A and ZF would have taken A out of focus.
	if r.last != "C" || !reflect.DeepEqual(r.visited, []string{"A", "C"}) || len(r.focus) != 0 {
		t.Errorf("last %q visited %v focus %+v", r.last, r.visited, r.focus)
	}

	// Ending on the detour leaves the board on the last node seen.
	r, _ = runRound(t, "A", []string{"A:1", "C:2"}, `execute "normal \<C-j>"
execute 'edit' tempname()`)
	if r.last != "C" {
		t.Errorf("ending on a detour: last %q, want C", r.last)
	}
}

// TestNodeVimReload checks that a strategy written by another process
// while in view is looked at again once keys stop.
func TestNodeVimReload(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim not installed")
	}
	_, bodies := runRound(t, "A", []string{"A:1", "C:2"}, `set autoread
call writefile(["A外"], expand('%:p'))
doautocmd CursorHold
call append('$', '見た')
write`)
	if bodies["A"] != "A外\n見た\n" {
		t.Errorf("A should be loaded again: %q", bodies["A"])
	}
}

// TestNodeVimNowhereToGo checks that C-j stays put with no other node in
// the round.
func TestNodeVimNowhereToGo(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim not installed")
	}
	r, _ := runRound(t, "A", []string{"A:1"}, `execute "normal \<C-j>"`)
	if r.last != "A" || !reflect.DeepEqual(r.visited, []string{"A"}) {
		t.Errorf("last %q visited %v", r.last, r.visited)
	}
}

// TestNodeVimZF checks that ZF records the node going in or out of focus,
// once per node however often it is pressed.
func TestNodeVimZF(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim not installed")
	}
	r, _ := runRound(t, "B", []string{"B:0", "A:1"}, `normal ZF
execute "normal \<C-j>"
normal ZF
normal ZFZF`)
	want := []focusChange{{"B", true}, {"A", false}}
	if !reflect.DeepEqual(r.focus, want) {
		t.Errorf("focus %+v, want %+v", r.focus, want)
	}
}
