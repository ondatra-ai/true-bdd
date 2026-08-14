package docs

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
)

// ErrUnknownDocKey is returned when a checklist prompt declares a
// `docs:` key the engine has no mapping for. Almost always a typo in
// the checklist.
var ErrUnknownDocKey = errors.New("unknown document key")

// ErrDocPathNotConfigured is returned when a known key maps to a
// `documents.*` config entry the host project left blank.
var ErrDocPathNotConfigured = errors.New("document path not configured")

// ErrDocFileMissing is returned when a document resolves to a path
// that does not exist on disk. This is the one that used to pass
// silently: the prompt still carried a `Read(<path>)` instruction, the
// model found nothing there, and answered from whatever else it had.
var ErrDocFileMissing = errors.New("document file does not exist")

// ErrDocNotRegularFile is returned when a document path exists but is a
// directory, a device or a socket. Same class of silent failure as
// ErrDocFileMissing — os.Stat succeeds, preflight passes, and the model
// is then asked to read something it cannot read.
var ErrDocNotRegularFile = errors.New("document path is not a regular file")

// Document keys a checklist may name under a prompt's `docs:` list.
// KeyScenariosYAML doubles as the cmd layer's route to the scenario
// registry path (`documents.scenarios_yaml`) — the engine has no
// hardcoded default for it.
const (
	KeyProduct          = "product"
	KeyArchitectureYAML = "architecture_yaml"
	KeyScenariosYAML    = "scenarios_yaml"
)

// keyToConfigPath maps a checklist `docs:` key to the `documents.*`
// config entry holding its path. Single source of truth — the
// evaluator, the fix-prompt generator, and the runner's up-front
// validation all resolve through here, so they can never disagree
// about what a key means.
func keyToConfigPath() map[string]string {
	return map[string]string{
		KeyProduct:          "documents." + KeyProduct,
		KeyArchitectureYAML: "documents." + KeyArchitectureYAML,
		KeyScenariosYAML:    "documents." + KeyScenariosYAML,
	}
}

// Resolver turns checklist document keys into paths on disk.
type Resolver struct {
	config *config.ViperConfig
}

// NewResolver builds a Resolver over the host project's config.
func NewResolver(cfg *config.ViperConfig) *Resolver {
	return &Resolver{config: cfg}
}

// Resolve returns the on-disk path for one document key, or an error
// explaining precisely why the key cannot be satisfied. A nil error
// guarantees the returned path exists.
func (r *Resolver) Resolve(key string) (string, error) {
	configPath, ok := keyToConfigPath()[key]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownDocKey, key)
	}

	filePath := r.config.GetString(configPath)
	if filePath == "" {
		return "", fmt.Errorf("%w: %q (config key %s)",
			ErrDocPathNotConfigured, key, configPath)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("%w: %q -> %s", ErrDocFileMissing, key, filePath)
	}

	// A directory satisfies os.Stat but cannot be handed to a prompt as a
	// document. Without this the run passes preflight and then dispatches
	// AI turns carrying a Read instruction for a directory — which is
	// exactly the silent, mid-run failure the up-front validation exists
	// to prevent.
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: %q -> %s", ErrDocNotRegularFile, key, filePath)
	}

	return filePath, nil
}

// ResolveAll resolves every key and reports EVERY failure at once
// rather than stopping at the first. A run missing two documents
// should tell you about both, not make you fix them one round-trip at
// a time. Keys are visited in sorted order so the message is stable.
//
// The returned map is complete only when err is nil.
func (r *Resolver) ResolveAll(keys []string) (map[string]string, error) {
	unique := make([]string, 0, len(keys))
	seen := make(map[string]bool, len(keys))

	for _, key := range keys {
		if !seen[key] {
			seen[key] = true

			unique = append(unique, key)
		}
	}

	sort.Strings(unique)

	paths := make(map[string]string, len(unique))
	problems := make([]error, 0)

	for _, key := range unique {
		path, err := r.Resolve(key)
		if err != nil {
			problems = append(problems, err)

			continue
		}

		paths[key] = path
	}

	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}

	return paths, nil
}
