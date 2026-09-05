package steps

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoTokensNamed is returned when a clause about several tokens names none.
var ErrNoTokensNamed = errors.New("the step names no token")

const (
	// typeAspect is the word the type sweep is written with; the colour sweep is
	// the same clause's other wording.
	typeAspect = "type"
	// monoExemptCSS is the file view, which E2E-226 holds to a MONOSPACED face on
	// purpose — so the type sweep leaves that subtree alone.
	monoExemptCSS = `[data-testid^="file-view"]`
	// designFace is the family the design system's font tokens name.
	designFace = "Poppins"
	// offenderLimit caps how many distinct offending values a failure lists, so
	// one stray colour does not print a page of them.
	offenderLimit = 10
)

// paletteSweepFunc answers verdictOK when every visible element paints only
// values the page's OWN :root tokens resolve to, and names the offenders when it
// does not. Judged after resolution: a hand-typed colour matching a token passes.
const paletteSweepFunc = `((properties, exemptCSS, limit) => {
	const root = document.documentElement
	const rootStyle = getComputedStyle(root)
	const names = new Set()
	for (const sheet of Array.from(document.styleSheets)) {
		let rules = []
		try { rules = Array.from(sheet.cssRules) } catch (err) { continue }
		for (const rule of rules) {
			if (!rule.style) { continue }
			const on = (rule.selectorText || "").split(",").map(part => part.trim())
			if (!on.some(part => part === ":root" || part === "html")) { continue }
			for (const name of Array.from(rule.style)) {
				if (name.startsWith("--")) { names.add(name) }
			}
		}
	}
	if (names.size === 0) { return "the page declares no design tokens at :root" }
	const probe = document.createElement("span")
	document.body.appendChild(probe)
	const allowed = {}
	for (const property of properties) {
		const values = new Set(["transparent", "rgba(0, 0, 0, 0)"])
		for (const name of names) {
			const raw = rootStyle.getPropertyValue(name).trim()
			if (raw === "" || !CSS.supports(property, raw)) { continue }
			probe.style.setProperty(property, raw)
			values.add(getComputedStyle(probe).getPropertyValue(property).trim())
			probe.style.removeProperty(property)
		}
		allowed[property] = values
	}
	probe.remove()
	const named = (el) => {
		const own = el.getAttribute("data-testid")
		if (own) { return "[" + own + "]" }
		const host = el.closest("[data-testid]")
		return el.tagName.toLowerCase() +
			(host ? " in [" + host.getAttribute("data-testid") + "]" : "")
	}
	const paints = (style, property) => {
		const side = /^border-(top|right|bottom|left)-color$/.exec(property)
		if (side) {
			return style.getPropertyValue("border-" + side[1] + "-style") !== "none" &&
				parseFloat(style.getPropertyValue("border-" + side[1] + "-width")) > 0
		}
		if (property === "outline-color") {
			return style.outlineStyle !== "none" && parseFloat(style.outlineWidth) > 0
		}
		return true
	}
	const offenders = new Map()
	const els = [document.body].concat(
		Array.from(document.body.querySelectorAll("*")))
	for (const el of els) {
		if (exemptCSS !== "" && el.closest(exemptCSS)) { continue }
		const box = el.getBoundingClientRect()
		if (box.width === 0 || box.height === 0) { continue }
		const style = getComputedStyle(el)
		if (style.visibility === "hidden") { continue }
		for (const property of properties) {
			const got = style.getPropertyValue(property).trim()
			if (got === "" || !paints(style, property)) { continue }
			if (allowed[property].has(got)) { continue }
			const key = property + " = " + got
			const seen = offenders.get(key) || { where: named(el), count: 0 }
			seen.count++
			offenders.set(key, seen)
		}
	}
	if (offenders.size === 0) { return "ok" }
	const listed = Array.from(offenders.entries()).slice(0, limit).map(
		entry => entry[1].count + " element(s) paint " + entry[0] +
			", e.g. " + entry[1].where).join("; ")
	if (offenders.size <= limit) { return listed }
	return listed + "; and " + (offenders.size - limit) + " more value(s)"
})`

// designFaceFunc answers verdictOK when the design face is loaded AND measures
// differently from the fallback: a face that merely resolves in the cascade and
// never paints is what this clause exists to catch.
const designFaceFunc = `((face) => document.fonts.ready.then(() => {
	const declared = Array.from(document.fonts).filter(
		font => font.family.replace(/["']/g, "") === face)
	if (declared.length === 0) { return "the page declares no " + face + " face" }
	if (!document.fonts.check('32px "' + face + '"')) {
		return face + " is declared but no face of it is loaded"
	}
	const context = document.createElement("canvas").getContext("2d")
	const sample = "The design face renders"
	context.font = '32px "' + face + '", monospace'
	const withFace = context.measureText(sample).width
	context.font = "32px monospace"
	const fallback = context.measureText(sample).width
	if (Math.abs(withFace - fallback) < 0.5) {
		return face + " measures exactly as the monospace fallback does, so it " +
			"is not rendering"
	}
	return "ok"
}))`

// tokensResolveFunc answers verdictOK when the page defines every token named,
// and with the ones it does not.
const tokensResolveFunc = `((names) => {
	const style = getComputedStyle(document.documentElement)
	const missing = names.filter(
		name => style.getPropertyValue(name).trim() === "")
	if (missing.length === 0) { return "ok" }
	return "the page defines no " + missing.join(", ")
})`

// registerDesignPaletteSteps binds the page-wide conformance clauses: the sweep
// over every visible element, the face the design must actually paint in, and
// the clause naming several tokens the page must define.
func registerDesignPaletteSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^every visible element's (colours|type) comes? from the design token palette$`,
		assertPaletteSweep)
	suite.Step(`^the design face actually renders$`, assertDesignFaceRenders)
	suite.Step(`^the tokens (.+) all resolve$`, assertTokensResolve)
}

// assertPaletteSweep holds every visible element to painting only values the
// page's own tokens resolve to.
func assertPaletteSweep(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	aspect := args[0]
	properties, exempt := paletteProperties(aspect)

	got, matched, err := await(readPaletteSweep(page, properties, exempt),
		equals(verdictOK))
	if err != nil {
		return state.fail("reading whether every visible element's %s comes from "+
			"the design token palette: %w", aspect, err)
	}

	if !matched {
		return state.fail("every visible element's %s must come from the design "+
			"token palette: %s", aspect, got)
	}

	return nil
}

// assertDesignFaceRenders holds the design face to painting rather than merely
// resolving.
func assertDesignFaceRenders(state *State, _ []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	script := fmt.Sprintf(`() => %s(%q)`, designFaceFunc, designFace)

	got, matched, err := await(readPageProbe(page, script), equals(verdictOK))
	if err != nil {
		return state.fail("reading whether %s renders: %w", designFace, err)
	}

	if !matched {
		return state.fail("the design face %s must actually render: %s",
			designFace, got)
	}

	return nil
}

// assertTokensResolve holds the page to defining every token the clause names.
func assertTokensResolve(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	names := quotedList(args[0])
	if len(names) == 0 {
		return state.fail("%w: %q", ErrNoTokensNamed, args[0])
	}

	script := fmt.Sprintf(`() => %s(%s)`, tokensResolveFunc, jsQuotedList(names))

	got, matched, err := await(readPageProbe(page, script), equals(verdictOK))
	if err != nil {
		return state.fail("reading whether the tokens %s resolve: %w", args[0], err)
	}

	if !matched {
		return state.fail("the tokens %s must all resolve: %s", args[0], got)
	}

	return nil
}

// paletteProperties are the computed properties one sweep reads and the subtree
// it leaves alone.
func paletteProperties(aspect string) ([]string, string) {
	if aspect == typeAspect {
		return []string{"font-family", "font-size"}, monoExemptCSS
	}

	return []string{
		"color", "background-color", "border-top-color", "border-right-color",
		"border-bottom-color", "border-left-color", "outline-color",
	}, ""
}

// readPaletteSweep reads that verdict as a reader, so the sweep polls through
// the same await every value clause uses.
func readPaletteSweep(page playwright.Page, properties []string, exempt string,
) func() (string, error) {
	script := fmt.Sprintf(`() => %s(%s, %q, %d)`,
		paletteSweepFunc, jsQuotedList(properties), exempt, offenderLimit)

	return readPageProbe(page, script)
}

// readPageProbe runs a probe written to answer one string ABOUT THE PAGE, the
// page-wide twin of readProbe.
func readPageProbe(page playwright.Page, script string) func() (string, error) {
	return func() (string, error) { return probeString(page, script) }
}

// jsQuotedList renders a Go slice as the JS array literal a probe takes. It
// is not quotedList, which is the entries of a step's own quoted list.
func jsQuotedList(values []string) string {
	quoted := make([]string, 0, len(values))

	for _, value := range values {
		quoted = append(quoted, strconv.Quote(value))
	}

	return "[" + strings.Join(quoted, ", ") + "]"
}
