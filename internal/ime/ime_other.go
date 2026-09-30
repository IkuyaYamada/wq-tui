//go:build !darwin

package ime

// System is a no-op outside macOS.
type System = Noop
