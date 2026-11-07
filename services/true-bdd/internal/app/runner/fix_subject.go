package runner

import (
	"context"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
)

func prepareFixSubject[I any](
	ctx context.Context, spec Spec[I], item I, failed checklistmodels.ValidationResult,
) (I, error) {
	if spec.FixSubject != nil {
		return spec.FixSubject(ctx, item, failed)
	}

	return item, nil
}

func prepareApplySubject[I any](
	ctx context.Context, spec Spec[I], item I, failed checklistmodels.ValidationResult,
) (I, error) {
	subject, err := prepareFixSubject(ctx, spec, item, failed)
	if err != nil {
		return item, err
	}

	if spec.BeforeApply != nil {
		err = spec.BeforeApply(ctx, item, failed)
	}

	return subject, err
}

func applySpecializedFix[I any](ctx context.Context, spec Spec[I], item I,
	failed checklistmodels.ValidationResult, decision engine.FixDecision,
) (bool, error) {
	if spec.ApplyFix == nil {
		return false, nil
	}

	return spec.ApplyFix(ctx, item, failed, decision)
}
