package commands_test

const narrowingHelperSource = `package helpers
import ("fmt"; "os"; "strings"; "gopkg.in/yaml.v3")
func NarrowChecklists(path, snippet string) error {
 data, err := os.ReadFile(path)
 if err != nil { return err }
 var document struct {
  Sections []struct {
   Prompts []map[string]any ` + "`yaml:\"validation_prompts\"`" + `
  } ` + "`yaml:\"sections\"`" + `
 }
 if err := yaml.Unmarshal(data, &document); err != nil { return err }
 matches := 0
 for index := range document.Sections {
  var selected []map[string]any
  for _, prompt := range document.Sections[index].Prompts {
   options, _ := prompt["Q"].(map[string]any)
   question, _ := options["prompt"].(string)
   if strings.Contains(question, snippet) { selected = append(selected, prompt); matches++ }
  }
  document.Sections[index].Prompts = selected
 }
 if matches != 1 { return fmt.Errorf("snippet matched %d prompts", matches) }
 data, err = yaml.Marshal(document)
 if err != nil { return err }
 return os.WriteFile(path, data, 0600)
}
`

func narrowingHostFiles() map[string]string {
 return map[string]string{
 "go.mod": "module fixture/narrow\n\ngo 1.25.0\nrequire gopkg.in/yaml.v3 v3.0.1\n",
 "go.sum": "gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=\n" +
  "gopkg.in/yaml.v3 v3.0.1/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=\n",
 "tests/calc/steps/state.go": "package steps\ntype State struct{ Path string }\n",
 "docs/scenarios.yaml": `scenarios:
  narrow-001:
    description: A checklist keeps its selected question
    service: calc
    path: tests/calc/narrow_test.go
    merged_steps:
      given: ['the checklist is narrowed to "Keep"']
      when: []
      then: []
`,
 "docs/product/product.yaml": `concepts:
  given:
  - concept: project tree
    means: The checklist keeps only the prompt containing the stated snippet.
    terms: []
`,
 "docs/architecture/architecture.yaml": `architecture:
  services:
  - name: calc
    path: services/calc
    public_interface:
      kind: cli
      document: docs/architecture/calc-cli.yaml
  concepts:
  - concept: project tree
    block: given
    service: calc
    references: [{command: build}]
    does: Narrow the checklist to the single prompt containing the stated snippet.
  testing:
    framework: go-test
    commands:
      record: go test -json ./tests/calc/...
      replay: go test -json ./tests/calc/...
      live: go test -json ./tests/calc/...
`,
 "docs/architecture/calc-cli.yaml": `opencli: 0.1
info: {title: Narrowing, version: '1.0'}
commands: {build: {description: Builds steps, responses: {'0': {description: Built}}}}
`,
 }
}
