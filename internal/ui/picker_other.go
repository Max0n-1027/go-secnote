//go:build !windows

package ui

import "io/fs"

// protected reports whether e is a file the operating system hides from
// everyday view. Only Windows has such files.
func protected(fs.DirEntry) bool { return false }
