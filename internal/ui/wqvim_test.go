package ui

import (
	"os"
	"os/exec"
	"path/filepath"
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

// runNodeVim runs wq.vim headless on a strategy, with check.vim's lines
// after it and env set, and returns vim's exit status and the strategy.
func runNodeVim(t *testing.T, check string, env ...string) (error, string) {
	t.Helper()
	dir := t.TempDir()
	strategy := filepath.Join(dir, "strategy.md")
	os.WriteFile(strategy, []byte("方針\n"), 0o644)
	script := filepath.Join(dir, "wq.vim")
	os.WriteFile(script, nodeVimScript, 0o644)
	checkPath := filepath.Join(dir, "check.vim")
	os.WriteFile(checkPath, []byte(check), 0o644)
	cmd := exec.Command("vim", "-N", "-u", "NONE", "-es", "-S", script, "-S", checkPath, strategy)
	cmd.Env = append(os.Environ(), env...)
	err := cmd.Run()
	body, _ := os.ReadFile(strategy)
	return err, string(body)
}

// TestNodeVimCtrlJK checks that C-j / C-k save and leave vim with the exit
// codes wq reads as next / previous node, when focus has somewhere to go.
func TestNodeVimCtrlJK(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim not installed")
	}
	for key, want := range map[string]int{"<C-j>": 1, "<C-k>": -1} {
		err, body := runNodeVim(t, `call setline(1, "書き換え")
execute "normal \`+key+`"
`, "WQ_FOCUS_OTHERS=1")
		step, err := exitStep(err)
		if err != nil || step != want {
			t.Errorf("%s: step %d err %v, want %d", key, step, err, want)
		}
		if body != "書き換え\n" {
			t.Errorf("%s should save the strategy: %q", key, body)
		}
	}
}

// TestNodeVimCtrlJKNowhere checks that C-j stays in vim when no other node
// is in focus.
func TestNodeVimCtrlJKNowhere(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim not installed")
	}
	err, _ := runNodeVim(t, `execute "normal \<C-j>"
qall!
`, "WQ_FOCUS_OTHERS=0")
	if step, err := exitStep(err); step != 0 || err != nil {
		t.Errorf("C-j should do nothing: step %d err %v", step, err)
	}
}

// TestNodeVimZF checks that ZF leaves its answer where wq reads it.
func TestNodeVimZF(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim not installed")
	}
	for _, c := range []struct {
		place string
		keys  string
		want  bool
	}{{"0", "ZF", true}, {"2", "ZF", false}, {"0", "ZFZF", false}} {
		ff := filepath.Join(t.TempDir(), "focus")
		_, _ = runNodeVim(t, "normal "+c.keys+"\nqall!\n", "WQ_FOCUS="+c.place, "WQ_FOCUS_FILE="+ff)
		got := readFocusFile(ff)
		if got == nil || *got != c.want {
			t.Errorf("place %s, %s: got %v, want %v", c.place, c.keys, got, c.want)
		}
		if _, err := os.Stat(ff); err == nil {
			t.Errorf("reading should clear the file")
		}
	}
}
