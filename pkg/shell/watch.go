package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"
)

// ErrStalled means neither output stream grew within the stall window.
var ErrStalled = errors.New("command stalled")

// Watchdog bounds silence, total execution and process-group shutdown.
type Watchdog struct {
	Stall time.Duration
	Limit time.Duration
	Grace time.Duration
}

// Watch runs a command with a watchdog and stops its entire process group,
// including descendants left behind after a successful exit.
func Watch(ctx context.Context, argv []string, opt Options, watch Watchdog) (Result, error) {
	opt.Group = true
	opt.WaitDelay = watch.Grace

	cmd, err := newCommand(context.WithoutCancel(ctx), argv, opt)
	if err != nil {
		return Result{Code: NotStarted}, err
	}

	stdout, stderr := opt.Output.wire(cmd)

	var activity atomic.Int64
	activity.Store(time.Now().UnixNano())
	cmd.Stdout = activityWriter{target: cmd.Stdout, activity: &activity}
	cmd.Stderr = activityWriter{target: cmd.Stderr, activity: &activity}

	logSpawn(argv, opt.Dir)

	err = cmd.Start()
	if err != nil {
		return Result{Code: NotStarted}, fmt.Errorf("%w: %s: %w", ErrNotStarted, label(argv), err)
	}

	ended := make(chan error, 1)
	go func() { ended <- cmd.Wait() }()

	waitErr, stopped := watchProcess(ctx, cmd, ended, &activity, watch)
	if errors.Is(waitErr, exec.ErrWaitDelay) {
		waitErr = nil
	}

	code, signal, _ := classify(waitErr)

	result := Result{Code: code, Signal: signal, Stdout: stdout.String(), Stderr: stderr.String()}
	if stopped != nil {
		return result, fmt.Errorf("%s: %w", label(argv), stopped)
	}

	if waitErr != nil && code == NotStarted && signal == 0 {
		return result, fmt.Errorf("waiting for %s: %w", label(argv), waitErr)
	}

	return result, nil
}

func watchProcess(
	ctx context.Context, cmd *exec.Cmd, ended <-chan error, activity *atomic.Int64, watch Watchdog,
) (error, error) {
	const interval = 10 * time.Millisecond

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	began := time.Now()

	for {
		select {
		case err := <-ended:
			stopGroup(cmd.Process.Pid, watch.Grace)

			return err, nil
		case <-ctx.Done():
			stopGroup(cmd.Process.Pid, watch.Grace)

			return <-ended, fmt.Errorf("cancelled: %w", ctx.Err())
		case now := <-ticker.C:
			reason := watch.expired(began, time.Unix(0, activity.Load()), now)
			if reason != nil {
				stopGroup(cmd.Process.Pid, watch.Grace)

				return <-ended, reason
			}
		}
	}
}

func (w Watchdog) expired(began, grew, now time.Time) error {
	if w.Limit > 0 && now.Sub(began) >= w.Limit {
		return ErrTimeout
	}

	if w.Stall > 0 && now.Sub(grew) >= w.Stall {
		return ErrStalled
	}

	return nil
}

func stopGroup(pid int, grace time.Duration) {
	if syscall.Kill(-pid, syscall.SIGTERM) != nil {
		return
	}

	const interval = 10 * time.Millisecond

	for end := time.Now().Add(grace); time.Now().Before(end); {
		if syscall.Kill(-pid, 0) != nil {
			return
		}

		time.Sleep(interval)
	}

	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

type activityWriter struct {
	target   io.Writer
	activity *atomic.Int64
}

func (w activityWriter) Write(data []byte) (int, error) {
	w.activity.Store(time.Now().UnixNano())

	count, err := w.target.Write(data)
	if err != nil {
		return count, fmt.Errorf("writing subprocess output: %w", err)
	}

	return count, nil
}
