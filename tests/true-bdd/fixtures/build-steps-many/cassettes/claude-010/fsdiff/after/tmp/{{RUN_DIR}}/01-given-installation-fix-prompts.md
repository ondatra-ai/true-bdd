# Fix Prompt for Item given-installation: given concept "installation"

## Target
tests/calc/steps

## Required Changes

Two files change, and nothing else. Do not touch `tests/calc/steps/register.go`,
the other two concept files, `tests/calc/scenarios/scenarios.go` or
`tests/calc/calc_test.go`.

### 1. `tests/calc/steps/state.go` — add the shared fields the Given keeps

The Given has to hand two things to the When: where the host project root is
(the When runs the program "from the host root") and the path of the program it
built ("keep its path for the When"). `State` is currently empty, so add exactly
those two fields. Add no other field here — the When's and Then's own fields are
their authors' business.

Replace the whole file with:

```go
package steps

// State is what one scenario's calc steps share.
type State struct {
	// HostRoot is the host project's root — the directory holding the
	// go.mod of the module under test — located by the installation Given.
	// It is the directory the calculator program is run from.
	HostRoot string
	// Program is the path of the calculator program the installation Given
	// built under the host's tmp directory.
	Program string
}
```

### 2. `tests/calc/steps/conceptbef0930c9a09dec2b565c664833e27eb5e85810f2e50d51372923cf970aa997c.go` — write the body

Keep the `package steps` clause and the `//truebdd:concept given "installation"`
record and the function's name and signature exactly as they are. Only the
import block and the function body change.

The body does precisely what the binding's does clause says and nothing more:
it locates the host project root by walking up from the test's working directory
to the nearest `go.mod` (never by testing for `services/calc`, and never past
that module), builds the declared `double` command's program with `go build`
from `./services/calc` with the host root as the build's directory — exactly as
the Public interface document declares the program is built — into a directory
under the host's `tmp`, and stores the root and the program's path for the When.
It never doubles anything and never inspects the program's output; the When runs
it and the Then compares.

The build inherits the process environment, so the Go build cache and temporary
directory handed to the test run are the ones used — the body sets no `GOCACHE`,
no `GOTMPDIR` and no other build variable.

`args` stays unused: the pattern captures nothing, matching the engine's
skeleton.

Replace the whole file with:

```go
package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//truebdd:concept given "installation"
func conceptbef0930c9a09dec2b565c664833e27eb5e85810f2e50d51372923cf970aa997c(state *State, args []string) error {
	const (
		// servicePackage and the go build that follows are what the CLI
		// document declares the calculator is built from, at the host root.
		servicePackage = "./services/calc"
		// buildSubdir keeps the program under the host's tmp directory, as
		// the binding's does clause requires. A fixed name rather than a
		// fresh temp dir: a step gets no *testing.T to register cleanup
		// with, and the suite's scenarios run sequentially, so re-using one
		// directory leaves one program behind instead of one per run.
		buildSubdir = "tmp/bdd-calc"
		programName = "calc"
		buildDirPerm = 0o755
	)

	start, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("installation: locating the working directory: %w", err)
	}

	// The host project root is the nearest enclosing directory with a
	// go.mod — the module the suite belongs to. Decided by go.mod and not
	// by looking for the service's directory, so a host project copied
	// without its service still resolves to its own root and the search
	// never climbs out of the module to find one elsewhere.
	hostRoot := ""
	dir := start

	for {
		_, statErr := os.Stat(filepath.Join(dir, "go.mod"))
		if statErr == nil {
			hostRoot = dir

			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	if hostRoot == "" {
		return fmt.Errorf("installation: no go.mod at or above %s, so the host project root is unknown", start)
	}

	buildDir := filepath.Join(hostRoot, buildSubdir)

	err = os.MkdirAll(buildDir, buildDirPerm)
	if err != nil {
		return fmt.Errorf("installation: preparing the build directory %s: %w", buildDir, err)
	}

	program := filepath.Join(buildDir, programName)

	build := exec.Command("go", "build", "-o", program, servicePackage)
	build.Dir = hostRoot
	// The inherited environment carries the Go build cache and temporary
	// directory this test run was given; the build must use those.
	build.Env = os.Environ()

	output, err := build.CombinedOutput()
	if err != nil {
		return fmt.Errorf("installation: go build -o %s %s in %s: %w\n%s",
			program, servicePackage, hostRoot, err, output)
	}

	state.HostRoot = hostRoot
	state.Program = program

	return nil
}
```

Run `gofmt` on both files after writing (the `const` block above aligns once
gofmt has seen it).

## Notes for the applier
- The declared Public interface is sufficient for this concept: it names the
  program, how it is built (`go build` from `./services/calc` at the host root)
  and how it is invoked, so no `interface_cannot_serve` return is needed.
- No package-level constant, variable, function or type is added. The only
  changes outside the function body are the `State` fields and this file's
  import block.
