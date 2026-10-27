package testrunner_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

func TestVerificationArgumentArrivesWhole(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	const argument = `-engine-bin=/some engine's "build"/true-bdd`

	script := `#!/bin/sh
set -e
[ "$#" = 3 ]
[ "$1" = '{engine_bin}' ]
[ "$2" = '-json' ]
[ "$3" = '-engine-bin=/some engine'"'"'s "build"/true-bdd' ] || exit 1
printf '%s\n' '{"Action":"pass","Package":"fixture/selected","Test":"TestWhole"}'
`

	path := filepath.Join(root, "runner.sh")

	err := disk.Write(path, []byte(script), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	cfg := testrunner.Config{
		Framework: testrunner.FrameworkGoTest, ConfigFile: filepath.Join(root, "go.mod"),
		Command: "sh runner.sh '{engine_bin}' -json", Arguments: []string{argument},
	}

	failures, err := testrunner.NewGoTestRunner(nil).Discover(context.Background(), cfg, "", "")
	if err != nil || len(failures) != 0 {
		t.Fatalf("argument must reach runner as one element: %v, %v", failures, err)
	}
}
