package main

import (
	"strings"
)

// stemFromLog derives which checklist a run executed from the first
// segment's "Loaded prompts" command field, as the hyphenated command stem
// ("us apply 99.3 --fix" → "us-apply"). Empty means the run executed no
// checklist command (e.g. help-flag).
//
// The run's own log is the only source. The stem used to be read from a
// `cmd:` field in the fixture manifest first, but the invocation is
// behaviour and lives in the scenario now — and a scenario is not reachable
// from a retained run directory, whereas the log the run wrote always is.
func stemFromLog(segments []logSegment) string {
	for _, seg := range segments {
		for _, event := range seg.Events {
			if event.Kind == evLoadedPrompts && event.Command != "" {
				return strings.ReplaceAll(event.Command, " ", "-")
			}
		}
	}

	return ""
}
