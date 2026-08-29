package commit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// recordingsGlob is the RECORDED fixture data — cassettes and goldens.
const recordingsGlob = "tests/bdd-cli/fixtures/*/cassettes"

// reportLimit is how many offending files a category names, as `head -5` did.
const reportLimit = 5

// sweep is one thing a recording must not carry out of the machine that made it.
type sweep struct {
	report  string
	pattern *regexp.Regexp
	// allowed, when set, clears a file every one of whose lines carries it.
	allowed *regexp.Regexp
}

//nolint:gochecknoglobals // the sweep table; a constant in all but syntax.
var sweeps = []sweep{
	{
		// Not $HOME literally: a cassette recorded by someone else carries
		// THEIR path, and this must fail for them too.
		report:  "a home directory path survived normalization",
		pattern: regexp.MustCompile(`/(Users|home)/[a-z_][a-z0-9_-]*`),
	},
	{
		// Which integrations, skills and plugins the recording machine had.
		report:  "session inventory survived sanitizing",
		pattern: regexp.MustCompile(`"(mcp_servers|slash_commands|memory_paths|apiKeySource)"`),
	},
	{
		// Never yet seen in a recording, which is exactly when a check is
		// worth having.
		report: "credential-shaped string in a recording",
		pattern: regexp.MustCompile(
			`(sk-[A-Za-z0-9_-]{16,}|ghp_[A-Za-z0-9]{20,}|` +
				`xox[baprs]-[A-Za-z0-9-]{10,}|BEGIN [A-Z ]*PRIVATE KEY)`),
	},
	{
		report:  "e-mail address in a recording",
		pattern: regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-z]{2,}`),
		allowed: regexp.MustCompile(`noreply@|example\.(com|org)`),
	},
}

// scanRecordings sweeps the recorded fixture data for anything identifying the
// machine that made it. The LLM reviewer cannot: a re-record changes ~475
// files, four times CodeRabbit's per-review limit — see docs/adr/0002.
func (r *Run) scanRecordings() {
	r.banner("recordings")

	files := r.recordingFiles()
	found := false

	for _, check := range sweeps {
		hits := check.hits(files)
		if len(hits) == 0 {
			continue
		}

		found = true

		r.logf("RECORDING LEAK — %s", check.report)

		for _, hit := range hits {
			r.logf("    %s", hit)
		}
	}

	if found {
		r.dief("a recording carries something the machine that made it should have kept.\n" +
			"  Re-record after fixing the shim's normalization — never edit a cassette by hand.")
	}

	r.logf("recordings: clean (%d files)", len(files))
}

// hits names up to reportLimit files this sweep objects to.
func (s sweep) hits(files []string) []string {
	var found []string

	for _, path := range files {
		if len(found) == reportLimit {
			return found
		}

		raw, err := disk.Read(path)
		if err != nil || !s.pattern.Match(raw) {
			continue
		}

		if s.allowed != nil && s.cleared(string(raw)) {
			continue
		}

		found = append(found, path)
	}

	return found
}

// cleared reports whether every line of the file carries an allowed form, so
// the only addresses in it are ones this project legitimately ships.
func (s sweep) cleared(content string) bool {
	for _, line := range strings.Split(strings.TrimSuffix(content, "\n"), "\n") {
		if !s.allowed.MatchString(line) {
			return false
		}
	}

	return true
}

// recordingFiles is every file under a fixture's cassettes directory.
func (r *Run) recordingFiles() []string {
	dirs, err := filepath.Glob(recordingsGlob)
	if err != nil {
		r.dief("globbing %s: %v", recordingsGlob, err)
	}

	var files []string

	for _, dir := range dirs {
		err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				files = append(files, path)
			}

			return nil
		})
		if err != nil {
			r.dief("walking %s: %v", dir, err)
		}
	}

	return files
}
