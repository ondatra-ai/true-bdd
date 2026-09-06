package steps

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/playwright-community/playwright-go"
)

const (
	// textProbe answers with one element's rendered text, whitespace collapsed:
	// a trail wraps its crumbs across source lines and a reader sees one gap.
	textProbe = `els => els.map(el => (el.textContent || "").trim().replace(/\s+/g, " "))`
	// linkProbe answers with that text and the link's own target, joined by a
	// NUL — a byte no rendered text carries, so the split back cannot be fooled.
	linkProbe = `els => els.map(el => (el.textContent || "").trim().replace(/\s+/g, " ") + ` +
		`"\u0000" + (el.getAttribute("href") || ""))`
	// linkFieldSeparator is that byte, on the Go side of the same contract.
	linkFieldSeparator = "\x00"
)

// evaluateAllStrings runs a probe written to answer one string per matched
// element, and says so when it answers anything else.
func evaluateAllStrings(locator playwright.Locator, script string) ([]string, error) {
	raw, err := locator.EvaluateAll(script)
	if err != nil {
		return nil, fmt.Errorf("read the matching elements: %w", err)
	}

	values, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %v", ErrUnreadableProbe, raw)
	}

	texts := make([]string, 0, len(values))

	for _, value := range values {
		text, isText := value.(string)
		if !isText {
			return nil, fmt.Errorf("%w: %v", ErrUnreadableProbe, value)
		}

		texts = append(texts, text)
	}

	return texts, nil
}

// readQuotedTexts renders every matched element's text the way a step writes a
// list of them, so the poll and the failure speak one vocabulary.
func readQuotedTexts(locator playwright.Locator) func() (string, error) {
	return func() (string, error) {
		values, err := evaluateAllStrings(locator, textProbe)
		if err != nil {
			return "", err
		}

		quoted := make([]string, 0, len(values))

		for _, value := range values {
			quoted = append(quoted, strconv.Quote(value))
		}

		return strings.Join(quoted, ", "), nil
	}
}

// readTexts renders those texts one per line, which is what a clause about ONE
// of many elements is matched against.
func readTexts(locator playwright.Locator) func() (string, error) {
	return func() (string, error) {
		values, err := evaluateAllStrings(locator, textProbe)
		if err != nil {
			return "", err
		}

		return strings.Join(values, "\n"), nil
	}
}

// readLinks renders every matched link as `"text" -> "href"`, one per line.
func readLinks(locator playwright.Locator) func() (string, error) {
	return func() (string, error) {
		values, err := evaluateAllStrings(locator, linkProbe)
		if err != nil {
			return "", err
		}

		lines := make([]string, 0, len(values))

		for _, value := range values {
			text, href, _ := strings.Cut(value, linkFieldSeparator)
			lines = append(lines, linkReading(text, href))
		}

		return strings.Join(lines, "\n"), nil
	}
}
