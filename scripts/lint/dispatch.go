package lint

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Dispatch runs the gates the named files select; with none, all of them.
// alint always runs: its rules are about where a file sits, not what is in it.
//
//	*.go           golangci-lint on its package, comments
//	*.sh           comments
//	*.yaml|*.yml   schemas, comments
//	*.md           markdownlint — CLAUDE.md excepted, .alint.yml owns it
//	anything else  nothing beyond alint
func Dispatch(out io.Writer, files []string) error {
	picked := selectGates(files)

	// Named files get `fix`, bare gets `check`. alint applies only fixes a
	// rule declares, and exits non-zero on what it could not fix.
	verb := "check"
	if len(files) > 0 {
		verb = "fix"
	}

	steps := []func() error{func() error { return runTool(out, "alint", verb) }}

	if picked.comments {
		steps = append(steps, func() error { return Comments(out, files) })
	}

	if picked.schemas {
		steps = append(steps, func() error { return Schemas(out, files) })
	}

	if picked.markdown {
		steps = append(steps, func() error { return Markdown(out, files) })
	}

	steps = append(steps, func() error {
		return golangci(out, files, picked.goDirs, picked.wholeRepoGo)
	})

	failed := false

	for _, step := range steps {
		if note(out, step()) {
			failed = true
		}
	}

	if failed {
		return ErrFailed
	}

	return nil
}

// note prints a gate's own hard failure and reports whether it failed at all;
// every gate runs even after one fails, so a pass reports everything.
func note(out io.Writer, err error) bool {
	if err == nil {
		return false
	}

	if !isVerdict(err) {
		_, _ = fmt.Fprintln(out, err)
	}

	return true
}

func isVerdict(err error) bool {
	return errors.Is(err, ErrFailed)
}

// gates is which of them this run selected.
type gates struct {
	comments    bool
	schemas     bool
	markdown    bool
	wholeRepoGo bool
	goDirs      []string
}

func selectGates(files []string) gates {
	if len(files) == 0 {
		return gates{comments: true, schemas: true, markdown: true, wholeRepoGo: true}
	}

	picked := gates{}

	for _, file := range files {
		switch filepath.Ext(file) {
		case ".md":
			picked.markdown = true
		case ".go":
			picked.comments = true
			picked.goDirs = addGoPackage(picked.goDirs, file)
		case ".sh":
			picked.comments = true
		case ".yaml", ".yml":
			picked.comments = true
			picked.schemas = true
		}
	}

	return picked
}

// addGoPackage names the file's package, unless a sentinel go.mod fences its
// subtree out of the root module — golangci-lint asked to lint inside one
// exits 5 with "no go files to analyze", so such a file selects no Go gate.
func addGoPackage(dirs []string, file string) []string {
	dir := filepath.Dir(file)

	for walk := dir; walk != "."; walk = filepath.Dir(walk) {
		_, err := os.Stat(filepath.Join(walk, "go.mod"))
		if err == nil {
			return dirs
		}
	}

	target := "./" + dir
	for _, known := range dirs {
		if known == target {
			return dirs
		}
	}

	return append(dirs, target)
}

// golangci lints the whole module, or the packages the named files belong to.
// The package is the floor: `golangci-lint run <file>.go` typechecks that file
// ALONE, so all 17 package-mates in review.go came back "undefined".
func golangci(out io.Writer, files, dirs []string, wholeRepo bool) error {
	switch {
	case wholeRepo:
		return hushed(out, "golangci-lint", "run")
	case len(dirs) == 0:
		return nil
	default:
		// --fix only when files are named: bare, this gate mirrors CI and
		// must not rewrite; named, fixing as the code is written is the point.
		args := []string{"run"}
		if len(files) > 0 {
			args = append(args, "--fix")
		}

		return hushed(out, "golangci-lint", append(args, dirs...)...)
	}
}

// hushed drops golangci's own chatter — nine lines of exclusion-rule
// bookkeeping per run, which buries the finding in what the hook hands back.
func hushed(out io.Writer, name string, args ...string) error {
	//nolint:gosec // the tool name is a literal; the arguments are package paths.
	cmd := exec.CommandContext(context.Background(), name, args...)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("piping %s: %w", name, err)
	}

	cmd.Stderr = cmd.Stdout

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("starting %s: %w", name, err)
	}

	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), bufio.MaxScanTokenSize)

	for scanner.Scan() {
		if !strings.HasPrefix(scanner.Text(), "level=warning") {
			_, _ = fmt.Fprintln(out, scanner.Text())
		}
	}

	err = cmd.Wait()
	if err != nil {
		return ErrFailed
	}

	return nil
}
