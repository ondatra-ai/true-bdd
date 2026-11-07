package testkit

import "github.com/ondatra-ai/true-bdd/pkg/disk"

// FileMode selects private process state or files shared with people and tools.
type FileMode = disk.Mode

const (
	// Private creates files readable only by the current user.
	Private = disk.Private
	// Shared creates files readable by other users.
	Shared = disk.Shared
)

// Read returns a file's bytes through the suite's filesystem channel.
func Read(path string) ([]byte, error) { return disk.Read(path) }

// Write atomically publishes bytes, creating parent directories as needed.
func Write(path string, data []byte, mode FileMode) error { return disk.Write(path, data, mode) }

// Dir creates a directory and its missing parents with the requested visibility.
func Dir(path string, mode FileMode) error { return disk.Dir(path, mode) }

// Executable makes an existing fixture-controlled script executable.
func Executable(path string) error { return disk.Executable(path) }
