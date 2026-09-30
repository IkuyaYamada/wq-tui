// Package ime switches the keyboard input source so board keys arrive as
// ASCII even when a Japanese input method was active. Only macOS is
// supported; elsewhere every call is a no-op.
//
// Calls must come from the main OS thread (macOS requires it for the Text
// Input Sources API); main locks the main goroutine to it.
package ime

// Switcher is what the UI needs from the platform.
type Switcher interface {
	// ASCII selects an ASCII-capable input source and returns the ID of the
	// one that was active before, or "" if nothing changed.
	ASCII() string
	// Select activates the input source with the given ID.
	Select(id string)
}

// Noop leaves the input source alone.
type Noop struct{}

func (Noop) ASCII() string { return "" }
func (Noop) Select(string) {}
