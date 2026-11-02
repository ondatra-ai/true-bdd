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
		buildSubdir  = "tmp/bdd-calc"
		programName  = "calc"
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
