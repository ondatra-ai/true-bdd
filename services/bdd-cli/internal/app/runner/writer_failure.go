package runner

import (
	"errors"
	"log/slog"

	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

func recoverWriterFailure(enabled bool, err error) bool {
	if !enabled {
		return false
	}

	var claudeFailure *pkgerrors.ClaudeTurnError
	if errors.As(err, &claudeFailure) {
		slog.Warn("Writer turn failed", "outcome", claudeFailure.Outcome, "error", err)

		return true
	}

	var failure *pkgerrors.CrushTurnError
	if errors.As(err, &failure) {
		if failure.Outcome != pkgerrors.CrushTurnStalled {
			slog.Warn("Writer turn failed", "outcome", failure.Outcome, "error", err)
		}

		return true
	}

	if errors.Is(err, pkgerrors.ErrFixApplierNoContent) {
		slog.Warn("Writer confirmation missing", "error", err)

		return true
	}

	var codexFailure *pkgerrors.CodexTurnError
	if errors.As(err, &codexFailure) {
		slog.Warn("Writer turn failed", "outcome", codexFailure.Outcome, "error", err)

		return true
	}

	return false
}
