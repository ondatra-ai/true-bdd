package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	spoolFilePerm   = 0o600
	spoolSuffix     = "-spool.json"
	spoolTempSuffix = "-spool.json.tmp"
	eventTypeGap    = "gap"
)

// spoolState is the durable per-run outbox journal (plan §3.6). It records
// the seq high-water mark, the server's contiguous acked_through and
// answers_consumed_through watermarks, whether the terminal event has been
// acked, and the still-unacked events (contiguous by seq, possibly
// including RANGE TOMBSTONE gap events from head+tail truncation).
type spoolState struct {
	Seq                    int        `json:"seq"`
	AckedThrough           int        `json:"acked_through"`
	AnswersConsumed        int        `json:"answers_consumed"`
	AnswersConsumedThrough int        `json:"answers_consumed_through"`
	TerminalAcked          bool       `json:"terminal_acked"`
	Events                 []OutEvent `json:"events"`
}

// spool persists a spoolState atomically to a per-run file. The file is
// deleted ONLY after the terminal ack is recorded (plan §3.6); an unacked
// spool survives a crash and is replayed on the next startup.
type spool struct {
	path string
}

// newSpool builds a spool backed by <dir>/<runID>-spool.json.
func newSpool(dir, runID string) *spool {
	return &spool{path: filepath.Join(dir, runID+spoolSuffix)}
}

// load reads the persisted state, returning a zero state when absent.
func (s *spool) load() (spoolState, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return spoolState{}, nil
		}

		return spoolState{}, fmt.Errorf("read spool %s: %w", s.path, err)
	}

	var state spoolState

	err = json.Unmarshal(data, &state)
	if err != nil {
		return spoolState{}, fmt.Errorf("parse spool %s: %w", s.path, err)
	}

	return state, nil
}

// save atomically rewrites the spool (temp file + rename) so a crash
// never leaves a torn journal.
func (s *spool) save(state spoolState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal spool: %w", err)
	}

	temp := strings.TrimSuffix(s.path, spoolSuffix) + spoolTempSuffix

	err = os.WriteFile(temp, data, spoolFilePerm)
	if err != nil {
		return fmt.Errorf("write spool temp: %w", err)
	}

	err = os.Rename(temp, s.path)
	if err != nil {
		return fmt.Errorf("rename spool: %w", err)
	}

	return nil
}

// remove deletes the spool file (after the terminal ack is recorded).
func (s *spool) remove() {
	_ = os.Remove(s.path)
}

// eventBytes is the on-wire size of one event, used for head+tail budget
// accounting.
func eventBytes(event OutEvent) int {
	data, err := json.Marshal(event)
	if err != nil {
		return 0
	}

	return len(data)
}

// truncateUnacked bounds the unacked event journal to a byte budget by
// collapsing the MIDDLE into a single RANGE TOMBSTONE that covers the
// dropped seq span (plan §3.6). The head and tail are retained; the gap
// participates in contiguous ack so acked_through can still reach the
// terminal seq. A pre-existing gap inside the collapsed middle is merged.
func truncateUnacked(events []OutEvent, budget, headKeep, tailKeep int) []OutEvent {
	if len(events) <= headKeep+tailKeep+1 || totalBytes(events) <= budget {
		return events
	}

	head := events[:headKeep]
	tail := events[len(events)-tailKeep:]
	middle := events[headKeep : len(events)-tailKeep]

	gap := OutEvent{
		Type:         eventTypeGap,
		Seq:          middle[0].Seq,
		ThroughSeq:   middle[len(middle)-1].SpanEnd(),
		DroppedBytes: totalBytes(middle),
	}

	out := make([]OutEvent, 0, len(head)+1+len(tail))
	out = append(out, head...)
	out = append(out, gap)
	out = append(out, tail...)

	return out
}

// totalBytes sums the on-wire size of a batch, adding a gap's dropped
// bytes so repeated truncation accounts for already-dropped payload.
func totalBytes(events []OutEvent) int {
	total := 0
	for _, event := range events {
		total += eventBytes(event) + event.DroppedBytes
	}

	return total
}

// replayUnackedSpools replays every unacked run spool under dir to the
// server, then removes it (plan §3.6 startup replay). The server accepts
// replayed events for any known run_id and dedups by (run_id, seq), so a
// replay is exactly-once in effect. Failures are logged by the caller;
// this returns the first transport error so startup can decide.
func replayUnackedSpools(ctx context.Context, poster eventPoster, sessionID, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil //nolint:nilerr // a missing tmp dir simply means nothing to replay
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), spoolSuffix) {
			continue
		}

		runID := strings.TrimSuffix(entry.Name(), spoolSuffix)

		err = replayOneSpool(ctx, poster, sessionID, dir, runID)
		if err != nil {
			return err
		}
	}

	return nil
}

// replayOneSpool replays a single run's unacked events and removes the
// spool once its terminal event is acked.
func replayOneSpool(ctx context.Context, poster eventPoster, sessionID, dir, runID string) error {
	spoolFile := newSpool(dir, runID)

	state, err := spoolFile.load()
	if err != nil {
		return err
	}

	if state.TerminalAcked || len(state.Events) == 0 {
		spoolFile.remove()

		return nil
	}

	resp, err := poster.PostEvents(ctx, EventsRequest{
		SessionID:       sessionID,
		RunID:           runID,
		Events:          state.Events,
		AnswersConsumed: state.AnswersConsumed,
	})
	if err != nil {
		return err
	}

	applyAck(&state, resp)

	if state.TerminalAcked {
		spoolFile.remove()

		return nil
	}

	return spoolFile.save(state)
}

// applyAck folds a server ack into the spool state: it advances the
// contiguous acked_through and answers_consumed_through watermarks, trims
// events at or below acked_through, and records whether the terminal
// event (the highest-seq event) is now covered.
func applyAck(state *spoolState, resp EventsResponse) {
	if resp.AckedThrough > state.AckedThrough {
		state.AckedThrough = resp.AckedThrough
	}

	if resp.AnswersConsumedThrough > state.AnswersConsumedThrough {
		state.AnswersConsumedThrough = resp.AnswersConsumedThrough
	}

	kept := state.Events[:0]
	terminalCovered := false

	for _, event := range state.Events {
		if event.SpanEnd() <= state.AckedThrough {
			if event.Type == eventTypeTerminal {
				terminalCovered = true
			}

			continue
		}

		kept = append(kept, event)
	}

	state.Events = kept

	if terminalCovered {
		state.TerminalAcked = true
	}
}
