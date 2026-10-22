package codexrun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func (r Runner) claim(ctx context.Context, opts Options, resume bool) (Record, error) {
	err := r.localArtifacts()
	if err != nil {
		return Record{}, err
	}

	err = disk.Dir(r.file("", ""), disk.Private)
	if err != nil {
		return Record{}, err
	}

	before, err := r.records()
	if err != nil {
		return Record{}, err
	}

	record, err := r.candidate(ctx, opts, resume, before)
	if err != nil {
		return Record{}, err
	}

	err = disk.Update(r.file("", ".registry"), disk.Private, func(_ []byte) ([]byte, error) {
		records, err := r.records()
		if err != nil {
			return nil, err
		}

		err = r.recheck(record, resume, before, records)
		if err != nil {
			return nil, err
		}

		err = available(record, records)
		if err != nil {
			return nil, err
		}

		err = r.publish(&record)
		if err != nil {
			return nil, err
		}

		return []byte(time.Now().UTC().Format(time.RFC3339Nano)), nil
	})

	return record, err
}

func (r Runner) candidate(ctx context.Context, opts Options, resume bool, records []Record) (Record, error) {
	path := r.file(opts.Name, "run.json")
	if resume {
		record, err := load(path)
		if err != nil {
			return record, refuse("no readable run named " + opts.Name)
		}

		err = canResume(record)
		if err != nil {
			return record, err
		}

		err = r.checkWriter(ctx, record)
		if err != nil {
			return record, err
		}

		return record, nil
	}

	_, err := os.Stat(filepath.Dir(path))
	if err == nil {
		return Record{}, refuse("run name already exists")
	}

	record := Record{Options: opts, Agent: agentCodex, Workdir: r.Root}
	if opts.writer() {
		return r.writerRecord(ctx, record, records)
	}

	return record, nil
}

func canResume(record Record) error {
	if record.Status == statusRunning && alive(record.PID) {
		return refuse("run is still running")
	}

	if record.Status == "done" {
		return refuse("run is already done")
	}

	if record.Status == "unavailable" && !record.writer() {
		return refuse("Claude does unavailable readings itself")
	}

	if record.Attempt >= maxAttempts {
		return refuse("two attempts used; Claude takes the work")
	}

	return nil
}

func available(record Record, records []Record) error {
	if !record.writer() {
		return nil
	}

	const maxWriters = 4

	count := 0

	for _, other := range records {
		if other.Status != statusRunning || !alive(other.PID) || !other.writer() {
			continue
		}

		if other.Worktree == record.Worktree || other.Branch == record.Branch && record.Branch != "" {
			return refuse("another running command holds this worktree")
		}

		count++
	}

	if count >= maxWriters {
		return refuse("four writers are already running")
	}

	return nil
}

func alive(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)

	return err == nil || errors.Is(err, syscall.EPERM)
}

func (r Runner) records() ([]Record, error) {
	entries, err := os.ReadDir(r.file("", ""))
	if err != nil {
		return nil, fmt.Errorf("listing run records: %w", err)
	}

	records := make([]Record, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		record, readErr := load(r.file(entry.Name(), "run.json"))
		if readErr == nil {
			records = append(records, record)
		}
	}

	return records, nil
}

func (r Runner) localArtifacts() error {
	for _, path := range []string{r.file("", ""), filepath.Join(r.Root, "tmp", "codex-worktrees")} {
		ancestor := path
		for {
			_, err := os.Lstat(ancestor)
			if err == nil {
				break
			}

			parent := filepath.Dir(ancestor)
			if parent == ancestor {
				return refuse("cannot resolve artifact path")
			}

			ancestor = parent
		}

		resolved, err := filepath.EvalSymlinks(ancestor)
		if err != nil {
			return refuse(err.Error())
		}

		relative, err := filepath.Rel(r.Root, resolved)
		if err != nil || !filepath.IsLocal(relative) && relative != "." {
			return refuse("artifact path escapes the checkout")
		}
	}

	return nil
}

func (r Runner) publish(record *Record) error {
	record.Attempt++
	record.PID = os.Getpid()
	record.Status = statusRunning
	record.Reason = ""
	record.ExitCode = -1
	record.StartedAt = time.Now().UTC()
	record.EndedAt = time.Time{}

	suffix := ""
	if record.Attempt > 1 {
		suffix = fmt.Sprintf("-%d", record.Attempt)
	}

	record.Log = r.file(record.Name, "run"+suffix+".log")

	record.Final = r.file(record.Name, "final"+suffix+".md")

	err := disk.Write(record.Log, nil, disk.Private)
	if err != nil {
		return err
	}

	return r.save(*record)
}
