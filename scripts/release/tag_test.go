package release_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/release"
)

func TestTagVersionAcceptsOnlyTheEngineModuleTag(t *testing.T) {
	t.Parallel()

	for tag, want := range map[string]string{
		"services/true-bdd/v0.1.0":       "v0.1.0",
		"services/true-bdd/v1.12.3":      "v1.12.3",
		"services/true-bdd/v2.0.0-rc.1":  "v2.0.0-rc.1",
		"services/true-bdd/v0.0.0-alpha": "v0.0.0-alpha",
	} {
		got, err := release.TagVersion(tag)
		if err != nil || got != want {
			t.Errorf("TagVersion(%q) = %q, %v; want %q", tag, got, err, want)
		}
	}

	for _, tag := range []string{
		"v0.1.0", "pkg/v0.1.0", "services/true-bdd/0.1.0", "services/true-bdd/v0.1",
		"services/true-bdd/v01.0.0", "refs/tags/services/true-bdd/v0.1.0", "services/bdd-cli/v0.1.0",
		"services/true-bdd/v0.1.0/extra", "",
	} {
		_, err := release.TagVersion(tag)
		if err == nil {
			t.Errorf("TagVersion(%q) accepted", tag)
		}
	}
}
