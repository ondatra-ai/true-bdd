// Command reporter renders the BDD run report for a session that has
// already finished.
//
// The BDD suite renders in-process after every fixture (see
// tests/bdd-cli/bdd_test.go), so this command is the manual path: for
// re-rendering an older session, or one whose suite was killed.
//
//	go run ./tests/bdd-cli/cmd/reporter
//
//	# -> tmp/test_report/index.html       the index
//	# -> tmp/test_report/<fixture>.html   one page per fixture
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/reporter"
)

func main() {
	err := run()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "reporter: %v\n", err)

		os.Exit(1)
	}
}

// run parses flags, renders, and prints the summary the library
// deliberately does not print itself.
func run() error {
	opts := parseFlags()

	summary, err := reporter.Render(opts)
	if err != nil {
		return fmt.Errorf("render report: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "wrote %s  (%d fixture(s), %d turns, %d detail page(s))\n",
		summary.IndexPath, summary.Fixtures, summary.Turns, summary.Pages)

	for _, drift := range summary.Drift {
		_, _ = fmt.Fprintf(os.Stdout, "  %s: wall %.2fs, phases %.2fs, drift %.3fs\n",
			drift.Name, drift.Wall, drift.Phases, drift.Drift)
	}

	return nil
}

// parseFlags reads the command line. Every default is left empty so the
// library owns them in exactly one place; the usage strings state them.
func parseFlags() reporter.Options {
	var opts reporter.Options

	flag.StringVar(&opts.Session, "session", "",
		"run session directory to report on (default: newest under tmp/test_run)")
	flag.StringVar(&opts.GoTest, "gotest", "",
		"verbose `go test` log to fall back on for fixtures with no harness record "+
			"(default: none)")
	flag.StringVar(&opts.OutDir, "out-dir", "",
		"directory for index.html and the per-fixture pages (default: tmp/test_report)")
	flag.Parse()

	return opts
}
