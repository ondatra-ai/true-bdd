package commands

import (
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
)

// BuildStepsItem is one Catalogue phrase missing a definition in one go-test suite.
type BuildStepsItem struct {
	subjectID  string
	Context    string
	Definition stepskeleton.Definition
	Findings   []string
	Suite      string
	Service    string
	Phrase     catalogue.Entry
}
