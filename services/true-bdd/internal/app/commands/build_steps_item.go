package commands

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"

// BuildStepsItem is one Catalogue phrase missing a definition in one go-test suite.
type BuildStepsItem struct {
 Suite string
 Service string
 Phrase catalogue.Entry
}
