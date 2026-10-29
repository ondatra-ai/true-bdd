package drill_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDocumentedDrillPipeline(t *testing.T) {
	guide, err := os.ReadFile("../../docs/agents/drill.md")
	must(t, err)

	commands := regexp.MustCompile(`(?m)^go run ./scripts/cmd/drill run b \S+ --iteration <dir> -- (.+)$`).
		FindAllStringSubmatch(string(guide), -1)

	pipeline := make([]string, 0, len(commands))
	for _, command := range commands {
		pipeline = append(pipeline, command[1])
	}

	if !slices.Equal(pipeline, []string{"build tests --fix", "build steps --fix", "build code --fix"}) {
		t.Fatalf("documented drill pipeline diverges from ADR-0031: %v", pipeline)
	}

	runner, record := fixture(t)
	if runner.Limit != 3*time.Hour {
		t.Fatalf("command limit diverges from drill rules: %s", runner.Limit)
	}

	cut, err := runner.Cut(t.Context(), record.Directory, "b")
	must(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(cut.Directory) })

	for index, command := range pipeline {
		attempt, err := runner.Prepare(t.Context(), record.Directory, "b", "documented-"+strconv.Itoa(index),
			strings.Fields(command))
		must(t, err)
		must(t, runner.Supervise(t.Context(), record.Directory, attempt.Tag))

		_, err = os.Stat(filepath.Join(record.Directory, "runs", attempt.Tag, "engine.log"))
		must(t, err)
	}
}
