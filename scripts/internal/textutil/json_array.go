package textutil

import (
	"errors"
	"regexp"
	"strings"
)

// ErrNoJSONArray reports that a model's answer held no `[...]` at all.
var ErrNoJSONArray = errors.New("no JSON array in the answer")

// Python's `\s` matches the vertical tab; Go's does not. Spelling the class
// out keeps the fence stripping identical to the regex this ports.
const whitespaceClass = `[\s\v]`

var fenceRE = regexp.MustCompile(
	`(?m)^` + whitespaceClass + "*```(?:json)?" + whitespaceClass +
		`*|` + whitespaceClass + "*```" + whitespaceClass + `*$`)

// ExtractJSONArray returns the `[...]` span of a model's answer, ready to
// unmarshal, or ErrNoJSONArray if there is none.
//
// Models were asked for "ONLY a JSON array, no prose and no code fence" and
// mostly comply, so this strips a fence if one came anyway and then takes
// everything from the first `[` to the last `]`. Deliberately not a parse of
// the whole answer: a leading sentence is common and harmless, and failing on
// it would turn a usable answer into a stop.
func ExtractJSONArray(answer string) ([]byte, error) {
	text := fenceRE.ReplaceAllString(strings.TrimSpace(answer), "")

	start, end := strings.Index(text, "["), strings.LastIndex(text, "]")
	if start == -1 || end == -1 || end < start {
		return nil, ErrNoJSONArray
	}

	return []byte(text[start : end+1]), nil
}
