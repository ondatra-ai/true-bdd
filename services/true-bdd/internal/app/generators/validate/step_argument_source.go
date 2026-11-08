package validate

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"

// stepArgumentSource keeps a build command's selected architecture ahead of the configured default.
type stepArgumentSource struct{ path string }

// UseStepArgumentArchitecture supplies the architecture selected by the command, including a CLI override.
func (source *stepArgumentSource) UseStepArgumentArchitecture(path string) { source.path = path }

func (source *stepArgumentSource) evidence(cfg *config.ViperConfig, reference string) (string, error) {
	path := source.path
	if path == "" {
		path = reference
	}

	if source.path != "" && reference != "" {
		reference = source.path
	}

	return stepArgumentsEvidence(cfg, path, reference)
}
