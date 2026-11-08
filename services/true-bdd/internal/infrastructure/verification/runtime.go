package verification

import (
	"go/parser"
	"go/scanner"
	"go/token"
	"regexp"
	"slices"
	"strings"
)

type documentationDeclaration struct {
	name     string
	text     string
	function bool
}

// RuntimeDocumentation keeps called symbols and the complete types their declarations reference.
func RuntimeDocumentation(documentation, source string) string {
	names := sourceIdentifiers(source)
	identifier := regexp.MustCompile(`\b[A-Za-z_][A-Za-z_0-9]*\b`)

	declarations := documentationDeclarations(documentation)
	selected := make(map[int]bool)

	for changed := true; changed; {
		changed = false

		for index, declaration := range declarations {
			if selected[index] || !documentationUsed(declaration, names) {
				continue
			}

			selected[index], changed = true, true

			addDocumentationTypes(names, declaration.text, declarations, identifier)
		}
	}

	var result strings.Builder

	for index, declaration := range declarations {
		if selected[index] {
			result.WriteString(declaration.text)
			result.WriteString("\n")
		}
	}

	return result.String()
}

func documentationUsed(declaration documentationDeclaration, names map[string]bool) bool {
	if !strings.HasPrefix(declaration.text, "const ") && !strings.HasPrefix(declaration.text, "var ") {
		return names[declaration.name]
	}
	// Grouped const and var declarations remain whole, including their comments and values.
	for _, member := range documentationValueNames(declaration.text) {
		if names[member] {
			return true
		}
	}

	return false
}

func documentationValueNames(text string) []string {
	positions := token.NewFileSet()
	file := positions.AddFile("documentation.go", positions.Base(), len(text))

	var lexer scanner.Scanner
	lexer.Init(file, []byte(text), nil, 0)

	depth := 0

	for {
		position, kind, _ := lexer.Scan()
		if kind == token.EOF {
			return nil
		}

		if slices.Contains([]token.Token{token.LPAREN, token.LBRACK, token.LBRACE}, kind) {
			depth++
		}

		if slices.Contains([]token.Token{token.RPAREN, token.RBRACK, token.RBRACE}, kind) {
			depth--
		}

		if kind != token.SEMICOLON || depth != 0 {
			continue
		}

		// go doc follows the declaration with prose, which is not Go syntax.
		source := "package documentation\n" + text[:file.Offset(position)]

		tree, err := parser.ParseFile(token.NewFileSet(), "documentation.go", source, 0)
		if err != nil {
			return nil
		}

		names, _ := declarationNames(tree.Decls[0])

		return names
	}
}

func documentationDeclarations(documentation string) []documentationDeclaration {
	lines := strings.SplitAfter(documentation, "\n")
	declaration := regexp.MustCompile(`^(func (?:\([^)]*\) )?|type |const |var )([A-Za-z_]\w*)?`)
	declarations := make([]documentationDeclaration, 0)

	for _, line := range lines {
		matches := declaration.FindStringSubmatch(line)
		if matches != nil {
			declarations = append(declarations, documentationDeclaration{matches[2], line, strings.HasPrefix(line, "func ")})
		} else if len(declarations) > 0 && !slices.Contains(
			[]string{"CONSTANTS", "VARIABLES", "FUNCTIONS", "TYPES"}, strings.TrimSpace(line)) {
			last := &declarations[len(declarations)-1]
			last.text += line
		}
	}

	return declarations
}

func sourceIdentifiers(source string) map[string]bool {
	source = regexp.MustCompile(`(?m)^---[^\n]*---$`).ReplaceAllString(source, "")
	positions := token.NewFileSet()
	file := positions.AddFile("selected.go", positions.Base(), len(source))

	var lexer scanner.Scanner
	lexer.Init(file, []byte(source), nil, 0)

	names := make(map[string]bool)

	for {
		_, kind, literal := lexer.Scan()
		if kind == token.EOF {
			break
		}

		if kind == token.IDENT {
			names[literal] = true
		}
	}

	return names
}

func addDocumentationTypes(
	names map[string]bool, text string, declarations []documentationDeclaration, identifier *regexp.Regexp,
) {
	for _, name := range identifier.FindAllString(text, -1) {
		for _, dependency := range declarations {
			if !dependency.function && dependency.name == name {
				names[name] = true
			}
		}
	}
}
