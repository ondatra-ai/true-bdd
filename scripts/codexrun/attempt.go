package codexrun

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/codex"
	"github.com/ondatra-ai/true-bdd/pkg/cli/crush"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func (r Runner) attempt(ctx context.Context, record Record, note string) (Record, error) {
	prompt, err := r.prompt(record, note)
	if err != nil {
		return r.finish(record, "failed", err.Error(), -1)
	}

	record.WeeklyLimit = weeklyLimit()
	r.announce(record)

	login := codex.CheckLogin(ctx, r.Root, r.watch())
	if ctx.Err() != nil {
		return r.finish(record, "failed", ctx.Err().Error(), -1)
	}

	if login != "" && !record.writer() {
		record.CodexStatus = login

		return r.finish(record, "unavailable", "Codex sign-in check: "+login, -1)
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

	result, err := r.execute(ctx, &record, prompt, note, login, began)
	if err != nil {
		return r.finish(record, codex.FailureStatus(err), err.Error(), result.Code)
	}

	return r.completed(ctx, record, result.Code)
}

func (r Runner) prompt(record Record, note string) (string, error) {
	brief, err := disk.Read(r.file(record.Name, "brief.md"))
	if err != nil {
		return "", err
	}

	prompt := string(brief)

	if record.Attempt > 1 {
		if record.SessionID != "" && record.Agent != agentCrush {
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
	ctx context.Context, record *Record, prompt, note, login string, began time.Time,
) (codex.Result, error) {
	if login != "" || record.Agent == agentCrush {
		return r.takeover(ctx, record, note, login, began)
	}

	opts := codex.Options{Dir: record.Workdir, Final: record.Final, Writer: record.writer(), Search: record.Search,
		Output: logWriter{path: record.Log}, Watchdog: r.remaining(began)}

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
	if !record.writer() {
		return result, nil
	}

	return r.takeover(ctx, record, note, terminal, began)
}

func (r Runner) remaining(began time.Time) codex.Watchdog {
	watch := r.watch()
	watch.Limit = max(time.Nanosecond, r.Limit-time.Since(began))

	return watch
}

func (r Runner) takeover(
	ctx context.Context, record *Record, note, reason string, began time.Time,
) (codex.Result, error) {
	record.Agent = agentCrush

	record.SessionID = ""
	if reason != "" {
		record.CodexStatus = reason
	}

	err := r.save(*record)
	if err != nil {
		return codex.Result{Code: -1}, err
	}

	brief, err := disk.Read(r.file(record.Name, "brief.md"))
	if err != nil {
		return codex.Result{Code: -1}, err
	}

	prompt := string(brief) + "\n\n" + note +
		"\nAn earlier agent may have left work half-done; start by reading git status and the diff before continuing.\n"

	filename := "crush.md"
	if record.Attempt > 1 {
		filename = fmt.Sprintf("crush-%d.md", record.Attempt)
	}

	err = disk.Write(r.file(record.Name, filename), []byte(prompt), disk.Private)
	if err != nil {
		return codex.Result{Code: -1}, err
	}

	err = disk.Write(record.Final, nil, disk.Private)
	if err != nil {
		return codex.Result{Code: -1}, err
	}

	return crush.WriteCode(ctx, prompt, crush.Options{Dir: record.Workdir,
		Output: logWriter{path: record.Log}, Report: logWriter{path: record.Final}, Watchdog: r.remaining(began)})
}

func (r Runner) completed(ctx context.Context, record Record, code int) (Record, error) {
	if record.CodexStatus != "" && !record.writer() {
		return r.finish(record, "unavailable", "Codex ended "+record.CodexStatus+"; Claude does the reading", code)
	}

	if code != 0 {
		return r.finish(record, "failed", fmt.Sprintf("%s exited %d; read the log", record.Agent, code), code)
	}

	final, _ := disk.Read(record.Final)
	if strings.TrimSpace(string(final)) == "" {
		return r.finish(record, "failed", "agent left no final message", code)
	}

	if record.writer() {
		problem := r.repository(record.Worktree).CompletionProblem(ctx, record.Branch, record.Since)
		if problem != "" {
			return r.finish(record, "incomplete", problem, code)
		}
	}

	return r.finish(record, "done", "", code)
}
