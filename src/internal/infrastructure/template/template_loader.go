package template

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"bdd-cli/src/internal/pkg/errors"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v3"
)

// TemplateLoader is a generic template loader for any data type.
type TemplateLoader[T any] struct {
	templateFilePath string
	funcMap          template.FuncMap
}

// NewTemplateLoader creates a new generic TemplateLoader instance.
func NewTemplateLoader[T any](templateFilePath string) *TemplateLoader[T] {
	return &TemplateLoader[T]{
		templateFilePath: templateFilePath,
		funcMap:          getCommonTemplateFunctions(),
	}
}

// getCommonTemplateFunctions returns commonly used template functions.
func getCommonTemplateFunctions() template.FuncMap {
	// Start with Sprig's comprehensive function map
	funcMap := sprig.FuncMap()

	// Override or add custom template functions
	customFuncs := template.FuncMap{
		"toYaml": func(v interface{}) string {
			data, err := yaml.Marshal(v)
			if err != nil {
				return fmt.Sprintf("# Error marshaling to YAML: %v", err)
			}

			return string(data)
		},
		"nindent": func(spaces int, text string) string {
			lines := strings.Split(text, "\n")
			indent := strings.Repeat(" ", spaces)

			for i, line := range lines {
				if line != "" {
					lines[i] = indent + line
				}
			}

			return strings.Join(lines, "\n")
		},
	}

	// Merge custom functions, overriding Sprig's if there are conflicts
	for name, fn := range customFuncs {
		funcMap[name] = fn
	}

	return funcMap
}

// LoadTemplate loads and processes the template with the provided data.
func (l *TemplateLoader[T]) LoadTemplate(inputData T) (string, error) {
	// Load the template file
	templateContent, err := l.loadTemplateFile()
	if err != nil {
		return "", errors.ErrLoadTemplateFileFailed(err)
	}

	// Execute template directly with input data
	prompt, err := l.executeTemplate(templateContent, inputData)
	if err != nil {
		return "", errors.ErrExecuteTemplateFailed(err)
	}

	return prompt, nil
}

// loadTemplateFile loads the template file from disk.
func (l *TemplateLoader[T]) loadTemplateFile() (string, error) {
	content, err := os.ReadFile(l.templateFilePath)
	if err != nil {
		return "", errors.ErrReadTemplateFileFailed(l.templateFilePath, err)
	}

	return string(content), nil
}

// executeTemplate uses Go's text/template system to properly inject data.
func (l *TemplateLoader[T]) executeTemplate(templateContent string, data T) (string, error) {
	// Parse the template with custom functions
	tmpl, err := template.New("prompt").Funcs(l.funcMap).Parse(templateContent)
	if err != nil {
		return "", errors.ErrParseTemplateFailed(err)
	}

	// Execute the template with data
	var buf bytes.Buffer

	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", errors.ErrExecuteTemplateFailed(err)
	}

	return buf.String(), nil
}
