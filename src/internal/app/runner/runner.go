package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/src/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/src/internal/app/generators/validate"
	checklistmodels "github.com/ondatra-ai/true-bdd/src/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/events"
	"github.com/ondatra-ai/true-bdd/src/internal/pkg/console"
)

// ErrExpectedNonconvergence marks a finalize error that is an EXPECTED,
// non-erroneous nonconvergence — a `build tests` / `build code` walk that
// legitimately did not converge (plan §3.2 / finding 7). The command still
// returns the error so the CLI exit code is unchanged (mirror, don't fix),
// but the terminal envelope reports finalization_ok=true and a not_fixed
// classification rather than a finalization FAILURE. Build finalizers wrap
// this sentinel; genuine write failures (e.g. a story-file write) do not.
var ErrExpectedNonconvergence = errors.New("expected nonconvergence")

// renderedPrompt is the Q value the engine passes around. Pairs a
// PromptWithContext with its 1-based index so per-cell file naming
// stays stable across walks.
type renderedPrompt struct {
	Prompt checklistmodels.PromptWithContext
	Index  int
}

// renderPrompt is the engine's GenerateQFn for every us-* command.
// Cheap on purpose — the heavy template rendering happens inside the
// evaluator's Claude call, which has both item and q.
func renderPrompt(idx int, prompt checklistmodels.PromptWithContext) *renderedPrompt {
	return &renderedPrompt{Prompt: prompt, Index: idx}
}

// Spec describes one checklist-driven command: its name, the
// checklist YAML it drives, the items it walks, and the per-command
// hooks (load + post-fix + finalize). Everything else — header,
// engine wiring, report rendering — is identical across commands and
// lives in Run.
//
// To author a new us-* command, fill in this struct and call Run.
type Spec[I any] struct {
	// Name is the human-facing command name (e.g. "us create").
	// Used in headers and error messages.
	Name string
	// ChecklistName is the checklist YAML stem (e.g. "us-create").
	// Resolved by ChecklistLoader.
	ChecklistName string
	// StoryNumber is the positional CLI argument (`4.1`).
	StoryNumber string
	// Fix mirrors the --fix flag. Propagated into the engine's
	// CellHandler.FixMode.
	Fix bool

	// LoadItems is the per-command source. For us create it loads
	// from the epic; us refine from docs/prd/stories/; us apply parses
	// the refined story into one item per AC plus seeding the
	// scratch registry.
	LoadItems func(ctx context.Context) ([]I, error)
	// PostFix is invoked after the FixApplier returns. For us
	// create/refine it unmarshals the new ACs and stores a new
	// version; for us apply the mutation is already on disk via the
	// Edit tool so it just returns the item unchanged.
	PostFix func(ctx context.Context, item I, applierContent string) (I, error)
	// Finalize handles the post-walk write. For us create it writes
	// a new story file; us refine updates in place; us apply
	// atomically renames the scratch registry over the canonical.
	Finalize func(result *engine.Result[I]) error
	// GetSubject reads the per-item (id, title) used for tmp file
	// naming and the post-walk report table.
	GetSubject func(item I) (subjectID, subjectTitle string)
	// OnItemStart, if non-nil, is invoked before each item walk. us
	// apply uses this to print the "AC N/M: <description>" banner so
	// per-AC progress is visible in stdout. Story-based commands have
	// a single item and typically leave this nil.
	OnItemStart func(idx, total int, item I)

	// Evaluator / FixGenerator / FixApplier together form the
	// generator triple. us create/refine use the standard triple;
	// us apply uses the apply-flavoured triple (different templates,
	// fix-applier configured with EditMode).
	Evaluator    *validate.ChecklistEvaluator
	FixGenerator *validate.FixPromptGenerator
	FixApplier   *validate.FixApplier

	// ChecklistLoader, DocResolver, Renderer, UI are static
	// dependencies. DocResolver backs the up-front check that every
	// document the checklist's prompts declare actually exists.
	ChecklistLoader *checklist.ChecklistLoader
	DocResolver     *docs.Resolver
	Renderer        *TableRenderer
	UI              engine.FixLoopUI

	// TmpDir is the per-run output directory. Prompts, responses,
	// and per-cell artifacts are written here.
	TmpDir string
}

// Run is the template method every `us` subcommand walks through:
//  1. Validate the story number.
//  2. Print the command header.
//  3. LoadItems (per-command).
//  4. Load the checklist.
//  5. Build the engine with shared closures.
//  6. Run the engine.
//  7. Render the per-item report tables.
//  8. Finalize (per-command).
//
// The shared closures package up Evaluator/FixGenerator/FixApplier
// calls so per-command code never touches them directly.
func Run[I any](ctx context.Context, spec Spec[I]) error {
	// Validation phase. Everything here is a precondition on the run
	// itself — it must fail before the header, before any item is
	// loaded, and above all before the first AI turn, so a
	// misconfigured run costs nothing.
	if spec.StoryNumber != "" {
		err := validateStoryNumber(spec.StoryNumber)
		if err != nil {
			return fmt.Errorf("invalid story number: %w", err)
		}
	}

	doc, err := spec.ChecklistLoader.LoadFull(spec.ChecklistName)
	if err != nil {
		return fmt.Errorf("failed to load checklist: %w", err)
	}

	prompts := flattenChecklistPrompts(doc, spec.ChecklistName)

	err = validateRequiredDocs(prompts, spec.DocResolver)
	if err != nil {
		// Both, deliberately. The console line is for whoever ran the
		// command; the log record is what a harness, a CI scrape, or the
		// BDD judge reads afterwards, and it must name the offending
		// document there too — a refusal nobody can attribute is barely
		// better than the silent degradation this check replaced.
		slog.Error("Refusing to start: checklist documents unsatisfiable",
			"command", spec.Name,
			"checklist", spec.ChecklistName,
			"error", err,
		)
		console.Println("Cannot start: " + err.Error())

		return err
	}

	// Execution phase.
	console.Header(headerLine(spec.Name, spec.StoryNumber), SeparatorWidth)

	items, err := spec.LoadItems(ctx)
	if err != nil {
		return fmt.Errorf("failed to load items: %w", err)
	}

	maxAttempts := 0

	if doc.Config != nil && doc.Config.MaxApplyAttempts > 0 {
		maxAttempts = doc.Config.MaxApplyAttempts
	}

	slog.Info("Loaded prompts",
		"command", spec.Name,
		"items", len(items),
		"prompts", len(prompts),
		"max_apply_attempts", maxAttempts,
	)

	builder := newReportBuilder()
	eng := buildSpecEngine(spec, builder, items, maxAttempts)

	result, err := eng.Run(ctx, items, prompts)
	if err != nil {
		return fmt.Errorf("%s command failed: %w", spec.Name, err)
	}

	builder.RenderAll(spec.Renderer, spec.Fix)

	// Trailing banner has no em-dash so fixture regexes like
	// "APPLY COMPLETE" / "CREATE COMPLETE" match as a contiguous
	// substring of the upper-cased command name.
	console.Header(
		strings.ToUpper(spec.Name)+" COMPLETE",
		SeparatorWidth,
	)

	finErr := spec.Finalize(result)

	emitRunResult(result.Reason, finErr)

	return finErr
}

// emitRunResult publishes the terminal result event AFTER finalization
// (plan §3.2 / finding 7). An EXPECTED build nonconvergence is NOT a
// finalization failure: it reports finalization_ok=true and a not_fixed
// classification (the CLI still exits non-zero via the returned error —
// mirror, don't fix). A genuine post-walk write failure keeps
// finalization_ok=false and its detail. The emitter is the process-wide
// instance the collector uses for prompts, so the result ordinal follows
// them. No-op without the event-channel env var.
func emitRunResult(reason engine.StopReason, finErr error) {
	expectedNonconvergence := errors.Is(finErr, ErrExpectedNonconvergence)
	finalizationOK := finErr == nil || expectedNonconvergence

	outcome := outcomeForReason(reason)
	if expectedNonconvergence && outcome == "error" {
		outcome = "not_fixed"
	}

	detail := ""
	if finErr != nil && !expectedNonconvergence {
		detail = finErr.Error()
	}

	events.NewEmitter().EmitResult(outcome, finalizationOK, detail)
}

// outcomeForReason maps the engine's stop reason to the event-channel
// outcome vocabulary the harness server understands (plan §3.2/§3.3).
func outcomeForReason(reason engine.StopReason) string {
	switch reason {
	case engine.Converged:
		return "converged"
	case engine.UserExit:
		return "user_exit"
	case engine.NotFixed:
		return "not_fixed"
	case engine.MaxAttemptsExhausted:
		return "max_attempts"
	}

	return "error"
}

// buildSpecEngine wires the four-layer engine with closures whose
// behaviour is determined entirely by the Spec's generator triple,
// GetSubject, and PostFix. Per-command files never construct an
// engine directly. `items` is captured so the engine's index-only
// OnItemStart can dispatch to the spec's typed OnItemStart hook.
func buildSpecEngine[I any](
	spec Spec[I],
	builder *reportBuilder,
	items []I,
	maxAttempts int,
) *engine.Engine[I, checklistmodels.PromptWithContext, *renderedPrompt] {
	var latestResult checklistmodels.ValidationResult

	cell := &engine.CellHandler[I, *renderedPrompt]{
		Query:   buildQueryClosure(spec, builder, &latestResult),
		GenFix:  buildGenFixClosure(spec, &latestResult),
		Fix:     buildFixClosure(spec, &latestResult),
		UI:      spec.UI,
		FixMode: spec.Fix,
	}
	// Same budget on both loops: max_apply_attempts caps the fixes one
	// walk may apply as well as the outer re-walks, so a cell that never
	// converges fails with a named error instead of spinning.
	walker := &engine.SequentialWalker[I, *renderedPrompt]{Cell: cell, MaxFixes: maxAttempts}

	return engine.New(
		renderPrompt, walker,
		engine.Options{
			MaxApplyAttempts: maxAttempts,
			OnAttemptStart:   reWalkBanner,
			OnItemStart:      itemBannerDispatcher(spec, items),
		},
	)
}

// reWalkBanner emits the "RE-WALK N/M (fixes applied — verifying)"
// banner at the top of every outer-walk attempt past the first.
// Attempt 1 is the initial walk; banners only make sense on retries.
func reWalkBanner(attempt, maxAttempts int) {
	if attempt <= 1 {
		return
	}

	console.Header(
		fmt.Sprintf("RE-WALK %d/%d (fixes applied — verifying)", attempt, maxAttempts),
		SeparatorWidth,
	)
}

// itemBannerDispatcher adapts the engine's index-only OnItemStart to
// the spec's typed OnItemStart, looking up the live item from the
// captured slice. Nil out → no-op so commands that omit the spec hook
// pay nothing.
func itemBannerDispatcher[I any](
	spec Spec[I],
	items []I,
) func(idx, total int) {
	if spec.OnItemStart == nil {
		return nil
	}

	return func(idx, total int) {
		if idx < 0 || idx >= len(items) {
			return
		}

		spec.OnItemStart(idx, total, items[idx])
	}
}

// buildQueryClosure produces the engine.QueryFn that calls the
// Spec's Evaluator and side-effects into the report builder. The
// shared *latestResult slot lets buildGenFixClosure read the failing
// check on the next iteration.
func buildQueryClosure[I any](
	spec Spec[I],
	builder *reportBuilder,
	latestResult *checklistmodels.ValidationResult,
) engine.QueryFn[I, *renderedPrompt] {
	return func(
		ctx context.Context,
		item I,
		query *renderedPrompt,
	) (bool, error) {
		subjectID, subjectTitle := spec.GetSubject(item)

		result, err := spec.Evaluator.EvaluateOne(
			ctx, item, subjectID, query.Prompt, spec.TmpDir, query.Index,
		)
		if err != nil {
			return false, fmt.Errorf("evaluator failed: %w", err)
		}

		*latestResult = result
		builder.Add(subjectID, subjectTitle, result)

		return result.Status == checklistmodels.StatusPass, nil
	}
}

// buildGenFixClosure produces the engine.GenerateFixFn. The first
// iteration displays the failure under a banner; every iteration
// pipes the latestResult into runFixPromptGeneration.
func buildGenFixClosure[I any](
	spec Spec[I],
	latestResult *checklistmodels.ValidationResult,
) engine.GenerateFixFn[I, *renderedPrompt] {
	return func(
		ctx context.Context,
		item I,
		_ *renderedPrompt,
		userAnswers map[string]string,
		iteration int,
	) (engine.FixResult, error) {
		if iteration == 1 {
			displayFailureInfo(latestResult)
		}

		subjectID, _ := spec.GetSubject(item)

		return runFixPromptGeneration(ctx, fixPromptGenInput{
			generator:   spec.FixGenerator,
			subject:     item,
			subjectID:   subjectID,
			failedCheck: *latestResult,
			tmpDir:      spec.TmpDir,
			userAnswers: userAnswers,
			iteration:   iteration,
		})
	}
}

// buildFixClosure produces the engine.FixFn. The captured fixCount
// keeps FixApplier tmp files uniquely named across multiple cell
// invocations within one run. The shared *latestResult slot carries
// the apply-model tier the evaluator resolved for this cell — the same
// channel buildGenFixClosure reads the failing check from.
func buildFixClosure[I any](
	spec Spec[I],
	latestResult *checklistmodels.ValidationResult,
) engine.FixFn[I] {
	fixCount := 0

	return func(
		ctx context.Context,
		item I,
		decision engine.FixDecision,
	) (I, error) {
		fixCount++
		subjectID, _ := spec.GetSubject(item)

		content, err := spec.FixApplier.Apply(ctx, validate.ApplyParams{
			Subject:   item,
			SubjectID: subjectID,
			FixPrompt: decision.FixPrompt,
			TmpDir:    spec.TmpDir,
			Iteration: fixCount,
			ModelTier: latestResult.ApplyModelTier,
		})
		if err != nil {
			return item, fmt.Errorf("fix applier failed: %w", err)
		}

		return spec.PostFix(ctx, item, content)
	}
}

// headerLine builds the opening banner for one runner.Run. Story-based
// commands append "— Story <number>"; story-less commands (build-*) get
// just the upper-cased name.
func headerLine(name, storyNumber string) string {
	upper := strings.ToUpper(name)

	if storyNumber == "" {
		return upper
	}

	return upper + " — Story " + storyNumber
}

// flattenChecklistPrompts walks a Checklist's sections and emits the
// non-skipped prompts with section context attached. Mirrors the loop
// inside ChecklistLoader.Load so Run can use LoadFull (to read the
// config block) and still produce the flat prompt list the engine
// expects.
func flattenChecklistPrompts(
	doc *checklistmodels.Checklist,
	commandName string,
) []checklistmodels.PromptWithContext {
	prompts := make([]checklistmodels.PromptWithContext, 0)

	for _, section := range doc.Sections {
		for _, prompt := range section.ValidationPrompts {
			if prompt.ShouldSkip() {
				continue
			}

			prompts = append(prompts, checklistmodels.PromptWithContext{
				SectionID:     commandName,
				SectionName:   commandName,
				CriterionID:   section.ID,
				CriterionName: section.Name,
				DefaultDocs:   doc.DefaultDocs,
				DefaultModels: doc.Engine,
				Prompt:        prompt,
			})
		}
	}

	return prompts
}
