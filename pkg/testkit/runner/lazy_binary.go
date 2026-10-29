package runner

import (
	"fmt"
	"sync"
)

// LazyBinary shares a service build without making it part of harness bring-up.
type LazyBinary struct {
	build func() (string, error)
	once  sync.Once
	path  string
	err   error
}

// NewLazyBinary defers the build until a step first requests the executable.
func NewLazyBinary(build func() (string, error)) *LazyBinary { return &LazyBinary{build: build} }

// Path returns the same build result to each Scenario at its calling step.
func (b *LazyBinary) Path() (string, error) {
	b.once.Do(func() { b.path, b.err = b.build() })

	if b.err != nil {
		return "", fmt.Errorf("build service for step: %w", b.err)
	}

	return b.path, nil
}
