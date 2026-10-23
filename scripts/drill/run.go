package drill

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"regexp"
	"time"

	drillcli "github.com/ondatra-ai/true-bdd/pkg/cli/drill"
	"github.com/ondatra-ai/true-bdd/pkg/cli/ps"
	"github.com/ondatra-ai/true-bdd/pkg/cli/truebdd"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Run records one command, including its operator pauses and cleanup evidence.
type Run struct {
	Tag             string     `json:"tag"`
	Cut             string     `json:"cut"`
	Base            string     `json:"base"`
	Digest          string     `json:"digest"`
	Args            []string   `json:"args"`
	Marker          string     `json:"marker"`
	Status          string     `json:"status"`
	WorkerPID       int        `json:"worker_pid"`
	Log             string     `json:"log"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         time.Time  `json:"ended_at"`
	ExitCode        *int       `json:"exit_code"`
	Reason          string     `json:"reason,omitempty"`
	CleanupError    string     `json:"cleanup_error,omitempty"`
	CleanupFailures []string   `json:"cleanup_failures,omitempty"`
	Questions       []Question `json:"questions"`
	Oracle          *Oracle    `json:"oracle,omitempty"`
}

// Prepare reserves a tag before any process starts.
func (r Runner) Prepare(ctx context.Context, directory, cut, tag string, args []string) (Run, error) {
	record, err := Read(directory)
	if err != nil {
		return Run{}, err
	}

	_, err = record.cut(cut)
	if err != nil {
		return Run{}, err
	}

	if !regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`).MatchString(tag) || len(args) == 0 {
		return Run{}, fmt.Errorf("%w: invalid tag or empty command", errRefused)
	}

	const markerBytes = 16

	token := make([]byte, markerBytes)

	_, err = rand.Read(token)
	if err != nil {
		return Run{}, fmt.Errorf("minting marker: %w", err)
	}

	run := Run{Tag: tag,
		Cut:    cut,
		Base:   record.Base,
		Digest: record.Digest,
		Args:   args,
		Marker: hex.EncodeToString(token),
		Status: "starting",
		Log: filepath.Join(directory,
			"runs",
			tag,
			"engine.log"),
		Questions: []Question{}}

	err = disk.Ensure(run.Log, disk.Private)
	if err != nil {
		return run, err
	}

	err = change(directory, func(record *Record) error {
		if _, exists := record.Runs[tag]; exists {
			return fmt.Errorf("%w: tag already exists", errRefused)
		}

		for _, prior := range record.Runs {
			if prior.EndedAt.IsZero() {
				return fmt.Errorf("%w: another run is not finished", errRefused)
			}
		}

		record.Runs[tag] = run

		return nil
	})
	_ = ctx

	return run, err
}

// Launch copies the worker so a go-run temporary executable may disappear safely.
func (r Runner) Launch(ctx context.Context, directory, tag, executable string) error {
	worker := filepath.Join(directory, "runs", tag, "worker")

	err := disk.Copy(worker, executable, disk.Private)
	if err == nil {
		err = disk.Executable(worker)
	}

	if err != nil {
		return r.finish(directory, tag, -1, err)
	}

	record, err := loadUnchecked(directory)
	if err != nil {
		return r.finish(directory, tag, -1, err)
	}

	pid, err := drillcli.StartSupervisor(ctx, worker, directory, tag, record.Runs[tag].Marker)
	if err != nil {
		return r.finish(directory, tag, -1, err)
	}

	return changeRun(directory, tag, func(run *Run) { run.WorkerPID = pid })
}

// Supervise owns the deadline until all marked descendants are gone.
func (r Runner) Supervise(ctx context.Context, directory, tag string) error {
	record, err := loadUnchecked(directory)
	if err != nil {
		return err
	}

	run, ok := record.Runs[tag]
	if !ok {
		return fmt.Errorf("%w: unknown tag", errRefused)
	}

	err = r.claim(directory, tag)
	if err != nil {
		return err
	}

	code, runErr := r.execute(ctx, record, run)

	return r.finish(directory, tag, code, runErr)
}

func (r Runner) claim(directory, tag string) error {
	return change(directory, func(record *Record) error {
		run := record.Runs[tag]
		if run.Status != "starting" {
			return fmt.Errorf("%w: run already claimed", errRefused)
		}

		run.StartedAt = time.Now().UTC()
		run.Status = "running"
		record.Runs[tag] = run

		return nil
	})
}

func (r Runner) execute(ctx context.Context, record Record, run Run) (int, error) {
	_, err := Read(record.Directory)
	if err != nil {
		return -1, err
	}

	cut, err := record.cut(run.Cut)
	if err != nil {
		return -1, err
	}

	if run.Base != record.Base || run.Digest != record.Digest {
		return -1, fmt.Errorf("%w: run base or binary mismatch", errRefused)
	}

	ctx, cancel := context.WithTimeout(ctx, r.Limit)

	defer cancel()
	go r.watchStop(ctx, cancel, record.Directory, run.Tag)

	if run.Oracle != nil {
		return r.executeOracle(ctx, record, run, cut)
	}

	input, answer := io.Pipe()
	defer func() { _ = input.Close(); _ = answer.Close() }()

	writer := &promptWriter{ctx: ctx,
		cancel:    cancel,
		runner:    r,
		directory: record.Directory,
		tag:       run.Tag,
		tree:      cut.Directory,
		log:       run.Log,
		input:     answer}

	code, err := truebdd.Rebuild(ctx,
		record.Binary,
		run.Args,
		truebdd.Options{Directory: cut.Directory,
			Marker: run.Marker,
			Input:  input,
			Output: writer,
			Limit:  r.Limit,
			Grace:  r.Grace})

	return code, errors.Join(err, writer.failure)
}

func (r Runner) watchStop(ctx context.Context, cancel context.CancelFunc, directory, tag string) {
	ticker := time.NewTicker(r.Poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := disk.Read(filepath.Join(directory, "runs", tag, "stop"))
			if err == nil {
				cancel()

				return
			}
		}
	}
}

func (r Runner) finish(directory, tag string, code int, reason error) error {
	record, err := loadUnchecked(directory)
	if err != nil {
		return errors.Join(reason, err)
	}

	run := record.Runs[tag]

	const cleanupLimit = 30 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), cleanupLimit)
	defer cancel()

	cleanup := ps.StopMarked(ctx, "TRUE_BDD_DRILL_RUN", run.Marker, r.Grace)

	err = changeRun(directory, tag, func(run *Run) {
		if reason != nil {
			run.Reason = reason.Error()
		}

		if cleanup != nil {
			run.Status = "cleanup_failed"
			run.CleanupError = cleanup.Error()
			run.CleanupFailures = append(run.CleanupFailures, cleanup.Error())

			return
		}

		run.CleanupError = ""

		run.Status = "done"
		if reason != nil || code != 0 {
			run.Status = "failed"
		}

		run.EndedAt = time.Now().UTC()
		run.ExitCode = &code
	})
	if cleanup != nil {
		slog.Error("Drill cleanup failed", "iteration", directory, "tag", tag, "error", cleanup)
	} else {
		slog.Info("Drill run ended", "iteration", directory, "tag", tag, "exit_code", code, "error", reason)
	}

	return errors.Join(reason, cleanup, err)
}

func changeRun(directory, tag string, edit func(*Run)) error {
	return change(directory, func(record *Record) error {
		run, ok := record.Runs[tag]
		if !ok {
			return fmt.Errorf("%w: unknown run", errRefused)
		}

		edit(&run)
		record.Runs[tag] = run

		return nil
	})
}
