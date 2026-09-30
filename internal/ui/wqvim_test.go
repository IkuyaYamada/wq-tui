package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNodeVimScript drives wq.vim in a headless vim: layout, picking an
// entry from the index, adding one with a, and q saving it.
func TestNodeVimScript(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim not installed")
	}
	dir := t.TempDir()
	threadDir := filepath.Join(dir, "thread")
	os.MkdirAll(threadDir, 0o755)
	strategy := filepath.Join(dir, "strategy.md")
	os.WriteFile(strategy, []byte("---\ntitle: 設計\n---\n\n方針\n"), 0o644)
	os.WriteFile(filepath.Join(threadDir, "20261001-140300.md"), []byte("クエリ流した\n"), 0o644)
	os.WriteFile(filepath.Join(threadDir, "20261001-152000.md"), []byte("\nログ見たら500多発\n"), 0o644)

	script := filepath.Join(dir, "wq.vim")
	os.WriteFile(script, nodeVimScript, 0o644)
	out := filepath.Join(dir, "out.txt")
	check := filepath.Join(dir, "check.vim")
	os.WriteFile(check, []byte(`redir! > `+out+`
echo "wins=" . winnr("$") . " cur=" . bufname("%")
echo join(getbufline(bufnr("[thread]"), 1, "$"), "|")
normal gg
execute "normal \<CR>"
echo "opened=" . fnamemodify(bufname("%"), ":t")
echo join(getbufline(bufnr("[thread]"), 1, "$"), "|")
wincmd k
normal a
call setline(1, "新しいメモ")
redir END
normal q
`), 0o644)

	cmd := exec.Command("vim", "-N", "-u", "NONE", "-es",
		"--cmd", "let g:wq_thread_dir = "+vimString(threadDir),
		"-S", script, "-S", check, strategy)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("vim: %v\n%s", err, b)
	}
	got, _ := os.ReadFile(out)
	// :silent hides file messages on screen but :redir still records them.
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(string(got)), "\n") {
		if l != "" && !strings.HasPrefix(l, `"`) {
			lines = append(lines, l)
		}
	}
	want := []string{
		"wins=3 cur=[thread]",
		"  10/01 14:03  クエリ流した|▸ 10/01 15:20  ログ見たら500多発",
		"opened=20261001-140300.md",
		"▸ 10/01 14:03  クエリ流した|  10/01 15:20  ログ見たら500多発",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	files, _ := filepath.Glob(filepath.Join(threadDir, "*.md"))
	if len(files) != 3 {
		t.Fatalf("new entry not saved by q: %v", files)
	}
	for _, f := range files {
		if name := filepath.Base(f); name == "20261001-140300.md" || name == "20261001-152000.md" {
			continue
		}
		if body, _ := os.ReadFile(f); !strings.Contains(string(body), "新しいメモ") {
			t.Errorf("new entry %s body %q", filepath.Base(f), body)
		}
	}
}

func TestVimString(t *testing.T) {
	if got := vimString("/a/it's"); got != "'/a/it''s'" {
		t.Errorf("got %s", got)
	}
}
