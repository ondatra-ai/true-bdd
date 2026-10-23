package drill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

// Oracle is the independent suite's Scenario count and first failures.
type Oracle struct {
	Directory string   `json:"directory"`
	Binary    string   `json:"binary"`
	Total     int      `json:"total"`
	Passed    int      `json:"passed"`
	Failures  []string `json:"failures"`
}

// PrepareOracle reserves a supervised oracle run with a unique tag.
func (r Runner) PrepareOracle(ctx context.Context, directory, cut string) (Attempt, error) {
	tag := "oracle-" + cut + "-" + time.Now().UTC().Format("20060102-150405.000000000")

	run, err := r.prepare(ctx, directory, cut, tag, []string{"oracle"})
	if err != nil {
		return run, err
	}

	run.Oracle = &Oracle{Binary: filepath.Join(directory, "runs", tag, "engine"), Failures: []string{}}
	err = changeRun(directory, tag, func(saved *Attempt) { saved.Oracle = run.Oracle })

	return run, err
}

func (r Runner) executeOracle(ctx context.Context, record Record, run Attempt, cut Cut) (int, error) {
	oracle := *run.Oracle

	tree, err := privateDirectory(record.Repository)
	if err != nil {
		return -1, err
	}

	oracle.Directory = tree

	archive, err := (git.Repository{Dir: record.Repository}).Archive(ctx, record.Base)
	if err != nil {
		return -1, err
	}

	err = exportTree(tree, archive, "a")
	if err != nil {
		return -1, err
	}

	err = validateTree(tree, "a")
	if err != nil {
		return -1, err
	}

	_, err = (git.Repository{Dir: tree}).InitSnapshot(ctx)
	if err != nil {
		return -1, err
	}

	err = changeRun(record.Directory, run.Tag, func(saved *Attempt) { saved.Oracle = &oracle })
	if err != nil {
		return -1, err
	}

	opt := gotool.OracleOptions{Directory: cut.Directory,
		Marker: run.Marker,
		Output: fileWriter{run.Log},
		Limit:  r.Limit,
		Grace:  r.Grace}

	code, err := gotool.BuildOracle(ctx, oracle.Binary, opt)
	if err != nil || code != 0 {
		return code, err
	}

	opt.Directory = tree
	replayLog := filepath.Join(record.Directory, "runs", run.Tag, "oracle.jsonl")
	opt.Output = fileWriter{replayLog}
	code, err = gotool.ReplayOracle(ctx, oracle.Binary, opt)
	summaryErr := oracle.summarize(tree, replayLog)
	saveErr := changeRun(record.Directory, run.Tag, func(saved *Attempt) { saved.Oracle = &oracle })

	return code, errors.Join(err, summaryErr, saveErr)
}

func (o *Oracle) summarize(tree, log string) error {
	names, err := registryTests(tree)
	if err != nil {
		return err
	}

	o.Total = len(names)

	events, err := disk.Read(log)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(strings.NewReader(string(events)))

	for {
		var event oracleEvent

		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("reading oracle result: %w", err)
		}

		if !names[event.Test] {
			continue
		}

		if event.Action == "pass" {
			o.Passed++
		}

		const firstFailures = 10
		if event.Action == "fail" && len(o.Failures) < firstFailures {
			o.Failures = append(o.Failures, event.Test)
		}
	}
}

func scenarioTestName(id string) string {
	var name strings.Builder
	name.WriteString("Test")

	for _, word := range strings.FieldsFunc(id,
		func(char rune) bool { return !unicode.IsLetter(char) && !unicode.IsDigit(char) }) {
		chars := []rune(word)
		chars[0] = unicode.ToUpper(chars[0])
		name.WriteString(string(chars))
	}

	return name.String()
}

type fileWriter struct{ path string }

func (w fileWriter) Write(data []byte) (int, error) {
	err := disk.AppendBytes(w.path, data, disk.Private)
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

func registryTests(tree string) (map[string]bool, error) {
	data, err := disk.Read(filepath.Join(tree, "docs/scenarios.yaml"))
	if err != nil {
		return nil, err
	}

	var registry struct {
		Scenarios map[string]any `yaml:"scenarios"`
	}

	err = yaml.Unmarshal(data, &registry)
	if err != nil {
		return nil, fmt.Errorf("reading oracle Registry: %w", err)
	}

	names := map[string]bool{}
	for id := range registry.Scenarios {
		names[scenarioTestName(id)] = true
	}

	return names, nil
}

//nolint:tagliatelle // go test owns this PascalCase wire format.
type oracleEvent struct {
	Action string `json:"Action"`
	Test   string `json:"Test"`
	Output string `json:"Output"`
}
