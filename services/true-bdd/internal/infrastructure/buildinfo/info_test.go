package buildinfo_test

import (
	"runtime/debug"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/buildinfo"
)

const (
	revisionKey   = "vcs.revision"
	releaseCommit = "release-revision"
	moduleVersion = "v2.3.4"
)

func TestStampedBuild(t *testing.T) {
	info := &debug.BuildInfo{
		Main:     debug.Module{Version: "v0.9.0"},
		Settings: []debug.BuildSetting{{Key: revisionKey, Value: "old-revision"}},
	}
	got := buildinfo.Resolve("v1.2.3", releaseCommit, info)

	want := buildinfo.Info{Version: "v1.2.3", Commit: releaseCommit}
	if got != want {
		t.Fatalf("build = %+v, want %+v", got, want)
	}
}

func TestModuleVersionWithoutStamp(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: moduleVersion},
		Deps: []*debug.Module{{Version: "v9.9.9"}},
	}
	got := buildinfo.Resolve("", "", info)

	want := buildinfo.Info{Version: moduleVersion, Commit: "unknown"}
	if got != want {
		t.Fatalf("build = %+v, want %+v", got, want)
	}
}

func TestVCSBuildWithoutStamp(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v0.0.0-20261027072958-954797a04424"},
		Settings: []debug.BuildSetting{
			{Key: "vcs", Value: "git"},
			{Key: revisionKey, Value: "954797a04424d1e347862c30d71cc0c989ac0282"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	got := buildinfo.Resolve("", "", info)

	want := buildinfo.Info{
		Version: "v0.0.0-20261027072958-954797a04424",
		Commit:  "954797a04424d1e347862c30d71cc0c989ac0282",
	}
	if got != want {
		t.Fatalf("build = %+v, want %+v", got, want)
	}
}

func TestArchiveBuildWithoutMetadata(t *testing.T) {
	for _, info := range []*debug.BuildInfo{nil, {}, {Main: debug.Module{Version: "(devel)"}}} {
		got := buildinfo.Resolve("", "", info)

		want := buildinfo.Info{Version: "(devel)", Commit: "unknown"}
		if got != want {
			t.Fatalf("build = %+v, want %+v", got, want)
		}
	}
}

func TestBuildFieldsResolveIndependently(t *testing.T) {
	info := &debug.BuildInfo{
		Main:     debug.Module{Version: moduleVersion},
		Settings: []debug.BuildSetting{{Key: revisionKey, Value: "vcs-revision"}},
	}
	for _, check := range []struct {
		version string
		commit  string
		want    buildinfo.Info
	}{
		{"v1.0.0", "", buildinfo.Info{Version: "v1.0.0", Commit: "vcs-revision"}},
		{"", releaseCommit, buildinfo.Info{Version: moduleVersion, Commit: releaseCommit}},
	} {
		if got := buildinfo.Resolve(check.version, check.commit, info); got != check.want {
			t.Fatalf("build = %+v, want %+v", got, check.want)
		}
	}
}
