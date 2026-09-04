//go:build unix

package fstree

import (
	"io/fs"
	"syscall"
)

// hardLinked reports whether a regular file has more than one directory
// entry pointing at its inode. Split by build tag: link count lives in a
// platform stat struct (Stat_t.Nlink on unix; Windows exposes none via io/fs).
func hardLinked(info fs.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}

	return stat.Nlink > 1
}
