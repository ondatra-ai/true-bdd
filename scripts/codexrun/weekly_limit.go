package codexrun

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// WeeklyLimit is the newest unexpired Codex weekly token-count event.
type WeeklyLimit struct {
	UsedPercent float64 `json:"used_percent"`
	ResetsAt    int64   `json:"resets_at"`
}

func (w WeeklyLimit) String() string {
	return fmt.Sprintf("%g%% used; resets at %s", w.UsedPercent, time.Unix(w.ResetsAt, 0).UTC().Format(time.RFC3339))
}

type limitWindow struct {
	UsedPercent   *float64 `json:"used_percent"`
	ResetsAt      *int64   `json:"resets_at"`
	WindowMinutes int      `json:"window_minutes"`
}

type limitEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Payload   struct {
		Type       string `json:"type"`
		RateLimits struct {
			LimitID   string       `json:"limit_id"`
			Primary   *limitWindow `json:"primary"`
			Secondary *limitWindow `json:"secondary"`
		} `json:"rate_limits"`
	} `json:"payload"`
}

func weeklyLimit() *WeeklyLimit {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		home = defaultCodexHome()
	}

	var (
		latest    *limitWindow
		timestamp time.Time
	)

	_ = filepath.WalkDir(filepath.Join(home, "sessions"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fs.SkipDir
		}

		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}

		window, at := fileLimit(path)
		if window != nil && !at.Before(timestamp) {
			latest = window
			timestamp = at
		}

		return nil
	})

	return latest.current()
}

func defaultCodexHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".codex")
}

func fileLimit(path string) (*limitWindow, time.Time) {
	data, _ := disk.Read(path)

	var (
		latest    *limitWindow
		timestamp time.Time
	)

	for line := range strings.SplitSeq(string(data), "\n") {
		var event limitEvent
		if json.Unmarshal([]byte(line), &event) != nil || event.Timestamp.Before(timestamp) {
			continue
		}

		if window := event.weekly(); window != nil {
			latest = window
			timestamp = event.Timestamp
		}
	}

	return latest, timestamp
}

func (e limitEvent) weekly() *limitWindow {
	limits := e.Payload.RateLimits
	if e.Type != "event_msg" || e.Payload.Type != "token_count" || e.Timestamp.IsZero() {
		return nil
	}

	if limits.LimitID != "" && limits.LimitID != agentCodex {
		return nil
	}

	const weekMinutes = 10080
	for _, window := range []*limitWindow{limits.Primary, limits.Secondary} {
		if window != nil && window.WindowMinutes == weekMinutes {
			return window
		}
	}

	return nil
}

func (w *limitWindow) current() *WeeklyLimit {
	if w == nil || w.ResetsAt == nil || w.UsedPercent == nil {
		return nil
	}

	if *w.ResetsAt <= time.Now().Unix() || *w.UsedPercent < 0 {
		return nil
	}

	return &WeeklyLimit{UsedPercent: *w.UsedPercent, ResetsAt: *w.ResetsAt}
}
