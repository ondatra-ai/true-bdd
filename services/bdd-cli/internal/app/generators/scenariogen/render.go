package scenariogen

import (
	"bytes"
	"fmt"
	"go/format"
	"strconv"
	"strings"
)

// Render produces the gofmt-canonical bytes of one generated file.
//
// Always formatted, never the template's raw output. gofmt canonicalises
// the spacing around a build constraint and around declarations, so a
// comparison against unformatted bytes would report drift on a file that
// is already correct — and a drift report nobody believes is worse than
// none.
func Render(file PlannedFile) ([]byte, error) {
	var buf bytes.Buffer

	err := fileTemplate().Execute(&buf, file)
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", file.Path, err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format %s: %w", file.Path, err)
	}

	return formatted, nil
}

// literal renders a step's text as a Go string literal.
//
// A raw string by preference, because step text is full of "quoted"
// runs and regex escapes like \( — in an interpreted literal every one
// of those would need a backslash, and the file would stop reading like
// the scenario it transcribes. A backtick or a line break in the text
// makes a raw string impossible, so those fall back to the quoted form.
func literal(text string) string {
	if !strings.ContainsAny(text, "`\r\n") {
		return "`" + text + "`"
	}

	return strconv.Quote(text)
}
