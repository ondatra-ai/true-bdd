package main

import (
	"strings"
)

// stemFromLog derives which checklist a run executed from the first
// segment's "Loaded prompts" command field, as the hyphenated command
// stem ("us apply 99.3 --fix" → "us-apply"); empty means no checklist command ran.
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
