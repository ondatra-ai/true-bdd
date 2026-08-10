package reporter

import (
	"fmt"
	"os"
	"path/filepath"
)

// indexFileName is the report's entry page. Every detail page links back
// to it by this name, from the same flat directory.
const indexFileName = "index.html"

const (
	outDirPerm  = 0o755
	outFilePerm = 0o644
)

// writePages writes the flat set — index.html plus one <fixture>.html
// beside it — then drops any page a previous session left behind.
func writePages(fixtures []*Fixture, session, outDir string) (int, error) {
	err := os.MkdirAll(outDir, outDirPerm)
	if err != nil {
		return 0, fmt.Errorf("create report dir %s: %w", outDir, err)
	}

	keep := map[string]bool{indexFileName: true}

	err = writePage(filepath.Join(outDir, indexFileName),
		newRenderer(fixtures, session).Render())
	if err != nil {
		return 0, err
	}

	for _, fixture := range fixtures {
		name := detailFileName(fixture.Name)
		keep[name] = true

		err = writePage(filepath.Join(outDir, name),
			newDetailRenderer(fixture, session).Render())
		if err != nil {
			return 0, fmt.Errorf("detail page for %s: %w", fixture.Name, err)
		}
	}

	pruneStalePages(outDir, keep)

	return len(fixtures), nil
}

// writePage puts one rendered document on disk.
func writePage(path, document string) error {
	err := os.WriteFile(path, []byte(document), outFilePerm)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// pruneStalePages removes pages a previous session left for fixtures
// this one does not have. A renamed or deleted fixture would otherwise
// keep a page that is unreachable from the index but still live at its
// old URL — last week's run served as if it were this one.
//
// Deliberately os.Remove of named *.html entries rather than a
// RemoveAll of the directory: this runs after every fixture, and a
// RemoveAll on a path that ever resolves wrong is unrecoverable.
//
// Mid-run, a previous session's last fixture is pruned after this
// session's first and re-created when it runs. That is correct — the
// index only ever lists what has actually run.
func pruneStalePages(outDir string, keep map[string]bool) {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || keep[name] || filepath.Ext(name) != ".html" {
			continue
		}

		_ = os.Remove(filepath.Join(outDir, name))
	}
}
