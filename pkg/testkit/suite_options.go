package testkit

import proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"

// SuiteOptions carries the fixture choices declared by the Host project's architecture.
type SuiteOptions struct {
	// RecordExclude names host-relative paths neither run nor dependency recording snapshots read.
	RecordExclude []string
	// RepoLayer names Host project paths copied before fixture input is overlaid.
	RepoLayer []string
	// FixtureStep is the Given pattern whose first capture names a fixture tree.
	FixtureStep string
}

func (o SuiteOptions) recordingDependencies(dependencies []proxycli.Dependency) []proxycli.Dependency {
	combined := make([]proxycli.Dependency, 0, len(dependencies))
	for _, dependency := range dependencies {
		dependency.Recording.Unread = append(append([]string(nil), dependency.Recording.Unread...), o.RecordExclude...)
		combined = append(combined, dependency)
	}

	return combined
}
