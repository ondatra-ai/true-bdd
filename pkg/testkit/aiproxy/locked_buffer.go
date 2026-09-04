package main

import (
	"bytes"
	"sync"
)

// lockedBuffer is a mutex-guarded byte buffer: the stdin tee goroutine can
// still be running (the engine may hold the pipe open past the child's
// lifetime), so reads must not race its writes.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p) //nolint:wrapcheck // io.Writer contract; bytes.Buffer never errors
}

// Bytes returns a copy of the buffered bytes.
func (b *lockedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	return append([]byte(nil), b.buf.Bytes()...)
}

// Len returns the current byte count.
func (b *lockedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Len()
}
