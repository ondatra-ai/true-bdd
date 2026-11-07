package runner

import (
	"context"
	"errors"
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

func cannotFix(err error) bool {
	return errors.Is(err, pkgerrors.ErrCannotFix) || errors.Is(err, pkgerrors.ErrImpossibleWork) ||
		errors.Is(err, pkgerrors.ErrInterfaceCannotServe)
}

// locateCannotFix preserves the established instances while naming their checklist item.
func locateCannotFix(failed *checklist.ValidationResult, err error) error {
	if !cannotFix(err) {
		return err
	}

	return fmt.Errorf("%w %s #%d: %w", pkgerrors.ErrCannotFix, failed.SectionPath, failed.PromptIndex, err)
}

func stopCannotFix[I any](ctx context.Context, spec Spec[I], item I,
	failed *checklist.ValidationResult, err error,
) error {
	if !errors.Is(err, pkgerrors.ErrInterfaceCannotServe) && spec.StopImpossible != nil {
		err = spec.StopImpossible(ctx, item, err)
	}

	return locateCannotFix(failed, err)
}
