package stepskeleton

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// conceptDirective records a skeleton's concept on its function, as
// `//truebdd:concept <block> "<concept>"`, so a later walk can read it.
const conceptDirective = "//truebdd:concept "

// ConceptDefinition is a function recording its concept, with the pattern
// its registration binds.
type ConceptDefinition struct {
	Definition

	Block   string
	Concept string
}

// ConceptLocation is the skeleton's identity: its concept, suite and pattern.
func ConceptLocation(dir, block, concept, pattern string) Definition {
	identity := strings.Join([]string{block, concept, filepath.ToSlash(dir), pattern}, "\x00")
	name := fmt.Sprintf("concept%x", sha256.Sum256([]byte(identity)))

	return Definition{File: filepath.Join(dir, name+".go"), Function: name, Pattern: pattern}
}

// WriteConcept adds a pending definition recording its concept; an existing
// skeleton of the same identity is kept, so an interrupted walk resumes.
func WriteConcept(dir, block, concept, pattern string) (Definition, error) {
	files, err := readPackage(dir)
	if err != nil {
		return Definition{}, err
	}

	definition := ConceptLocation(dir, block, concept, pattern)

	_, statErr := os.Stat(definition.File)
	if statErr == nil {
		return definition, definition.Intact(dir)
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return Definition{}, fmt.Errorf("read skeleton location: %w", statErr)
	}

	packageName := "steps"
	if len(files) > 0 {
		packageName = files[0].tree.Name.Name
	}

	source := fmt.Sprintf(`package %s
import "fmt"
%s%s %s
func %s(state *State, args []string) error {
 return fmt.Errorf("step not written yet: %%s", %q)
}
`, packageName, conceptDirective, block, strconv.Quote(concept), definition.Function,
		fmt.Sprintf("%s concept %q: %s", block, concept, pattern))

	err = writeGo(definition.File, []byte(source))
	if err != nil {
		return Definition{}, err
	}

	err = ensureState(dir, packageName, files)
	if err != nil {
		return Definition{}, err
	}

	err = register(dir, packageName, definition.Function, pattern, files)
	if err != nil {
		return Definition{}, err
	}

	return definition, nil
}

// Concepts lists every function of the steps package that records a
// concept, each with the pattern registered for it.
func Concepts(dir string) ([]ConceptDefinition, error) {
	files, err := readPackageMode(dir, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	patterns := registeredPatterns(packageFunctions(files))
	found := make([]ConceptDefinition, 0)

	for _, file := range files {
		for _, declaration := range file.tree.Decls {
			fun, ok := declaration.(*ast.FuncDecl)
			if !ok || fun.Recv != nil || fun.Doc == nil {
				continue
			}

			block, concept, recorded := recordedConcept(fun.Doc)
			if !recorded {
				continue
			}

			found = append(found, ConceptDefinition{Block: block, Concept: concept, Definition: Definition{
				File: file.path, Function: fun.Name.Name, Pattern: patterns[fun.Name.Name]}})
		}
	}

	return found, nil
}

func recordedConcept(doc *ast.CommentGroup) (string, string, bool) {
	for _, comment := range doc.List {
		record, found := strings.CutPrefix(comment.Text, conceptDirective)
		if !found {
			continue
		}

		block, quoted, found := strings.Cut(record, " ")
		concept, err := strconv.Unquote(quoted)

		if found && err == nil && block != "" && concept != "" {
			return block, concept, true
		}
	}

	return "", "", false
}

// registeredPatterns reads every literal registration of the package, as the
// resolver binds them, wherever Register's helpers make them.
func registeredPatterns(functions []*ast.FuncDecl) map[string]string {
	patterns := make(map[string]string)

	for _, fun := range functions {
		if fun.Body == nil {
			continue
		}

		ast.Inspect(fun.Body, func(node ast.Node) bool {
			pattern, function, found := stepCall(node)
			if found {
				patterns[function] = pattern
			}

			return true
		})
	}

	return patterns
}
