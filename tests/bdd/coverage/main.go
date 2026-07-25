// Command coverage is the bootstrap checklist-branch coverage
// collector: it scans retained BDD fixture run directories under
// tmp/test_run/, reduces their artifacts to occurrence-based coverage
// of the shipped checklist-content branches, renders a report, and
// reads/writes the reconstructed baseline.
//
// Zero engine changes: everything is recovered from artifacts the CLI
// already writes. Profiles and baselines are always labeled
// evidence_quality: reconstructed — they have no ratchet authority.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

// errWriteAndCheck rejects the ambiguous flag combination: checking a
// baseline that was just overwritten proves nothing.
var errWriteAndCheck = errors.New(
	"-write-baseline and -check-baseline are mutually exclusive")

// errEmptyUniverse rejects a checklist dir that yields no branches.
var errEmptyUniverse = errors.New("universe is empty — wrong -checklists path?")

// errSessionEmpty rejects a -session that selects no runs.
var errSessionEmpty = errors.New("-session matched no fixture runs")

// errNothingToWrite refuses to capture a baseline from an empty scan.
var errNothingToWrite = errors.New(
	"refusing -write-baseline: the scan selected no runs or covered no branches " +
		"(wrong -root?); use -rebaseline to force an empty baseline deliberately")

// options carries the parsed CLI flags.
type options struct {
	Root          string
	ChecklistsDir string
	FixturesDir   string
	BaselinePath  string
	JSONPath      string
	Session       string
	OnlyGaps      bool
	WriteBaseline bool
	CheckBaseline bool
	Rebaseline    bool
}

func main() {
	err := run(os.Args[1:], os.Stdout)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "coverage: %v\n", err)

		os.Exit(1)
	}
}

// run executes the tool.
func run(args []string, out *os.File) error {
	opts, err := parseFlags(args)
	if err != nil {
		return err
	}

	uni, err := LoadUniverse(opts.ChecklistsDir)
	if err != nil {
		return err
	}

	if len(uni.Branches) == 0 {
		return errEmptyUniverse
	}

	profile, selected, err := scanAndBuild(opts, uni)
	if err != nil {
		return err
	}

	reporter := &Reporter{Universe: uni, Profile: profile, Out: out,
		OnlyGaps: opts.OnlyGaps, Source: sourceLabel(opts)}
	reporter.Render()

	if opts.JSONPath != "" {
		err = profile.WriteJSON(opts.JSONPath)
		if err != nil {
			return err
		}
	}

	return baselineOps(opts, profile, selected, out)
}

// parseFlags parses CLI arguments into options.
func parseFlags(args []string) (*options, error) {
	opts := &options{}
	flagSet := flag.NewFlagSet("coverage", flag.ContinueOnError)

	flagSet.StringVar(&opts.Root, "root", "tmp/test_run", "retained fixture run root")
	flagSet.StringVar(&opts.ChecklistsDir, "checklists", "true-bdd/checklists", "shipped checklists dir")
	flagSet.StringVar(&opts.FixturesDir, "fixtures", "tests/bdd/fixtures", "fixture manifests dir")
	flagSet.StringVar(&opts.BaselinePath, "baseline", "tests/bdd/coverage/baseline.yaml", "baseline file")
	flagSet.StringVar(&opts.JSONPath, "json", "", "write the machine profile to this path")
	flagSet.StringVar(&opts.Session, "session", "", "restrict the scan to one session (regression mode)")
	flagSet.BoolVar(&opts.OnlyGaps, "uncovered", false, "show only prompts with uncovered branches")
	flagSet.BoolVar(&opts.WriteBaseline, "write-baseline", false, "write the covered set as the baseline")
	flagSet.BoolVar(&opts.CheckBaseline, "check-baseline", false, "fail when a baselined branch is uncovered")
	flagSet.BoolVar(&opts.Rebaseline, "rebaseline", false, "allow -write-baseline to drop existing entries")

	err := flagSet.Parse(args)
	if err != nil {
		return nil, fmt.Errorf("parsing flags: %w", err)
	}

	if opts.WriteBaseline && opts.CheckBaseline {
		return nil, errWriteAndCheck
	}

	return opts, nil
}

// scanAndBuild collects every run and merges the profile. Universe
// defects (duplicate Q hashes) become hard diagnostics so they block
// baseline writes.
func scanAndBuild(opts *options, uni *Universe) (*Profile, int, error) {
	runs, err := ScanRoot(opts.Root)
	if err != nil {
		return nil, 0, err
	}

	observations := make([]Observation, 0)
	diagnostics := make([]Diagnostic, 0)
	selected := 0

	for _, run := range runs {
		if opts.Session != "" && run.Session != opts.Session {
			continue
		}

		selected++

		obs, diags := CollectRun(run, uni, opts.FixturesDir)

		observations = append(observations, obs...)
		diagnostics = append(diagnostics, diags...)
	}

	if opts.Session != "" && selected == 0 {
		return nil, 0, fmt.Errorf("%w: %s", errSessionEmpty, opts.Session)
	}

	for _, defect := range uni.Diagnostics {
		diagnostics = append(diagnostics,
			Diagnostic{Fixture: "(universe)", Hard: true, Message: defect})
	}

	return BuildProfile(uni, observations, diagnostics), selected, nil
}

// sourceLabel describes what the profile was computed from.
func sourceLabel(opts *options) string {
	if opts.Session != "" {
		return "session " + opts.Session + " only (reconstructed, no ratchet authority)"
	}

	return "all-history union of " + opts.Root + " (reconstructed, no ratchet authority)"
}

// baselineOps applies the requested baseline action.
func baselineOps(opts *options, profile *Profile, selected int, out *os.File) error {
	if opts.WriteBaseline {
		baseline := BaselineFromProfile(profile)

		if (selected == 0 || len(baseline.Covered) == 0) && !opts.Rebaseline {
			return errNothingToWrite
		}

		err := baseline.Write(opts.BaselinePath,
			profile.Diagnostics.HardRunDiagnostics, opts.Rebaseline)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(out, "\nbaseline written: %s (%d covered ids, %s)\n",
			opts.BaselinePath, len(baseline.Covered), profile.EvidenceQuality)
	}

	if opts.CheckBaseline {
		baseline, err := LoadBaseline(opts.BaselinePath)
		if err != nil {
			return err
		}

		newlyCovered, err := baseline.Check(profile)

		for _, id := range newlyCovered {
			_, _ = fmt.Fprintf(out, "newly covered (not in baseline): %s\n", id)
		}

		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(out, "\nbaseline check: OK (%d ids)\n", len(baseline.Covered))
	}

	return nil
}
