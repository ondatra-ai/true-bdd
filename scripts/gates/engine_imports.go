package gates

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

const (
	repositoryImport = "github.com/ondatra-ai/true-bdd"
	enginePath       = "services/true-bdd"
)

var errEngineImport = errors.New("engine must own all repository code it uses")

// CheckEngineImports examines every Go source, including tests and files hidden by build constraints.
// It needs neither compilable packages nor the engine's module dependencies, so the drill can use it on a cut.
func CheckEngineImports(root string) error {
	var findings []error

	sources := 0

	err := filepath.WalkDir(filepath.Join(root, enginePath), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk engine source: %w", walkErr)
		}

		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%w: source symlink %s", errEngineImport, path)
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		sources++

		err := checkEngineFile(path)
		if err != nil {
			findings = append(findings, err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("checking engine imports: %w", err)
	}

	if sources == 0 {
		return fmt.Errorf("%w: no engine Go sources under %s", errEngineImport, enginePath)
	}

	return errors.Join(findings...)
}

func checkEngineFile(path string) error {
	source, err := disk.Read(path)
	if err != nil {
		return fmt.Errorf("reading engine source: %w", err)
	}

	files := token.NewFileSet()

	parsed, err := parser.ParseFile(files, path, source, parser.ImportsOnly)
	if err != nil {
		return fmt.Errorf("parse engine imports: %w", err)
	}

	var findings []error

	for _, item := range parsed.Imports {
		imported, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			return fmt.Errorf("%s: decode import: %w", path, err)
		}

		if repositoryPackage(imported) && !enginePackage(imported) {
			findings = append(findings, fmt.Errorf("%w: %s imports %s", errEngineImport, files.Position(item.Pos()), imported))
		}
	}

	return errors.Join(findings...)
}

func repositoryPackage(imported string) bool {
	return imported == repositoryImport || strings.HasPrefix(imported, repositoryImport+"/")
}

func enginePackage(imported string) bool {
	prefix := repositoryImport + "/" + enginePath

	return imported == prefix || strings.HasPrefix(imported, prefix+"/")
}

func engineImports() error { return CheckEngineImports(".") }
