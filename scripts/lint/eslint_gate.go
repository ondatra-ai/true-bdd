package lint

import (
	"io"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/eslint"
)

// prototypeTree is carried for reference, not authored: eslint reports 7963
// problems in it, so the gate answers for the frontend this repo writes.
const prototypeTree = eslint.Root + "/design/"

// eslintPathspecs is what a bare run walks: the extensions the config covers.
//
//nolint:gochecknoglobals // a pathspec list; a constant in all but syntax.
var eslintPathspecs = []string{
	eslint.Root + "/*.js", eslint.Root + "/*.jsx", eslint.Root + "/*.ts",
	eslint.Root + "/*.tsx", eslint.Root + "/*.mjs", eslint.Root + "/*.cjs",
}

// ESLint runs the frontend's eslint over the named files, or over every file
// this repository authors under that tree when none are named.
func ESLint(out io.Writer, files []string, fix bool) error {
	err := eslint.Available()
	if err != nil {
		return err
	}

	scoped, err := eslintScope(files)
	if err != nil || len(scoped) == 0 {
		return err
	}

	result, err := eslint.Lint(out, fix, scoped...)
	if err != nil {
		return err
	}

	if result.Code != 0 {
		return ErrFailed
	}

	return nil
}

// eslintScope narrows to the files this gate owns: inside eslint's tree, and
// outside the prototype.
func eslintScope(files []string) ([]string, error) {
	candidates := files

	if len(files) == 0 {
		tracked, err := trackedFiles(eslintPathspecs...)
		if err != nil {
			return nil, err
		}

		candidates = tracked
	}

	var scoped []string

	for _, file := range candidates {
		if eslint.Owns(file) && !strings.HasPrefix(file, prototypeTree) {
			scoped = append(scoped, file)
		}
	}

	return scoped, nil
}
