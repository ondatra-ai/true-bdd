package commands

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// applyCritique adds the critique's findings to the Item's. On a concept
// Item a finding against a pattern first sends the Item back to its
// proposal; a finding against a body goes back to the writer as it is.
func (e *buildStepsExecution) applyCritique(ctx context.Context, item *BuildStepsItem,
	findings []validate.CritiqueFinding,
) error {
	critique := make([]string, 0)

	for _, finding := range findings {
		item.Findings = append(item.Findings, finding.String())

		if item.Concept != nil && finding.Pattern() {
			critique = append(critique, "critique: "+finding.String())
		}
	}

	if len(critique) == 0 {
		return nil
	}

	return e.repairPatterns(ctx, item, critique)
}

// repairPatterns proposes the Item's patterns again with the critique's
// findings, holds the new proposal to the engine's check and rewrites the
// skeletons it changes; their bodies then go back to the writer.
func (e *buildStepsExecution) repairPatterns(ctx context.Context, item *BuildStepsItem, critique []string) error {
	run := e.concepts
	run.rounds[item]++

	slog.Warn("Critique sent the patterns back to the proposal", "suite", item.Suite, "concept", item.Label(),
		"round", run.rounds[item], "findings", critique)

	label := fmt.Sprintf("repair %d proposal", run.rounds[item])

	err := e.proposeConcept(ctx, run.proposal, run.attempts, item, run.repairSubject(item, critique), label)
	if err != nil {
		return err
	}

	return e.rewriteSkeletons(item)
}

// repairSubject is the Item's proposal again: its own steps are its own
// whatever binds them now, its own patterns refused with the findings.
func (run *conceptRun) repairSubject(item *BuildStepsItem, critique []string) *template.ConceptProposalData {
	key := conceptKey(item.Concept.Block, item.Concept.Name)
	subject := &template.ConceptProposalData{ID: key, Suite: item.Suite, Service: item.Service,
		Block: item.Concept.Block, Concept: item.Concept.Name, Steps: append([]string{}, item.Concept.Steps...),
		Previous: item.Concept.proposed(), Critique: critique}

	for _, pattern := range item.Concept.Existing {
		if !slices.ContainsFunc(item.Concept.Definitions, func(own stepskeleton.Definition) bool {
			return own.Pattern == pattern
		}) {
			subject.Existing = append(subject.Existing, pattern)
		}
	}

	for _, step := range run.discovery.steps[item.Suite] {
		if step.Block == item.Concept.Block && slices.Contains(item.Concept.Steps, step.Text) {
			step.Item, step.Bound = key, ""
		}

		subject.Suites = append(subject.Suites, step)
	}

	return subject
}

// proposed is the Item's current patterns as its proposal stated them,
// or as the registrations of resumed skeletons do.
func (concept *BuildStepsConcept) proposed() []catalogue.ProposedPattern {
	patterns := make([]catalogue.ProposedPattern, 0, len(concept.Definitions))

	for _, definition := range concept.Definitions {
		pattern := catalogue.ProposedPattern{Pattern: definition.Pattern, Captures: []catalogue.PatternCapture{}}

		for _, accepted := range concept.Patterns {
			if accepted.Pattern == definition.Pattern {
				pattern = accepted
			}
		}

		patterns = append(patterns, pattern)
	}

	return patterns
}

// rewriteSkeletons removes each skeleton whose pattern the new proposal
// dropped, writes one for each new pattern and keeps the unchanged ones.
func (e *buildStepsExecution) rewriteSkeletons(item *BuildStepsItem) error {
	removed := 0

	for _, definition := range item.Concept.Definitions {
		if slices.ContainsFunc(item.Concept.Patterns, func(pattern catalogue.ProposedPattern) bool {
			return pattern.Pattern == definition.Pattern
		}) {
			continue
		}

		err := stepskeleton.RemoveConcept(item.Suite, definition)
		if err != nil {
			return err
		}

		removed++
	}

	item.Concept.Definitions = nil

	for _, pattern := range item.Concept.Patterns {
		definition, err := stepskeleton.WriteConcept(item.Suite, string(item.Concept.Block), item.Concept.Name,
			pattern.Pattern)
		if err != nil {
			return err
		}

		item.Concept.Definitions = append(item.Concept.Definitions, definition)
	}

	slog.Info("Concept skeletons rewritten", "suite", item.Suite, "concept", item.Label(), "removed", removed,
		"definitions", len(item.Concept.Definitions))

	return e.afterRewrite(item)
}

// afterRewrite gives the writer the rewritten Item: its context, its shape
// and a finding for every body still to write.
func (e *buildStepsExecution) afterRewrite(item *BuildStepsItem) error {
	run := e.concepts

	err := describeConceptItem(item, run.arch, run.specification, run.discovery)
	if err != nil {
		return err
	}

	err = e.compiler.CompileDefinitions(e.command)
	if err != nil {
		item.Findings = append(item.Findings, err.Error())
	}

	e.shape, err = stepskeleton.Shape(item.Suite, item.functions()...)
	if err != nil {
		return fmt.Errorf("read the shape of %s after its rewrite: %w", item.Label(), err)
	}

	e.active = item

	for _, definition := range item.Concept.Definitions {
		if definition.Written() != nil {
			item.Findings = append(item.Findings, "step not written yet: "+item.Label())

			break
		}
	}

	return nil
}
