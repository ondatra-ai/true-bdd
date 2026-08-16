//go:build !unix

package fstree

import "io/fs"

// hardLinked cannot be answered from io/fs alone off unix, and a guess
// either way would be worse than the honest answer: report no hard link
// and let the symlink check carry the escape guarantee.
func hardLinked(_ fs.FileInfo) bool {
	return false
}
