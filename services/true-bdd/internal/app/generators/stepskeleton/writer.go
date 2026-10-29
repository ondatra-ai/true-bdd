// Package stepskeleton writes the engine-owned registration and pending body.
package stepskeleton

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

// Definition locates the function whose body a writer may replace.
type Definition struct {
	File     string
	Function string
	Pattern  string
}

// Write adds a pending definition without changing existing definitions or state.
func Write(dir, phrase, pattern string) (Definition, error) {
	files, err := readPackage(dir)
	if err != nil {
		return Definition{}, err
	}

	definition := Location(dir, phrase, pattern)

	_, statErr := os.Stat(definition.File)
	if statErr == nil {
		return definition, definition.Intact(dir)
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return Definition{}, fmt.Errorf("read skeleton location: %w", statErr)
	}

	name := definition.Function

	packageName := "steps"
	if len(files) > 0 {
		packageName = files[0].tree.Name.Name
	}

	source := fmt.Sprintf(`package %s
import "fmt"
func %s(state *State, args []string) error {
 return fmt.Errorf("step not written yet: %%s", %q)
}
`, packageName, name, phrase)

	err = writeGo(definition.File, []byte(source))
	if err != nil {
		return Definition{}, err
	}

	err = ensureState(dir, packageName, files)
	if err != nil {
		return Definition{}, err
	}

	err = register(dir, packageName, name, pattern, files)
	if err != nil {
		return Definition{}, err
	}

	return definition, nil
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

// Location is stable across interrupted walks, keeping one registration per phrase.
func Location(dir, phrase, pattern string) Definition {
	name := fmt.Sprintf("phrase%x", sha256.Sum256([]byte(phrase)))

	return Definition{File: filepath.Join(dir, name+".go"), Function: name, Pattern: pattern}
}

// Pending reports an engine-created placeholder, including one from a stopped walk.
func (d Definition) Pending() bool {
	_, err := os.Stat(d.File)

	return err == nil && errors.Is(d.Written(), ErrPending)
}
