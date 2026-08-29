package disk

import "os"

// Mode is this repository's whole permission vocabulary, replacing three
// conventions and the dirMode/fileMode pair five scripts/ packages each
// declared for themselves.
type Mode int

const (
	// Private is the process's own state: history, locks, the host store.
	Private Mode = iota
	// Shared is what a person or another tool reads: reports, goldens, logs.
	Shared
)

const (
	privateFilePerm os.FileMode = 0o600
	privateDirPerm  os.FileMode = 0o700
	sharedFilePerm  os.FileMode = 0o644
	sharedDirPerm   os.FileMode = 0o755
)

// filePerm is the mode a file created under m carries.
func (m Mode) filePerm() os.FileMode {
	if m == Shared {
		return sharedFilePerm
	}

	return privateFilePerm
}

// dirPerm is the mode a directory created under m carries.
func (m Mode) dirPerm() os.FileMode {
	if m == Shared {
		return sharedDirPerm
	}

	return privateDirPerm
}
