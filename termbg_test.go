//go:build darwin || linux

package main

import "testing"

func TestBackgroundIsDark(t *testing.T) {
	for _, c := range []struct {
		in       string
		dark, ok bool
	}{
		{"\x1b]11;rgb:3030/3434/3e3e\x1b\\\x1b[?62;22c", true, true},
		{"\x1b]11;rgb:ffff/ffff/ffff\x07", false, true},
		{"\x1b]11;rgb:fe/fe/fe\x1b\\", false, true},
		{"\x1b[?62;22c", false, false},
	} {
		dark, ok := backgroundIsDark([]byte(c.in))
		if dark != c.dark || ok != c.ok {
			t.Errorf("%q: got %v %v", c.in, dark, ok)
		}
	}
}
