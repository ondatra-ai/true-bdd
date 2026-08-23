package main

import (
	"regexp"
	"strconv"
	"strings"
)

// Artifact kind suffixes shared by the decoders and the collector.
const (
	kindResultYAML = "result.yaml"
	kindResponse   = "response.txt"
	kindSystem     = "system.txt"
	kindUser       = "user.txt"
	kindPromptsMD  = "prompts.md"
)

// EvaluatorArtifact identifies one evaluator artifact file decoded from
// its filename.
type EvaluatorArtifact struct {
	PromptIndex int    // 1-based flattened prompt index (NN)
	Subject     string // sanitized subject id, may contain dots/dashes
	Kind        string
}

// decodeEvaluatorName decodes "NN-<subject>-checklist-<sanitized-path>-<kind>"
// by right-anchoring on the run's own checklist section paths — never by
// dash-splitting, since subjects may contain dashes.
func decodeEvaluatorName(name string, sanitizedPaths []string) (EvaluatorArtifact, bool) {
	kinds := []string{kindResultYAML, kindResponse, kindSystem, kindUser}

	for _, path := range sanitizedPaths {
		for _, kind := range kinds {
			suffix := "-checklist-" + path + "-" + kind

			if !strings.HasSuffix(name, suffix) {
				continue
			}

			prefix := strings.TrimSuffix(name, suffix)

			idx, subject, ok := splitIndexSubject(prefix)
			if !ok {
				continue
			}

			return EvaluatorArtifact{PromptIndex: idx, Subject: subject, Kind: kind}, true
		}
	}

	return EvaluatorArtifact{}, false
}

// FixArtifact identifies one fix-generation artifact file.
type FixArtifact struct {
	PromptIndex int
	Subject     string
	Iteration   int // 0 for the fix-prompts.md summary file
	Kind        string
}

var fixIterRe = regexp.MustCompile(`^(\d{2})-(.+)-fix-iter(\d+)-(user|system|response)\.txt$`)

// decodeFixName decodes "NN-<subject>-fix-prompts.md" and
// "NN-<subject>-fix-iterN-<kind>.txt".
func decodeFixName(name string) (FixArtifact, bool) {
	if prefix, found := strings.CutSuffix(name, "-fix-"+kindPromptsMD); found {
		idx, subject, ok := splitIndexSubject(prefix)
		if !ok {
			return FixArtifact{}, false
		}

		return FixArtifact{PromptIndex: idx, Subject: subject, Kind: kindPromptsMD}, true
	}

	match := fixIterRe.FindStringSubmatch(name)
	if match == nil {
		return FixArtifact{}, false
	}

	idx, _ := strconv.Atoi(match[1])
	iter, _ := strconv.Atoi(match[3])

	return FixArtifact{PromptIndex: idx, Subject: match[2], Iteration: iter,
		Kind: match[4] + ".txt"}, true
}

// ApplyArtifact identifies one fix-applier artifact file.
type ApplyArtifact struct {
	Subject   string
	Iteration int
	Kind      string
}

// applyRe is greedy on the subject so the LAST "-iterN-" occurrence
// anchors, which keeps trailing-hyphen subjects intact
// (apply-frontend-integration-playwright-startup--iter1-result.yaml).
var applyRe = regexp.MustCompile(
	`^apply-(.+)-iter(\d+)-(result\.yaml|user\.txt|system\.txt|response\.txt)$`)

// decodeApplyName decodes "apply-<subject>-iterN-<kind>".
func decodeApplyName(name string) (ApplyArtifact, bool) {
	match := applyRe.FindStringSubmatch(name)
	if match == nil {
		return ApplyArtifact{}, false
	}

	iter, _ := strconv.Atoi(match[2])

	return ApplyArtifact{Subject: match[1], Iteration: iter, Kind: match[3]}, true
}

var indexPrefixRe = regexp.MustCompile(`^(\d{2})-(.+)$`)

// splitIndexSubject splits the "NN-<subject>" prefix common to
// evaluator and fix artifact names.
func splitIndexSubject(prefix string) (int, string, bool) {
	match := indexPrefixRe.FindStringSubmatch(prefix)
	if match == nil {
		return 0, "", false
	}

	idx, err := strconv.Atoi(match[1])
	if err != nil || idx < 1 {
		return 0, "", false
	}

	return idx, match[2], true
}
