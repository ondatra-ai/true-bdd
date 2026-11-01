// Package stepskeleton writes the engine-owned registration and pending body.
package stepskeleton

import (
	"errors"
	"fmt"
	"go/format"
	"os"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

// Definition locates the function whose body a writer may replace.
type Definition struct {
	File     string
	Function string
	Pattern  string
}

func writeGo(file string, source []byte) error {
	formatted, err := format.Source(source)
	if err != nil {
		return fmt.Errorf("format skeleton %s: %w", file, err)
	}

	err = disk.Write(file, formatted, disk.Shared)
	if err != nil {
		return fmt.Errorf("write skeleton: %w", err)
	}

	return nil
}

// Pending reports an engine-created placeholder, including one from a stopped walk.
func (d Definition) Pending() bool {
	_, err := os.Stat(d.File)

	return err == nil && errors.Is(d.Written(), ErrPending)
}
