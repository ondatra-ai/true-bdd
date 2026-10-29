package validate

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func conceptInterfaceContext(services []architecture.Service, name string) (map[string]any, error) {
	context := make(map[string]any)

	for _, service := range services {
		if service.Name != name {
			continue
		}

		context["declaration"] = service.PublicInterface
		if service.PublicInterface == nil || service.PublicInterface.Document == "" {
			continue
		}

		path := service.PublicInterface.Document

		content, err := disk.Read(path)
		if err != nil {
			return nil, fmt.Errorf("embed Public interface %s: %w", path, err)
		}

		context[path] = string(content)
	}

	return context, nil
}
