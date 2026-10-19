//go:build darwin || linux

package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/unix"
)

// tmuxBackgroundIsDark asks tmux for the background colour and tells
// whether it is dark. Lip Gloss does not ask inside tmux and takes the
// background as black.
//
// tmux answers from the colour it learned from the outer terminal, which
// goes stale when that terminal's theme changes without telling it (Ghostty
// after its config is reloaded). So the outer terminal is asked first
// through tmux (allow-passthrough must be on): its answer goes to tmux,
// which keeps the new colour.
func tmuxBackgroundIsDark() (dark, ok bool) {
	if os.Getenv("TMUX") == "" {
		return false, false
	}
	fd := int(os.Stdin.Fd())
	st, err := term.MakeRaw(uintptr(fd))
	if err != nil {
		return false, false
	}
	defer term.Restore(uintptr(fd), st) //nolint:errcheck

	fmt.Fprint(os.Stdout, "\x1bPtmux;\x1b\x1b]11;?\x1b\x1b\\\x1b\\")
	time.Sleep(100 * time.Millisecond)
	// The device attributes query, which every terminal answers, ends the wait.
	fmt.Fprint(os.Stdout, "\x1b]11;?\x1b\\\x1b[c")
	got := readUntilDA(fd, 500*time.Millisecond)
	return backgroundIsDark(got)
}

// readUntilDA reads the terminal's answers until the device attributes
// reply (CSI ? … c) or the timeout.
func readUntilDA(fd int, timeout time.Duration) []byte {
	var got []byte
	buf := make([]byte, 128)
	deadline := time.Now().Add(timeout)
	for {
		if i := bytes.LastIndex(got, []byte("\x1b[?")); i >= 0 && bytes.IndexByte(got[i:], 'c') >= 0 {
			return got
		}
		left := time.Until(deadline)
		if left <= 0 {
			return got
		}
		var fds unix.FdSet
		fds.Set(fd)
		tv := unix.NsecToTimeval(left.Nanoseconds())
		if n, err := unix.Select(fd+1, &fds, nil, nil, &tv); err != nil || n == 0 {
			return got
		}
		n, err := unix.Read(fd, buf)
		if err != nil || n == 0 {
			return got
		}
		got = append(got, buf[:n]...)
	}
}

// backgroundIsDark reads an OSC 11 answer (ESC ] 11 ; rgb:RRRR/GGGG/BBBB).
func backgroundIsDark(b []byte) (dark, ok bool) {
	i := bytes.Index(b, []byte("\x1b]11;rgb:"))
	if i < 0 {
		return false, false
	}
	rest := string(b[i+len("\x1b]11;rgb:"):])
	if j := strings.IndexAny(rest, "\x1b\x07"); j >= 0 {
		rest = rest[:j]
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 3 {
		return false, false
	}
	var rgb [3]float64
	for k, p := range parts {
		v, err := strconv.ParseUint(p, 16, 32)
		if err != nil || len(p) == 0 || len(p) > 4 {
			return false, false
		}
		rgb[k] = float64(v) / float64(uint64(1)<<(4*len(p))-1)
	}
	// Relative luminance, as Lip Gloss's lightness test near enough.
	l := 0.2126*rgb[0] + 0.7152*rgb[1] + 0.0722*rgb[2]
	return l < 0.5, true
}
