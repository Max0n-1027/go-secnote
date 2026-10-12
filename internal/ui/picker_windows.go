package ui

import (
	"io/fs"
	"syscall"
)

// protected reports whether e is one of the hidden system files that
// Explorer keeps out of sight, such as pagefile.sys or the legacy junctions
// in a profile folder.
func protected(e fs.DirEntry) bool {
	fi, err := e.Info()
	if err != nil {
		return false
	}
	const mask = syscall.FILE_ATTRIBUTE_HIDDEN | syscall.FILE_ATTRIBUTE_SYSTEM
	data, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&mask == mask
}
