//go:build unix

package fstree

import (
	"io/fs"
	"syscall"
)

// hardLinked reports whether a regular file has more than one directory
// entry pointing at its inode.
//
// Split by build tag because link count lives in a platform stat
// structure: every unix exposes it as Stat_t.Nlink, and Windows does not
// expose it through io/fs at all.
func hardLinked(info fs.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}

	return stat.Nlink > 1
}
