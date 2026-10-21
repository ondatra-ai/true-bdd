package inventory

// rawRegistry mirrors only the scenario-registry shape the scanner
// needs: the scenarios map, each entry decoded as the engine's loader
// decodes its lineage list, so a malformed entry reads as invalid.
type rawRegistry struct {
	Scenarios map[string]struct {
		UserStories []struct {
			Story      string `yaml:"story"`
			ScenarioID string `yaml:"scenario_id"`
		} `yaml:"user_stories"`
	} `yaml:"scenarios"`
}
