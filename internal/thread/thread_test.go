package thread

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAddListTrashRestore(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 1, 15, 20, 3, 0, time.Local)
	a, _ := Add(dir, now, "ログ見たら500多発\nSELECT のあと急増\n")
	b, _ := Add(dir, now, "同じ秒の2件目\n")
	c, _ := Add(dir, now.Add(-time.Hour), "\n\n  先に書いたもの\n")
	got, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Path != c.Path || got[1].Path != a.Path || got[2].Path != b.Path {
		t.Fatalf("order: %v", got)
	}
	if got[0].Summary() != "先に書いたもの" || !got[1].Time.Equal(now) {
		t.Errorf("summary %q time %v", got[0].Summary(), got[1].Time)
	}
	trashed, err := Trash(dir, got[1])
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := List(dir); len(got) != 2 {
		t.Errorf("after trash: %d", len(got))
	}
	if err := Restore(dir, trashed); err != nil {
		t.Fatal(err)
	}
	if got, _ := List(dir); len(got) != 3 {
		t.Errorf("after restore: %d", len(got))
	}
}

func TestMigrateLegacyThread(t *testing.T) {
	dir := t.TempDir()
	legacy := "## 2026-10-01 14:03\n\nクエリ流した\n結果OK\n\n- 2026-10-01 16:45 Completed\n\n## 2026-10-01 17:00\n\n"
	os.WriteFile(filepath.Join(dir, "thread.md"), []byte(legacy), 0o644)
	got, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("entries: %+v", got)
	}
	if got[0].Body != "クエリ流した\n結果OK\n" || got[0].Time.Hour() != 14 {
		t.Errorf("first: %q %v", got[0].Body, got[0].Time)
	}
	if got[1].Body != "Completed\n" || got[1].Time.Minute() != 45 {
		t.Errorf("second: %q %v", got[1].Body, got[1].Time)
	}
	if _, err := os.Stat(filepath.Join(dir, "thread.md.migrated")); err != nil {
		t.Errorf("legacy file not kept: %v", err)
	}
}

func TestDropBlank(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.Local)
	Add(dir, now, "  \n\n")
	Add(dir, now.Add(time.Minute), "keep\n")
	if err := DropBlank(dir); err != nil {
		t.Fatal(err)
	}
	got, _ := List(dir)
	if len(got) != 1 || got[0].Summary() != "keep" {
		t.Errorf("got %+v", got)
	}
}
