package validate

import (
	"context"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
)

// judgeSharedBindings gives the Scenario each linked binding's verdict, judged once per walk with every step of
// the walk's passing links that reached its concept for the service; the walker starts it after every link.
func (e *ChecklistEvaluator) judgeSharedBindings(ctx context.Context, prompt checklist.PromptWithContext,
	service string, linked map[string]linkedBinding, services []architecture.Service,
	result *checklist.ValidationResult,
) error {
	interfaceContext, err := conceptInterfaceContext(services, service)
	if err != nil {
		return err
	}

	productText, err := e.conceptProduct()
	if err != nil {
		return err
	}

	names := make([]string, 0, len(linked))
	keys := make(map[string]string, len(linked))

	for _, key := range sortedBindingKeys(linked) {
		name := sharedKey(prompt, sharedBindings, key+"@"+service)
		names = append(names, name)
		keys[name] = key
	}

	answers, err := e.sharing.resolve(ctx, names, func(name string) (string, error) {
		one := map[string]linkedBinding{keys[name]: linked[keys[name]]}

		schema, err := bindingSchema(one)
		if err != nil {
			return "", err
		}

		payload := struct {
			Product         string                   `json:"product"`
			Steps           []conceptStep            `json:"steps"`
			Service         string                   `json:"service"`
			Bindings        map[string]linkedBinding `json:"bindings"`
			PublicInterface map[string]any           `json:"public_interface"`
		}{productText, e.sharing.stepsLinking(keys[name], service), service, one, interfaceContext}

		attrs := []any{"judgement", sharedBindings, sharedConceptAttr, keys[name], "service", service}

		return e.sharedTurn(ctx, prompt, name, attrs, payload, schema)
	})
	if err != nil {
		return err
	}

	for _, name := range names {
		gradeBindings(answers[name], map[string]linkedBinding{keys[name]: linked[keys[name]]}, result)
	}

	return nil
}
