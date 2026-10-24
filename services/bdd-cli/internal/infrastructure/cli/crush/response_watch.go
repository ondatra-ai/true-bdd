package crush

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

const responseLimit = 10 * time.Minute

// ResponseWatch consumes one turn's log and times only its open model response.
// Check takes the clock explicitly so callers can exercise response boundaries.
type ResponseWatch struct {
	LogPath string
	Label   string
	offset  int
	opened  time.Time
	stopped bool
}

// Check reports whether the process group must stop, warning only once.
func (w *ResponseWatch) Check(now time.Time) bool {
	if w.stopped {
		return true
	}

	data, err := disk.Read(w.LogPath)
	if err != nil {
		return false
	}

	if len(data) < w.offset {
		w.offset = 0
		w.opened = time.Time{}
	}

	for {
		end := bytes.IndexByte(data[w.offset:], '\n')
		if end < 0 {
			break
		}

		line := data[w.offset : w.offset+end]
		w.offset += end + 1
		w.record(line, now)
	}

	if w.opened.IsZero() || now.Sub(w.opened) < responseLimit {
		return false
	}

	w.stopped = true
	slog.Warn("Writer response stalled", "label", w.Label, "minutes", now.Sub(w.opened).Minutes(), "log_path", w.LogPath)

	return true
}

func (w *ResponseWatch) record(line []byte, now time.Time) {
	var record struct {
		Time    time.Time `json:"time"`
		Message string    `json:"msg"`
	}
	if json.Unmarshal(line, &record) != nil {
		return
	}

	w.opened = time.Time{}
	if record.Message == "ModelProvider called" {
		w.opened = record.Time
		if w.opened.IsZero() {
			w.opened = now
		}
	}
}
