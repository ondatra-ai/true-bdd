package steps

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

// Registry-origin assertions read the registry a `us apply … --fix` run
// writes: a scenario "comes from" criterion X when a `user_stories` entry
// has `scenario_id: X`; it "cites" story S when one has `story: S`.

// registryFile is the slice of a scenario registry these assertions need:
// the `scenarios:` mapping, each entry kept as a raw node so it can be
// both decoded for its lineage and re-marshalled for a content match.
type registryFile struct {
	Scenarios map[string]yaml.Node `yaml:"scenarios"`
}

// registryScenario is the lineage of one registry entry: which stories
// and acceptance criteria (`scenario_id`) its user_stories cite. Other
// fields (description, service, merged_steps) are left undecoded.
type registryScenario struct {
	UserStories []struct {
		Story      string `yaml:"story"`
		ScenarioID string `yaml:"scenario_id"`
	} `yaml:"user_stories"`
}

// groupByStepText groups entry ids by the canonical serialization of their
// merged_steps; any key with more than one id is a duplicate set. An entry
// with no merged_steps is skipped, not collided into an empty-key group.
func groupByStepText(entries map[string]yaml.Node, ids []string) (map[string][]string, error) {
	bySteps := make(map[string][]string)

	for _, entryID := range ids {
		node := entries[entryID]

		var entry struct {
			MergedSteps yaml.Node `yaml:"merged_steps"`
		}

		err := node.Decode(&entry)
		if err != nil {
			return nil, fmt.Errorf("decoding scenario %q: %w", entryID, err)
		}

		if entry.MergedSteps.Kind == 0 {
			continue
		}

		serialized, err := yaml.Marshal(&entry.MergedSteps)
		if err != nil {
			return nil, fmt.Errorf("re-marshalling steps of scenario %q: %w", entryID, err)
		}

		key := string(serialized)
		bySteps[key] = append(bySteps[key], entryID)
	}

	return bySteps, nil
}

// loadRegistryScenarios reads and parses the registry at registryRel
// (relative to the run's tmpdir) into its entries keyed by id. A registry
// that fails to read, parse, or declares no scenarios fails loudly.
func loadRegistryScenarios(state *State, registryRel string) (map[string]yaml.Node, error) {
	full, containErr := state.containedPath(registryRel)
	if containErr != nil {
		return nil, containErr
	}

	data, err := disk.Read(full)
	if err != nil {
		return nil, state.fail("reading registry %q: %w", registryRel, err)
	}

	var file registryFile

	unmarshalErr := yaml.Unmarshal(data, &file)
	if unmarshalErr != nil {
		return nil, state.fail("parsing registry %q: %w", registryRel, unmarshalErr)
	}

	if len(file.Scenarios) == 0 {
		return nil, state.fail("registry %q declares no scenarios", registryRel)
	}

	return file.Scenarios, nil
}

// scenarioIDsFromCriterion returns, sorted, the ids of every registry
// entry whose user_stories cite the given criterion — sorted so a count
// failure names offenders in a stable order regardless of map iteration.
func scenarioIDsFromCriterion(entries map[string]yaml.Node, criterion string) ([]string, error) {
	var ids []string

	for entryID, node := range entries {
		var entry registryScenario

		err := node.Decode(&entry)
		if err != nil {
			return nil, fmt.Errorf("decoding scenario %q: %w", entryID, err)
		}

		for _, link := range entry.UserStories {
			if link.ScenarioID == criterion {
				ids = append(ids, entryID)

				break
			}
		}
	}

	sort.Strings(ids)

	return ids, nil
}

// uniqueScenarioFromCriterion resolves the single registry entry
// descending from an acceptance criterion. Zero or more-than-one are each
// their own failure: a lineage collapsed to nothing, or never collapsed.
func uniqueScenarioFromCriterion(
	state *State, entries map[string]yaml.Node, registryRel, criterion string,
) (string, yaml.Node, error) {
	ids, err := scenarioIDsFromCriterion(entries, criterion)
	if err != nil {
		return "", yaml.Node{}, state.fail(
			"reading acceptance criterion %q in %q: %w", criterion, registryRel, err)
	}

	switch len(ids) {
	case 0:
		return "", yaml.Node{}, state.fail(
			"expected exactly one scenario in %q from acceptance criterion %q, but none do",
			registryRel, criterion)
	case 1:
		return ids[0], entries[ids[0]], nil
	default:
		return "", yaml.Node{}, state.fail(
			"expected exactly one scenario in %q from acceptance criterion %q, but %d do: %v",
			registryRel, criterion, len(ids), ids)
	}
}

// assertRegistryScenarioCount pins how many registry entries descend
// from one acceptance criterion — how the duplicate-collapse scenario
// proves two entries sharing a lineage became one.
func assertRegistryScenarioCount(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	want, err := strconv.Atoi(args[0])
	if err != nil {
		return state.fail("scenario count %q is not a number: %w", args[0], err)
	}

	registryRel := args[1]
	criterion := args[2]

	entries, err := loadRegistryScenarios(state, registryRel)
	if err != nil {
		return err
	}

	ids, err := scenarioIDsFromCriterion(entries, criterion)
	if err != nil {
		return state.fail("reading acceptance criterion %q in %q: %w", criterion, registryRel, err)
	}

	if len(ids) != want {
		return state.fail(
			"expected exactly %d scenario(s) in %q from acceptance criterion %q, but %d do: %v",
			want, registryRel, criterion, len(ids), ids)
	}

	return nil
}

// assertRegistryScenarioCitesStory pins that the single entry descending
// from a criterion cites a given story — the lineage survived the collapse
// still pointing back at the right source.
func assertRegistryScenarioCitesStory(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	criterion := args[0]
	registryRel := args[1]
	story := args[2]

	entries, err := loadRegistryScenarios(state, registryRel)
	if err != nil {
		return err
	}

	entryID, node, err := uniqueScenarioFromCriterion(state, entries, registryRel, criterion)
	if err != nil {
		return err
	}

	var entry registryScenario

	decodeErr := node.Decode(&entry)
	if decodeErr != nil {
		return state.fail("decoding scenario %q: %w", entryID, decodeErr)
	}

	var cited []string

	for _, link := range entry.UserStories {
		if link.Story == story {
			return nil
		}

		cited = append(cited, link.Story)
	}

	return state.fail(
		"expected scenario %q (from acceptance criterion %q in %q) to cite story %q, but it cites %v",
		entryID, criterion, registryRel, story, cited)
}

// assertRegistryScenarioMatches pins that the single entry descending
// from a criterion, re-serialized to YAML, matches a regexp — proof the
// collapse preserved behaviour, not merely the entry id.
func assertRegistryScenarioMatches(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	criterion := args[0]
	registryRel := args[1]

	pattern, err := regexp.Compile(args[2])
	if err != nil {
		return state.fail("scenario-match pattern %q does not compile: %w", args[2], err)
	}

	entries, err := loadRegistryScenarios(state, registryRel)
	if err != nil {
		return err
	}

	entryID, node, err := uniqueScenarioFromCriterion(state, entries, registryRel, criterion)
	if err != nil {
		return err
	}

	content, err := yaml.Marshal(&node)
	if err != nil {
		return state.fail("re-marshalling scenario %q: %w", entryID, err)
	}

	if !pattern.Match(content) {
		return state.fail(
			"expected scenario %q (from acceptance criterion %q in %q) to match %q, but its content was:\n%s",
			entryID, criterion, registryRel, args[2], content)
	}

	return nil
}

// assertNoDuplicateScenarioSteps pins the duplicate-collapse invariant
// positively: no two registry entries share merged_steps — a check no
// per-criterion count can make, since a collision can span two criteria.
func assertNoDuplicateScenarioSteps(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	registryRel := args[0]

	entries, err := loadRegistryScenarios(state, registryRel)
	if err != nil {
		return err
	}

	// Sorted so both a collision's members and the order groups are
	// reported in stay stable regardless of map iteration.
	ids := make([]string, 0, len(entries))
	for entryID := range entries {
		ids = append(ids, entryID)
	}

	sort.Strings(ids)

	bySteps, err := groupByStepText(entries, ids)
	if err != nil {
		return state.fail("%s: %w", registryRel, err)
	}

	var collisions [][]string

	keys := make([]string, 0, len(bySteps))
	for key := range bySteps {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		if group := bySteps[key]; len(group) > 1 {
			collisions = append(collisions, group)
		}
	}

	if len(collisions) > 0 {
		return state.fail(
			"expected no two scenarios in %q to share the same steps, but %d group(s) do: %v",
			registryRel, len(collisions), collisions)
	}

	return nil
}
