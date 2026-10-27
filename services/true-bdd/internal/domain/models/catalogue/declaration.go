package catalogue

// Declaration is one term a step relies on: a term the entry it matched
// declares, beside that entry's template.
type Declaration struct {
	Template string
	Term     string
}

// Declarations lists the terms a run of steps relies on. Only a step a
// model does not run that matches exactly one template declares any; each
// (template, term) pair is listed once, in the order the steps reach it.
func (c *Catalogue) Declarations(steps []Step) []Declaration {
	declarations := make([]Declaration, 0)
	seen := make(map[Declaration]bool)

	for _, step := range steps {
		if step.ModelRun() {
			continue
		}

		matches := c.Match(step.Text)
		if len(matches) != 1 {
			continue
		}

		for _, term := range matches[0].Terms {
			declaration := Declaration{Template: matches[0].Template, Term: term}
			if seen[declaration] {
				continue
			}

			seen[declaration] = true

			declarations = append(declarations, declaration)
		}
	}

	return declarations
}
