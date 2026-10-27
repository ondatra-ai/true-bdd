package provider

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Tier is the engine-level name a checklist uses to pick a model
// without hardcoding one: the checklist asks for "coder",
// true-bdd.yaml decides which cli and model that means.
type Tier string

const (
	// TierXHigh is the strongest reasoning tier — validation questions
	// whose verdict must be trusted.
	TierXHigh Tier = "xhigh"
	// TierHigh is the everyday tier — turning a failure into a fix prompt.
	TierHigh Tier = "high"
	// TierCoder is the cheap, high-context writing tier — applying a fix
	// to a file.
	TierCoder Tier = "coder"
	// TierQuick is the cheap, fast verdict tier — a graded answer with a
	// short reason, where latency and cost matter more than depth. The
	// BDD harness's judge resolves its model from this tier.
	TierQuick Tier = "quick"
)

// ErrUnknownTier marks a tier name the engine does not define.
var ErrUnknownTier = errors.New("unknown model tier")

// Tiers lists every tier the engine recognises.
func Tiers() []Tier {
	return []Tier{TierXHigh, TierHigh, TierCoder, TierQuick}
}

// ParseTier validates a tier name read from YAML.
func ParseTier(raw string) (Tier, error) {
	tier := Tier(strings.TrimSpace(raw))
	if !slices.Contains(Tiers(), tier) {
		return "", fmt.Errorf("%w: %q (known: %s)", ErrUnknownTier, raw, joinTiers(Tiers()))
	}

	return tier, nil
}

// joinTiers renders a tier list for error messages.
func joinTiers(tiers []Tier) string {
	names := make([]string, 0, len(tiers))
	for _, tier := range tiers {
		names = append(names, string(tier))
	}

	return strings.Join(names, ", ")
}
