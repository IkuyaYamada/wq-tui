package ime

import (
	"os"
	"runtime"
	"testing"
)

func init() { runtime.LockOSThread() }

func currentID() string {
	t := load()
	cur := t.current()
	defer t.release(cur)
	return t.id(cur)
}

// TestLiveSwitch really changes the input source, so it only runs when
// WQ_IME_LIVE=1. It restores the original source at the end.
func TestLiveSwitch(t *testing.T) {
	if os.Getenv("WQ_IME_LIVE") != "1" {
		t.Skip("set WQ_IME_LIVE=1 to switch the real input source")
	}
	if load() == nil {
		t.Fatal("could not bind Carbon")
	}
	orig := currentID()
	prev := System{}.ASCII()
	after := currentID()
	t.Logf("orig=%s prev=%q after=%s", orig, prev, after)
	System{}.Select(orig)
	restored := currentID()
	t.Logf("restored=%s", restored)
	if restored != orig {
		t.Errorf("restore failed: %s != %s", restored, orig)
	}
	if prev != "" && after == orig {
		t.Errorf("ASCII reported a switch but source is unchanged")
	}
}
