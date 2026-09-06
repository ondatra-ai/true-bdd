package steps

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// backgroundProperty is the painted surface two elements are compared on.
const backgroundProperty = "background-color"

// tokenReadingFunc answers with an element's own computed value of a property
// and the value its token resolves to, joined by the NUL readLinks reserves. The
// token goes through a probe element: "#fff" and ".08em" read back differently.
const tokenReadingFunc = `((el, property, token) => {
	const style = getComputedStyle(el)
	const got = style.getPropertyValue(property).trim()
	const raw = style.getPropertyValue(token).trim()
	if (raw === "") { return got + "\u0000" }
	const probe = document.createElement("span")
	probe.style.fontSize = style.fontSize
	probe.style.fontFamily = style.fontFamily
	probe.style.setProperty(property, raw)
	document.body.appendChild(probe)
	const want = getComputedStyle(probe).getPropertyValue(property).trim()
	probe.remove()
	return got + "\u0000" + want
})`

// registerDesignTokenSteps binds the clauses holding a painted value to the
// design token it must come from, and the one holding two elements apart.
func registerDesignTokenSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) resolves "([^"]*)" from the token "([^"]*)"$`,
		assertResolvesToken)
	suite.Step(`^(`+selectorPattern+`) does not resolve "([^"]*)" from the token "([^"]*)"$`,
		refuteResolvesToken)
	suite.Step(`^(`+selectorPattern+`)'s background differs from (`+selectorPattern+`)'s$`,
		assertBackgroundDiffers)
	registerTokenTrackingSteps(suite)
	registerDesignPaletteSteps(suite)
}

// assertResolvesToken holds one painted property to the value its token
// resolves to, so a page hard-coding the same colour by hand still fails.
func assertResolvesToken(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	return assertTokenOn(state, sel.String(), locator, args[1], args[2])
}

// assertTokenOn is that comparison on an element this suite already resolved,
// which the clauses naming their subject in words cannot reach through the
// selector grammar.
func assertTokenOn(state *State, subject string, locator playwright.Locator,
	property, token string,
) error {
	reading, matched, err := await(readTokenReading(locator, property, token), readingsAgree)
	if err != nil {
		return state.fail("%s: %w", subject, err)
	}

	if !matched {
		got, want := renderReading(reading)

		return state.fail("%s has %s = %s, want the value of %s, which is %s",
			subject, property, got, token, want)
	}

	return nil
}

// refuteResolvesToken is the negative twin, watched rather than read once: a
// value painted a render late would pass a single reading.
func refuteResolvesToken(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	property, token := args[1], args[2]
	read := readTokenReading(locator, property, token)
	deadline := time.Now().Add(attributeSettle)

	for {
		reading, readErr := read()
		if readErr != nil {
			return state.fail("%s: %w", sel, readErr)
		}

		if !readingsDiffer(reading) {
			got, want := renderReading(reading)

			return state.fail("%s has %s = %s, want anything other than %s, which is %s",
				sel, property, got, token, want)
		}

		if !time.Now().Before(deadline) {
			return nil
		}

		time.Sleep(valuePollInterval)
	}
}

// assertBackgroundDiffers holds two elements to painting different surfaces,
// which is what "distinct from its neighbours" means with no colour named.
func assertBackgroundDiffers(state *State, args []string) error {
	first, one, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	second, other, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	reading, matched, err := await(readPairedStyle(one, other, backgroundProperty),
		readingsDiffer)
	if err != nil {
		return state.fail("%s and %s: %w", first, second, err)
	}

	if !matched {
		got, want := renderReading(reading)

		return state.fail("%s paints %s = %s and %s paints %s, want them different",
			first, backgroundProperty, got, second, want)
	}

	return nil
}

// readTokenReading reads that pair as a reader, so a token clause polls through
// the same await every value clause uses.
func readTokenReading(locator playwright.Locator, property, token string,
) func() (string, error) {
	return readProbe(locator, fmt.Sprintf(`el => %s(el, %q, %q)`,
		tokenReadingFunc, property, token))
}

// readPairedStyle reads one computed property off two elements, joined the way a
// token reading is, so both clauses share one comparison.
func readPairedStyle(one, other playwright.Locator, property string,
) func() (string, error) {
	readOne, readOther := readComputedStyle(one, property), readComputedStyle(other, property)

	return func() (string, error) {
		first, err := readOne()
		if err != nil {
			return "", err
		}

		second, err := readOther()
		if err != nil {
			return "", err
		}

		return first + linkFieldSeparator + second, nil
	}
}

// readingsAgree accepts a paired reading whose halves match. A token the page
// does not define agrees with nothing, rather than with an empty string.
func readingsAgree(reading string) bool {
	got, want, _ := strings.Cut(reading, linkFieldSeparator)

	return want != "" && got == want
}

// readingsDiffer is its twin, which an undefined token also fails: a clause that
// a value is NOT a token's says nothing about a token that is not there.
func readingsDiffer(reading string) bool {
	got, want, _ := strings.Cut(reading, linkFieldSeparator)

	return want != "" && got != want
}

// renderReading renders both halves for a failure, naming the absence when the
// page defines no such token.
func renderReading(reading string) (string, string) {
	got, want, _ := strings.Cut(reading, linkFieldSeparator)
	if want == "" {
		return strconv.Quote(got), "a value the page does not define"
	}

	return strconv.Quote(got), strconv.Quote(want)
}
