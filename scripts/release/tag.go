package release

import (
	"fmt"
	"regexp"
)

// TagPrefix is the engine module's directory: Go resolves `go install …/services/true-bdd@vX.Y.Z`
// only from a tag carrying it, and a release is that one tag (Owner, #222, 27 October).
const TagPrefix = "services/true-bdd/"

var releaseTag = regexp.MustCompile(`^` + regexp.QuoteMeta(TagPrefix) +
	`(v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?)$`)

// TagVersion returns the semantic version a release tag names, refusing any other tag.
func TagVersion(tag string) (string, error) {
	match := releaseTag.FindStringSubmatch(tag)
	if match == nil {
		return "", fmt.Errorf("%w: a release tag is %svX.Y.Z, not %q", errRefused, TagPrefix, tag)
	}

	return match[1], nil
}
