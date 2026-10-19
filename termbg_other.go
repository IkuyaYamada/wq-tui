//go:build !darwin && !linux

package main

func tmuxBackgroundIsDark() (dark, ok bool) { return false, false }
