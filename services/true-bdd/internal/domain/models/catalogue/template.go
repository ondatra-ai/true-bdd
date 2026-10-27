package catalogue

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Why a template does not compile. Each completes the reason a fail line
// gives, after what it names: `text slot {reply} is not between double quotes`.
var (
	errEmptyTemplate    = errors.New("it is empty")
	errUnclosedSlot     = errors.New("a { is never closed")
	errUnclosedBrackets = errors.New("a [ is never closed")
	errStrayCloser      = errors.New("closes nothing")
	errLoneBackslash    = errors.New("it ends in a lone backslash")
	errBadEscape        = errors.New(`escapes nothing; a backslash escapes only {, }, [, ] and \`)
	errSlotName         = errors.New("is not named in lower-case letters and underscores")
	errSlotTwice        = errors.New("is used twice")
	errUnknownSlotKind  = errors.New("has an unknown kind; a slot is text, number, role or pattern")
	errUnquotedText     = errors.New("is not between double quotes")
	errNoRoles          = errors.New("but the product document declares no roles")
	errPatternNotLast   = errors.New("is not the last token")
	errPatternNoSpace   = errors.New("does not follow a space")
	errBracketContent   = errors.New("hold a slot, a quote or another bracket")
	errEmptyAlternative = errors.New("hold an empty alternative")
)

// The slot kinds a template may name after a colon; none is a text slot.
const (
	kindNumber  = "number"
	kindRole    = "role"
	kindPattern = "pattern"
)

// The regular expression each slot kind compiles to.
const (
	textSlotExpr    = `[^"]+`
	numberSlotExpr  = `(?:[0-9]+|no)`
	patternSlotExpr = `(?s:.+)`
)

// specials open or close a token; every other character is literal, and
// a backslash makes any of escapable literal too.
const (
	specials  = `\{}[]`
	escapable = `{}[]\`
)

// slotName is the shape of a slot's name.
var slotName = regexp.MustCompile(`^[a-z_]+$`)

// compiler turns one template into the regular expression a step's whole
// text must match, refusing a template the grammar does not allow.
type compiler struct {
	src   string
	pos   int
	roles []string
	names map[string]bool
	expr  strings.Builder
}

// compileTemplate compiles one template against the product's roles.
func compileTemplate(template string, roles []string) (*regexp.Regexp, error) {
	if strings.TrimSpace(template) == "" {
		return nil, errEmptyTemplate
	}

	comp := &compiler{src: template, roles: roles, names: make(map[string]bool)}
	comp.expr.WriteString(`\A`)

	for comp.pos < len(comp.src) {
		err := comp.token()
		if err != nil {
			return nil, err
		}
	}

	comp.expr.WriteString(`\z`)

	pattern, err := regexp.Compile(comp.expr.String())
	if err != nil {
		return nil, fmt.Errorf("compile template: %w", err)
	}

	return pattern, nil
}

// token compiles the token starting at pos and moves past it.
func (c *compiler) token() error {
	switch c.src[c.pos] {
	case '\\':
		char, width, err := escapedRune(c.src, c.pos)
		if err != nil {
			return err
		}

		c.expr.WriteString(regexp.QuoteMeta(string(char)))
		c.pos += 1 + width

		return nil
	case '{':
		return c.slot()
	case '[':
		return c.brackets()
	case '}', ']':
		return fmt.Errorf("a %c %w", c.src[c.pos], errStrayCloser)
	}

	end := c.pos
	for end < len(c.src) && !strings.ContainsRune(specials, rune(c.src[end])) {
		end++
	}

	c.expr.WriteString(regexp.QuoteMeta(c.src[c.pos:end]))
	c.pos = end

	return nil
}

// escapedRune reads the character a backslash at pos escapes, and its width.
func escapedRune(src string, pos int) (rune, int, error) {
	if pos+1 >= len(src) {
		return 0, 0, errLoneBackslash
	}

	char, width := utf8.DecodeRuneInString(src[pos+1:])
	if !strings.ContainsRune(escapable, char) {
		return 0, 0, fmt.Errorf(`\%c %w`, char, errBadEscape)
	}

	return char, width, nil
}

// slot compiles the `{name}` or `{name:kind}` slot starting at pos.
func (c *compiler) slot() error {
	closing := strings.IndexByte(c.src[c.pos:], '}')
	if closing < 0 {
		return errUnclosedSlot
	}

	end := c.pos + closing
	body := c.src[c.pos+1 : end]
	name, kind, hasKind := strings.Cut(body, ":")

	if !slotName.MatchString(name) {
		return fmt.Errorf("slot {%s} %w", body, errSlotName)
	}

	if c.names[name] {
		return fmt.Errorf("slot name %q %w", name, errSlotTwice)
	}

	c.names[name] = true

	if hasKind && kind == "" {
		return fmt.Errorf("slot {%s} %w", body, errUnknownSlotKind)
	}

	expr, err := c.slotExpr(body, kind, end)
	if err != nil {
		return err
	}

	c.expr.WriteString(expr)
	c.pos = end + 1

	return nil
}

// slotExpr is the expression of one slot of the given kind, which ends at
// end, after the rules its kind places on where it sits.
func (c *compiler) slotExpr(body, kind string, end int) (string, error) {
	switch kind {
	case "":
		if !c.quoted(end) {
			return "", fmt.Errorf("text slot {%s} %w", body, errUnquotedText)
		}

		return textSlotExpr, nil
	case kindNumber:
		return numberSlotExpr, nil
	case kindRole:
		if len(c.roles) == 0 {
			return "", fmt.Errorf("role slot {%s}, %w", body, errNoRoles)
		}

		return alternation(c.roles), nil
	case kindPattern:
		if end+1 != len(c.src) {
			return "", fmt.Errorf("pattern slot {%s} %w", body, errPatternNotLast)
		}

		if c.pos == 0 || c.src[c.pos-1] != ' ' {
			return "", fmt.Errorf("pattern slot {%s} %w", body, errPatternNoSpace)
		}

		return patternSlotExpr, nil
	}

	return "", fmt.Errorf("slot {%s} %w", body, errUnknownSlotKind)
}

// quoted reports whether the slot from pos to end sits between literal
// double quotes.
func (c *compiler) quoted(end int) bool {
	return c.pos > 0 && c.src[c.pos-1] == '"' && end+1 < len(c.src) && c.src[end+1] == '"'
}

// brackets compiles the `[a|b]` or `[s]` group starting at pos: one of
// its literal alternatives, or, with a single one, that text or nothing.
func (c *compiler) brackets() error {
	alternatives := [][]byte{{}}

	for pos := c.pos + 1; pos < len(c.src); {
		last := len(alternatives) - 1

		switch c.src[pos] {
		case ']':
			return c.closeBrackets(alternatives, pos)
		case '|':
			alternatives = append(alternatives, []byte{})
			pos++
		case '\\':
			char, width, err := escapedRune(c.src, pos)
			if err != nil {
				return err
			}

			alternatives[last] = utf8.AppendRune(alternatives[last], char)
			pos += 1 + width
		case '[', '{', '}', '"':
			return fmt.Errorf("brackets %s %w", c.bracketsText(), errBracketContent)
		default:
			alternatives[last] = append(alternatives[last], c.src[pos])
			pos++
		}
	}

	return errUnclosedBrackets
}

// closeBrackets emits the group whose closing bracket sits at end.
func (c *compiler) closeBrackets(alternatives [][]byte, end int) error {
	quoted := make([]string, 0, len(alternatives))

	for _, alternative := range alternatives {
		if len(alternative) == 0 {
			return fmt.Errorf("brackets %s %w", c.src[c.pos:end+1], errEmptyAlternative)
		}

		quoted = append(quoted, string(alternative))
	}

	group := alternation(quoted)
	if len(quoted) == 1 {
		group += "?"
	}

	c.expr.WriteString(group)
	c.pos = end + 1

	return nil
}

// bracketsText is the group starting at pos as written, up to the first
// closing bracket, for a reason naming it.
func (c *compiler) bracketsText() string {
	closing := strings.IndexByte(c.src[c.pos:], ']')
	if closing < 0 {
		return c.src[c.pos:]
	}

	return c.src[c.pos : c.pos+closing+1]
}

// alternation matches exactly one of the literal texts.
func alternation(texts []string) string {
	quoted := make([]string, 0, len(texts))
	for _, text := range texts {
		quoted = append(quoted, regexp.QuoteMeta(text))
	}

	return "(?:" + strings.Join(quoted, "|") + ")"
}
