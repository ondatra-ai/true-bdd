package runner

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// ErrJudgeTierMissing marks an engine config whose `quick:` tier is
// absent or not bound to claude. The judge refuses to boot rather than
// falling back to a model nobody chose.
var ErrJudgeTierMissing = errors.New("engine.models.quick is not bound to a claude model")

// judgeModelPattern captures the model id from the `quick:` tier. The
// judge is claude-only, so the cli half is matched literally.
var judgeModelPattern = regexp.MustCompile(`(?m)^\s+quick:\s*"?claude:([^"\n]+?)"?\s*$`)

// JudgeModel reads the judge's pinned model from the REPO-ROOT engine
// config, never a fixture's overlay — a fixture retargets the engine
// under test, and must not reach the judge that grades it.
func JudgeModel(configPath string) (string, error) {
	raw, err := disk.Read(configPath)
	if err != nil {
		return "", fmt.Errorf("read engine config %s: %w", configPath, err)
	}

	match := judgeModelPattern.FindStringSubmatch(string(raw))
	if match == nil {
		return "", fmt.Errorf("%w: %s", ErrJudgeTierMissing, configPath)
	}

	return match[1], nil
}
