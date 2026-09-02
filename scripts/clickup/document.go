package clickup

import (
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// deferralOrigin is what a hand-written deferral shows under `### Why`. Not
// "a local review", which is what an absent pull request used to render as.
const deferralOrigin = "a hand-written deferral"

// FileDocument files a deferral a person wrote by hand. The document is raw
// material — one `## ` heading per ticket, prose under it — and reaches the
// same gate, scoring turn, render and filing turn a review finding does.
func FileDocument(path, tag string) error {
	raw, err := disk.Read(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	queue := findingsOf(splitSections(string(raw)))
	if len(queue) == 0 {
		return fmt.Errorf("%w: %s carries no `## ` heading, so there is no ticket in it",
			ErrNotFiled, path)
	}

	slog.Info("Document split into tickets", "count", len(queue), "document", path)

	// Strict: `clickup defer` is a person at a keyboard, who can be told to
	// try again, so a gate that cannot answer files nothing (ADR 0007).
	return fileQueue(queue, tag, deferralOrigin, true)
}
