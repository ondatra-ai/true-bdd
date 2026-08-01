package main

import (
	"fmt"
	"strings"
)

// attributeAndChainApplies pairs apply artifacts with the prompt whose
// failure triggered them, then checks the full fix-effective chain.
func (s *partitionState) attributeAndChainApplies() {
	queues := s.applyAttributions()

	for _, apply := range s.applies {
		attr, ok := s.attributionFor(apply, queues)
		if !ok {
			s.observe(cellID{0, apply.Subject}, ObsUnattributed,
				applyEvidence(s.partition, apply), "no fix-generation attribution")

			continue
		}

		s.chainFixEffective(cellID{attr.PromptIndex, apply.Subject}, apply, attr.EventIndex)
	}
}

// applyAttributions returns the log-derived attribution queues keyed by
// sanitized subject, in apply order. Sanitizer collisions (two distinct
// raw subjects collapsing to one artifact subject) poison the affected
// key: its applies become unattributed rather than guessed.
func (s *partitionState) applyAttributions() map[string][]applyAttribution {
	result := map[string][]applyAttribution{}

	if s.segment == nil {
		return result
	}

	rawBySanitized := map[string]map[string]bool{}

	for _, event := range s.segment.Events {
		if event.Subject == "" {
			continue
		}

		key := sanitizeID(event.Subject)
		if rawBySanitized[key] == nil {
			rawBySanitized[key] = map[string]bool{}
		}

		rawBySanitized[key][event.Subject] = true
	}

	for _, attr := range attributeApplies(*s.segment) {
		key := sanitizeID(attr.Subject)

		if len(rawBySanitized[key]) > 1 {
			s.diagnose("sanitizer collision on subject " + key +
				" — attribution poisoned, applies left unattributed")

			continue
		}

		result[key] = append(result[key], attr)
	}

	return result
}

// attributionFor resolves the attribution for one apply artifact. With
// a log segment present, ONLY consumed log attribution counts (a
// segment without a matching generation never falls back — Review-1
// policy). The single-failed-prompt fallback applies only to logless
// runs, and its result carries EventIndex -1 (no temporal proof, so it
// can never satisfy the fix chain; it merely labels candidates).
func (s *partitionState) attributionFor(
	apply ApplyArtifact, queues map[string][]applyAttribution,
) (applyAttribution, bool) {
	if s.segment != nil {
		queue := queues[apply.Subject]
		if len(queue) == 0 {
			return applyAttribution{}, false
		}

		attr := queue[0]
		queues[apply.Subject] = queue[1:]

		if attr.PromptIndex <= 0 {
			return applyAttribution{}, false
		}

		return attr, true
	}

	idx, ok := s.singleFailedPrompt(apply.Subject)
	if !ok {
		return applyAttribution{}, false
	}

	return applyAttribution{Subject: apply.Subject, PromptIndex: idx, EventIndex: -1}, true
}

// singleFailedPrompt is the conservative no-log fallback: attribution
// succeeds only when exactly one prompt failed for the subject.
func (s *partitionState) singleFailedPrompt(subject string) (int, bool) {
	found := 0

	for cell := range s.failedCells {
		if cell.Subject == subject {
			if found != 0 {
				return 0, false
			}

			found = cell.Index
		}
	}

	return found, found != 0
}

// chainFixEffective grants fix-effective credit only for the complete
// log-proven chain on an authored-F shipped prompt; anything less is
// reported as a candidate.
func (s *partitionState) chainFixEffective(cell cellID, apply ApplyArtifact, applyEventIdx int) {
	evidence := applyEvidence(s.partition, apply)

	shipped := s.ctx.shipped[cell.Index]
	if shipped == nil || !shipped.HasF {
		return // fix loop on an F-less or non-shipped prompt: no F branch exists
	}

	reason, complete := s.fixChainComplete(cell, applyEventIdx)
	if !complete {
		s.observe(cell, ObsFixCandidate, evidence, reason)

		return
	}

	s.observe(cell, ObsFixEffective, evidence, "reconstructed chain")
}

// fixChainComplete verifies the full reconstructed chain: semantic
// fail, final canonical pass, a log-proven clean post-apply walk, and
// a post-apply result save for THIS cell (cell-temporal, not just the
// run-global save count).
func (s *partitionState) fixChainComplete(cell cellID, applyEventIdx int) (string, bool) {
	if !s.failedCells[cell] {
		return "no semantic fail recorded for the attributed cell", false
	}

	if s.finalClass[cell] != ClassPassCanonical {
		return "cell did not end with a canonical pass", false
	}

	if s.segment == nil || applyEventIdx < 0 {
		return "no log proof of the apply chain", false
	}

	if s.ctx.logTainted {
		return "log contains malformed lines — chain unprovable", false
	}

	if !cleanWalkProven(*s.segment) {
		return "no log proof of a clean post-apply walk", false
	}

	if !s.cellSavedAfter(cell, applyEventIdx) {
		return "no post-apply result save for the attributed cell", false
	}

	return "", true
}

// cellSavedAfter reports whether the segment records a result save for
// this cell's artifact after the given event index.
func (s *partitionState) cellSavedAfter(cell cellID, eventIdx int) bool {
	prompt := s.promptAt(cell.Index)
	if prompt == nil {
		return false
	}

	wanted := "/" + resultFileName(cell, prompt.SanitizedPath)

	for _, event := range s.segment.Events[eventIdx+1:] {
		if event.Kind == evResultSaved && strings.HasSuffix(event.File, wanted) {
			return true
		}
	}

	return false
}

// promptAt returns the run prompt with the given flattened index.
func (s *partitionState) promptAt(idx int) *UniversePrompt {
	for i := range s.ctx.prompts {
		if s.ctx.prompts[i].Global == idx {
			return &s.ctx.prompts[i]
		}
	}

	return nil
}

// applyEvidence renders the primary evidence path for an apply.
func applyEvidence(partition string, apply ApplyArtifact) string {
	return fmt.Sprintf("%s/apply-%s-iter%d-result.yaml", partition, apply.Subject, apply.Iteration)
}
