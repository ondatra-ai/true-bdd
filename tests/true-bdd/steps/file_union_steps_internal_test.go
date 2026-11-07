package steps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSomeFileMatchObservesStepsAndHelpersTogether(t *testing.T) {
	t.Parallel()

	for _, directory := range []string{"steps", "helpers"} {
		t.Run(directory, func(t *testing.T) {
			t.Parallel()
			state := runAt(t.TempDir())

			path := filepath.Join(state.Result.TmpDir, "tests/calc", directory, "contract.go")

			err := os.MkdirAll(filepath.Dir(path), 0o700)
			if err != nil {
				t.Fatal(err)
			}

			err = os.WriteFile(path, []byte("CALC_REAL_GO"), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			args := []string{"tests/calc/steps/*.go", "tests/calc/helpers/*.go", "CALC_REAL_GO"}

			err = assertSomeFileMatches(state, args)
			if err != nil {
				t.Fatal(err)
			}

			args[2] = "CALC_LICENCE"

			err = assertSomeFileMatches(state, args)
			if err == nil {
				t.Fatal("absent contract passed")
			}
		})
	}
}
