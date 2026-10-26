package codexrun

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/codex"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func (r Runner) attempt(ctx context.Context, record Record, note string) (Record, error) {
	prompt, err := r.prompt(record, note)
	if err != nil {
		return r.finish(record, "failed", err.Error(), -1)
	}

	record.WeeklyLimit = weeklyLimit()
	r.announce(record)

	login := codex.CheckLogin(ctx, r.Root, r.preparationWatch())
	if ctx.Err() != nil {
		return r.finish(record, "failed", ctx.Err().Error(), -1)
	}

	if login != "" {
		record.CodexStatus = login

		return r.finish(record, "unavailable", "Codex sign-in check: "+login+"; Claude takes the work", -1)
	}

	err = r.prepareWorkspace(ctx, record)
	if err != nil {
		return r.finish(record, "failed", err.Error(), -1)
	}

	err = r.saveIssues(ctx, record)
	if err != nil {
		return r.finish(record, "failed", err.Error(), -1)
	}

	err = r.save(record)
	if err != nil {
		return record, err
	}

	began := time.Now()

	result, err := r.execute(ctx, &record, prompt, began)
	if err != nil {
		return r.finish(record, codex.FailureStatus(err), err.Error(), result.Code)
	}

	return r.completed(record, result.Code)
}

func (r Runner) prompt(record Record, note string) (string, error) {
	brief, err := disk.Read(r.file(record.Name, "brief.md"))
	if err != nil {
		return "", err
	}

	prompt := string(brief)

	if record.Attempt > 1 {
		if record.SessionID != "" {
			prompt = ""
		}

		prompt += "\n\n" + note + "\n"

		err = disk.Write(r.file(record.Name, "resume.md"), []byte(prompt), disk.Private)
		if err != nil {
			return "", err
		}
	}

	return prompt, nil
}

func (r Runner) execute(
	ctx context.Context, record *Record, prompt string, began time.Time,
) (codex.Result, error) {
	opts := codex.Options{Dir: record.Workdir, Final: record.Final, Writer: record.writer(), Search: record.Search,
		Model: record.Model, Effort: record.Effort,
		Output: logWriter{path: record.Log}, Watchdog: r.remaining(began)}

	if record.writer() {
		opts.RunMarker = r.marker(*record)
	}

	var (
		result codex.Result
		err    error
	)
	if record.SessionID != "" {
		result, err = codex.ResumeRun(ctx, record.SessionID, prompt, opts)
	} else {
		result, err = codex.StartRun(ctx, prompt, opts)
	}

	if err != nil || result.Code == 0 {
		return result, err
	}

	output, _ := disk.Read(record.Log)

	terminal := codex.TerminalStatus(string(output))
	if terminal == "" {
		return result, nil
	}

	record.CodexStatus = terminal

	return result, nil
}

func (r Runner) remaining(began time.Time) codex.Watchdog {
	watch := r.watch()
	watch.Limit = max(time.Nanosecond, r.Limit-time.Since(began))

	return watch
}

func (r Runner) completed(record Record, code int) (Record, error) {
	if record.CodexStatus != "" {
		return r.finish(record, "unavailable", "Codex ended "+record.CodexStatus+"; Claude takes the work", code)
	}

	if code != 0 {
		return r.finish(record, "failed", fmt.Sprintf("%s exited %d; read the log", record.Agent, code), code)
	}

	final, _ := disk.Read(record.Final)
	if strings.TrimSpace(string(final)) == "" {
		return r.finish(record, "failed", "agent left no final message", code)
	}

	return r.finish(record, statusDone, "", code)
}
