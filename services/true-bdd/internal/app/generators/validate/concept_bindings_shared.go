package validate

import (
	"context"
	"errors"
	"fmt"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
)

var errBindingsOutsideWalk = errors.New("a binding judgement needs a started walk (ADR-0037)")

// bindingRequest is one binding's turn: the product, every step reaching its concept for the service, the binding
// and the service's Public interface.
type bindingRequest struct {
	ProductPath     string                   `json:"product_path"`
	Steps           []conceptStep            `json:"steps"`
	Service         string                   `json:"service"`
	Bindings        map[string]linkedBinding `json:"bindings"`
	PublicInterface map[string]any           `json:"public_interface"`
}

// judgeSharedBindings gives the Scenario or suite each linked binding's verdict, judged once per walk with every
// step of the walk that reached its concept for the service; scen check's walker starts it after every link.
func (e *ChecklistEvaluator) judgeSharedBindings(ctx context.Context, prompt checklist.PromptWithContext,
	service string, linked map[string]linkedBinding, services []architecture.Service,
	result *checklist.ValidationResult,
) error {
	if !e.sharing.active() {
		return errBindingsOutsideWalk
	}

	interfaceContext, err := conceptInterfaceContext(services, service)
	if err != nil {
		return err
	}

	productPath, err := e.docResolver.Resolve(docs.KeyProduct)
	if err != nil {
		return fmt.Errorf("resolve binding product: %w", err)
	}

	keys := sortedBindingKeys(linked)

	answers, err := e.judgeShared(ctx, prompt, sharedJudgement{judgement: checklist.JudgementBindings, keys: keys,
		scope: "@" + service, attrs: []any{"service", service}, request: func(key string) (any, string, error) {
			one := map[string]linkedBinding{key: linked[key]}
			schema, err := bindingSchema(one)

			return bindingRequest{productPath, e.sharing.stepsLinking(key, service), service, one, interfaceContext},
				schema, err
		}})
	if err != nil {
		return err
	}

	for _, key := range keys {
		gradeBindings(answers[key], map[string]linkedBinding{key: linked[key]}, result)
	}

	return nil
}
