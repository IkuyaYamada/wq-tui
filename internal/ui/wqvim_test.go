package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestNodeVimScript drives wq.vim in a headless vim: the strategy alone on
// screen, and q saving it.
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
normal q
`), 0o644)
	if b, err := exec.Command("vim", "-N", "-u", "NONE", "-es", "-S", script, "-S", check, strategy).CombinedOutput(); err != nil {
		t.Fatalf("vim: %v\n%s", err, b)
	}
	if got, _ := os.ReadFile(out); string(got) != "\nwins=1" {
		t.Errorf("one window, the strategy: %q", got)
	}
	if body, _ := os.ReadFile(strategy); string(body) != "---\ntitle: 設計\n---\n\n方針\n追記\n" {
		t.Errorf("q should save the strategy: %q", body)
	}
}

// TestNodeVimCtrlJK checks that C-j / C-k save and leave vim with the exit
// codes wq reads as next / previous node.
func TestNodeVimCtrlJK(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim not installed")
	}
	for key, want := range map[string]int{"<C-j>": 1, "<C-k>": -1} {
		dir := t.TempDir()
		strategy := filepath.Join(dir, "strategy.md")
		os.WriteFile(strategy, []byte("方針\n"), 0o644)
		script := filepath.Join(dir, "wq.vim")
		os.WriteFile(script, nodeVimScript, 0o644)
		check := filepath.Join(dir, "check.vim")
		os.WriteFile(check, []byte(`call setline(1, "書き換え")
execute "normal \`+key+`"
`), 0o644)
		err := exec.Command("vim", "-N", "-u", "NONE", "-es", "-S", script, "-S", check, strategy).Run()
		step, err := exitStep(err)
		if err != nil || step != want {
			t.Errorf("%s: step %d err %v, want %d", key, step, err, want)
		}
		if body, _ := os.ReadFile(strategy); string(body) != "書き換え\n" {
			t.Errorf("%s should save the strategy: %q", key, body)
		}
	}
}
