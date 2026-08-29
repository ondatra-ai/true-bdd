package disk

import (
	"os"
	"path/filepath"
	"testing"
)

const payload = 4096

// BenchmarkWrite measures the committed path against a bare os.WriteFile, so
// the cost of the hold plus the rename is attributable.
func BenchmarkWrite(b *testing.B) {
	dir := b.TempDir()
	data := make([]byte, payload)

	for i := 0; b.Loop(); i++ {
		err := Write(filepath.Join(dir, "f"), data, Shared)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBareWriteFile(b *testing.B) {
	dir := b.TempDir()
	data := make([]byte, payload)

	for i := 0; b.Loop(); i++ {
		err := os.WriteFile(filepath.Join(dir, "f"), data, 0o644)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRead(b *testing.B) {
	path := filepath.Join(b.TempDir(), "f")

	err := os.WriteFile(path, make([]byte, payload), 0o644)
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		_, readErr := Read(path)
		if readErr != nil {
			b.Fatal(readErr)
		}
	}
}

func BenchmarkBareReadFile(b *testing.B) {
	path := filepath.Join(b.TempDir(), "f")

	err := os.WriteFile(path, make([]byte, payload), 0o644)
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		_, readErr := os.ReadFile(path)
		if readErr != nil {
			b.Fatal(readErr)
		}
	}
}
