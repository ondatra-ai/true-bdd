package validate

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"

func resetConceptLinks(subject any) {
	storeConceptLinks(subject, nil, false)
}

func storeConceptLinks(subject any, links []template.ConceptLink, passed bool) {
	switch item := subject.(type) {
	case *template.ScenarioCheckData:
		item.ConceptLinks, item.ConceptLinkPassed = links, passed
	case *template.StoryRefineData:
		item.ConceptLinks, item.ConceptLinkPassed = links, passed
	}
}

func conceptLinks(subject any) ([]template.ConceptLink, bool) {
	switch item := subject.(type) {
	case *template.ScenarioCheckData:
		return item.ConceptLinks, item.ConceptLinkPassed
	case *template.StoryRefineData:
		return item.ConceptLinks, item.ConceptLinkPassed
	default:
		return nil, false
	}
}
