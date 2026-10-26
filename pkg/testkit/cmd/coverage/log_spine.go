package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// logEventKind enumerates the slog records the spine cares about.
type logEventKind string

// Spine event kinds mapped from the exact production slog literals.
const (
	evLoadedPrompts logEventKind = "loaded_prompts"
	evResultSaved   logEventKind = "result_saved"
	evFixGenerated  logEventKind = "fix_generated"
	evFixApplied    logEventKind = "fix_applied"
	evWarnProtocol  logEventKind = "warn_protocol"
	evCodeDiscovery logEventKind = "code_discovery"
	evCodeComplete  logEventKind = "code_complete"
)

// logEvent is one relevant, ordered record from the slog JSON stream.
type logEvent struct {
	Kind        logEventKind
	Subject     string // raw (unsanitized) subject id where present
	PromptIndex int    // 1-based, for fix generation events
	File        string // file path field where present
	Command     string // for loaded_prompts
	Items       int
	Prompts     int
	Failures    int
}

// logSegment is the event slice of one CLI invocation, bound to the
// tmp/<ts> partition its file paths reference.
type logSegment struct {
	Partition string // partition basename, "" when never referenced
	Events    []logEvent
}

// partitionPathRe recovers the partition basename from a logged file
// path; must track partitionRe in run_scanner.go, including its
// backward-compat match for older, minute-granular retained runs.
var partitionPathRe = regexp.MustCompile(`tmp/(\d{4}-\d{2}-\d{2}-\d{2}-\d{2}(?:-\d{2}(?:-\d+)?)?)/`)

// parseLogSpine reads the slog JSON stream and splits it into one
// segment per "Loaded prompts" boundary. A zero-byte log yields nil;
// the second return value counts malformed (non-JSON, non-empty) lines.
func parseLogSpine(path string) ([]logSegment, int, error) {
	raw, err := disk.Read(path)
	if err != nil {
		return nil, 0, fmt.Errorf("opening log %s: %w", path, err)
	}

	const (
		logScanBufInit = 1 << 20
		logScanBufMax  = 16 << 20
	)

	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, logScanBufInit), logScanBufMax)

	segments := make([]logSegment, 0)
	malformed := 0

	for scanner.Scan() {
		event, ok, bad := parseLogLine(scanner.Bytes())
		if bad {
			malformed++
		}

		if !ok {
			continue
		}

		if event.Kind == evLoadedPrompts || len(segments) == 0 {
			segments = append(segments, logSegment{})
		}

		seg := &segments[len(segments)-1]
		seg.Events = append(seg.Events, event)

		if m := partitionPathRe.FindStringSubmatch(event.File); m != nil {
			if seg.Partition == "" {
				seg.Partition = m[1]
			}
		}
	}

	err = scanner.Err()
	if err != nil {
		return nil, malformed, fmt.Errorf("scanning log %s: %w", path, err)
	}

	return segments, malformed, nil
}

// rawLogLine is the superset of fields the spine reads from one record.
type rawLogLine struct {
	Failures    *int    `json:"failures"`
	Msg         string  `json:"msg"`
	File        string  `json:"file"`
	Path        string  `json:"path"`
	SubjectID   string  `json:"subjectID"`   //nolint:tagliatelle // upstream slog field name
	PromptIndex float64 `json:"promptIndex"` //nolint:tagliatelle // upstream slog field name
	Command     string  `json:"command"`
	Items       float64 `json:"items"`
	Prompts     float64 `json:"prompts"`
}

// parseLogLine maps one JSON record to a spine event. The last return
// value marks a malformed line (non-empty but not JSON) as distinct
// from a merely irrelevant record.
func parseLogLine(line []byte) (logEvent, bool, bool) {
	if len(strings.TrimSpace(string(line))) == 0 {
		return logEvent{}, false, false
	}

	var raw rawLogLine

	err := json.Unmarshal(line, &raw)
	if err != nil {
		return logEvent{}, false, true
	}

	if raw.File == "" {
		raw.File = raw.Path
	}

	return spineEvent(raw)
}

func spineEvent(raw rawLogLine) (logEvent, bool, bool) {
	filePath := raw.File

	switch raw.Msg {
	case "Total failing tests across architecture":
		if raw.Failures == nil {
			return logEvent{}, false, false
		}

		return logEvent{Kind: evCodeDiscovery, Failures: *raw.Failures}, true, false
	case "BUILD CODE COMPLETE":
		return logEvent{Kind: evCodeComplete}, true, false
	case "Loaded prompts":
		return logEvent{Kind: evLoadedPrompts, Command: raw.Command,
			Items: int(raw.Items), Prompts: int(raw.Prompts)}, true, false
	case "Result file saved":
		return logEvent{Kind: evResultSaved, File: filePath}, true, false
	case "Generating fix prompt":
		return logEvent{Kind: evFixGenerated, Subject: raw.SubjectID,
			PromptIndex: int(raw.PromptIndex)}, true, false
	case "Fix applied successfully":
		return logEvent{Kind: evFixApplied, Subject: raw.SubjectID}, true, false
	// One ERROR now replaces the two Warns this keyed on: a protocol
	// accident aborts the walk instead of grading a fabricated fail.
	case enginelog.MsgAnswerUnusable:
		return logEvent{Kind: evWarnProtocol, File: filePath}, true, false
	default:
		return logEvent{}, false, false
	}
}

// applyAttribution is the consumed-state correlation of applies to the
// latest unconsumed fix generation per subject within one segment.
type applyAttribution struct {
	Subject     string
	PromptIndex int // 0 when unattributed
	EventIndex  int // position of the apply event within the segment
}

// attributeApplies walks a segment and pairs every apply with the
// latest unconsumed generation for the same subject.
func attributeApplies(seg logSegment) []applyAttribution {
	pending := map[string]int{} // subject -> latest unconsumed prompt index
	attributions := make([]applyAttribution, 0)

	for i, event := range seg.Events {
		switch event.Kind {
		case evFixGenerated:
			pending[event.Subject] = event.PromptIndex
		case evFixApplied:
			attr := applyAttribution{Subject: event.Subject, EventIndex: i}

			if idx, ok := pending[event.Subject]; ok {
				attr.PromptIndex = idx

				delete(pending, event.Subject)
			}

			attributions = append(attributions, attr)
		case evLoadedPrompts, evResultSaved, evWarnProtocol, evCodeDiscovery, evCodeComplete:
		}
	}

	return attributions
}

// cleanWalkProven reports a complete clean walk after the last apply:
// strictly more result saves than one items×prompts pass (see the
// maxAttempts case in TestCleanWalkProven) and no fix generations after it.
func cleanWalkProven(seg logSegment) bool {
	lastApply, items, prompts := walkShape(seg)
	if lastApply == -1 || items == 0 || prompts == 0 {
		return false
	}

	events := seg.Events[lastApply+1:]

	saves, generated := postApplyActivity(events)
	if generated {
		return false
	}

	if seg.Events[0].Command == "build code" {
		return codeDiscoveryConverged(events)
	}

	return saves > items*prompts
}

// walkShape extracts the last apply position and the items×prompts
// dimensions from a segment.
func walkShape(seg logSegment) (int, int, int) {
	lastApply := -1
	items, prompts := 0, 0

	for eventIdx, event := range seg.Events {
		switch event.Kind {
		case evLoadedPrompts:
			items, prompts = event.Items, event.Prompts
		case evFixApplied:
			lastApply = eventIdx
		case evFixGenerated, evResultSaved, evWarnProtocol, evCodeDiscovery, evCodeComplete:
		}
	}

	return lastApply, items, prompts
}

// postApplyActivity counts result saves and detects fix generations
// after the last apply.
func postApplyActivity(events []logEvent) (int, bool) {
	saves := 0

	for _, event := range events {
		switch event.Kind {
		case evFixGenerated, evWarnProtocol:
			// A further repair attempt or ANY protocol failure after
			// the last apply means the post-apply walk was not clean.
			return saves, true
		case evResultSaved:
			saves++
		case evLoadedPrompts, evFixApplied, evCodeDiscovery, evCodeComplete:
		}
	}

	return saves, false
}

// codeDiscoveryConverged requires fresh full discovery and completion, not a restart alone.
func codeDiscoveryConverged(events []logEvent) bool {
	green, complete := false, false

	for _, event := range events {
		switch event.Kind {
		case evCodeDiscovery:
			green, complete = event.Failures == 0, false
		case evCodeComplete:
			complete = green
		case evLoadedPrompts, evResultSaved, evFixGenerated, evFixApplied, evWarnProtocol:
		}
	}

	return complete
}
