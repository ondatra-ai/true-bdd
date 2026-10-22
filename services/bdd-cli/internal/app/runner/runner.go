package runner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
)

// renderedPrompt is the Q value the engine passes around. Pairs a
// PromptWithContext with its 1-based index so per-cell file naming
// stays stable across walks.
type renderedPrompt struct {
	Prompt checklistmodels.PromptWithContext
	Index  int
}

// renderPrompt is the engine's GenerateQFn for every us-* command.
// The index is the prompt's file position, not its walk position: `D:`
// prompts walk first, and a Cell's artifacts must not move with them.
func renderPrompt(_ int, prompt checklistmodels.PromptWithContext) *renderedPrompt {
	return &renderedPrompt{Prompt: prompt, Index: prompt.Index}
}

// Spec describes one checklist-driven command: its name, the checklist
// it drives, the items it walks, and per-command hooks (load, post-fix,
// finalize) — everything else lives in Run. Fill it in and call Run.
type Spec[I any] struct {
	// Name is the human-facing command name (e.g. "us refine").
	// Used in headers and error messages.
	Name string
	// ChecklistName is the checklist YAML stem (e.g. "us-refine").
	// Resolved by ChecklistLoader.
	ChecklistName string
	// StoryNumber is the positional CLI argument, the Story id: a slug
	// (`scen-check`, `96.2`), refused at startup when it is not one.
	StoryNumber string
	// Fix mirrors the --fix flag. Propagated into the engine's
	// CellHandler.FixMode.
	Fix bool

	// LoadItems is the per-command source: us refine loads from
	// docs/product/stories/, us apply parses the refined story into one
	// item per AC and seeds the scratch registry.
	LoadItems func(ctx context.Context) ([]I, error)
	// Prepare, if non-nil, runs after items load and before the first AI
	// turn: it may refuse the run or narrow the item set. `build tests`
	// narrows to unbound scenarios so a converged repo costs zero AI turns.
	Prepare func(ctx context.Context, items []I) ([]I, error)
	// PostFix is invoked after the FixApplier returns: us refine
	// unmarshals the new ACs and stores a new version; us apply's mutation
	// is already on disk via the Edit tool, so it returns the item unchanged.
	PostFix func(ctx context.Context, item I, applierContent string) (I, error)
	// PostCheckFix, if non-nil, runs in PostFix's place after the fix of a
	// failed `D:` Prompt, named by check: us refine's applier then edits a
	// term document, not the Story, so the Story stays as it reads.
	PostCheckFix func(ctx context.Context, item I, check string) (I, error)
	// Finalize handles the post-walk write: us refine updates the story
	// file in place; us apply atomically renames the scratch registry
	// over the canonical.
	Finalize func(result *engine.Result[I]) error
	// GetSubject reads the per-item (id, title) used for tmp file
	// naming and the post-walk report table.
	GetSubject func(item I) (subjectID, subjectTitle string)
	// OnItemStart, if non-nil, runs before each item walk; us apply uses
	// it for the "AC N/M: <description>" progress banner. Story-based
	// commands have one item and typically leave it nil.
	OnItemStart func(idx, total int, item I)

	// Checks is the built-in checks this command's `D:` prompts may name;
	// nil leaves it none, so any `D:` is refused at startup.
	Checks checks.Set[I]

	// Evaluator / FixGenerator / FixApplier form the generator triple: us
	// refine uses the standard triple, us apply the apply-flavoured
	// one (different templates, fix applier configured with EditMode).
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

// Run is the template method every `us` subcommand walks: validate,
// load, build and run the engine, render, then finalize — closures wrap
// Evaluator/FixGenerator/FixApplier so per-command code never touches them.
func Run[I any](ctx context.Context, spec Spec[I]) error {
	doc, prompts, err := validateSpec(spec)
	if err != nil {
		return err
	}

	// Execution phase.
	slog.Info("Walk starting", "header", headerLine(spec.Name, spec.StoryNumber))

	items, err := loadAndPrepare(ctx, spec)
	if err != nil {
		return err
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

	result, err := eng.Run(ctx, items, walkOrder(prompts))
	if err != nil {
		// Logged here: cobra prints the error only to stderr, which no
		// `stdout matches` step reads (#168).
		slog.Error("Walk stopped", "command", spec.Name, "error", err)

		return fmt.Errorf("%s command failed: %w", spec.Name, err)
	}

	builder.RenderAll(spec.Renderer, spec.Fix)

	// Trailing banner has no em-dash so fixture regexes like
	// "APPLY COMPLETE" / "REFINE COMPLETE" match as a contiguous
	// substring of the upper-cased command name.
	slog.Info(strings.ToUpper(spec.Name) + " COMPLETE")

	return spec.Finalize(result)
}

// validateSpec runs the whole validation phase: everything here must
// fail before the header, any item load, or the first AI turn, so a
// misconfigured run costs nothing.
func validateSpec[I any](spec Spec[I]) (
	*checklistmodels.Checklist,
	[]checklistmodels.PromptWithContext,
	error,
) {
	err := validateGenerators(spec)
	if err != nil {
		return nil, nil, refuseStartup(spec.Name, err)
	}

	if spec.StoryNumber != "" {
		err = validateStoryID(spec.StoryNumber)
		if err != nil {
			return nil, nil, refuseStartup(spec.Name, err)
		}
	}

	doc, err := spec.ChecklistLoader.LoadFull(spec.ChecklistName)
	if err != nil {
		return nil, nil, refuseStartup(spec.Name, fmt.Errorf("failed to load checklist: %w", err))
	}

	prompts := flattenChecklistPrompts(doc, spec.ChecklistName)

	err = validateChecklistHasPrompts(spec.ChecklistName, prompts)
	if err != nil {
		return nil, nil, refuseStartup(spec.Name, err)
	}

	err = validatePrompts(spec.Name, spec.Checks, prompts)
	if err != nil {
		return nil, nil, refuseStartup(spec.Name, err)
	}

	err = validateRequiredDocs(prompts, spec.DocResolver)
	if err != nil {
		// Both, deliberately: console is for whoever ran the command, the
		// log record is what a harness, CI scrape or BDD judge reads
		// afterwards — an unattributed refusal is barely better than the degradation it replaced.
		slog.Error("Refusing to start: checklist documents unsatisfiable",
			"command", spec.Name,
			"checklist", spec.ChecklistName,
			"error", err,
		)

		return nil, nil, err
	}

	// Asking for --fix against a checklist with no fix template asks for
	// something the walk can't deliver — refused here, before any paid
	// turn, rather than discovered one fix at a time.
	err = validateFixTemplates(spec.Fix, spec.ChecklistName, prompts)
	if err != nil {
		return nil, nil, refuseStartup(spec.Name, err)
	}

	err = validateModelTiers(spec, prompts)
	if err != nil {
		return nil, nil, refuseStartup(spec.Name, err)
	}

	return doc, prompts, nil
}

// loadAndPrepare sources the items and lets the command narrow or
// refuse them, reporting either failure as a startup refusal — both
// mean the walk about to start would measure the wrong thing.
func loadAndPrepare[I any](ctx context.Context, spec Spec[I]) ([]I, error) {
	items, err := spec.LoadItems(ctx)
	if err != nil {
		return nil, refuseStartup(spec.Name, fmt.Errorf("failed to load items: %w", err))
	}

	if spec.Prepare == nil {
		return items, nil
	}

	items, err = spec.Prepare(ctx, items)
	if err != nil {
		return nil, refuseStartup(spec.Name, err)
	}

	return items, nil
}

// buildSpecEngine wires the four-layer engine with closures driven
// entirely by the Spec's generator triple, GetSubject and PostFix, so
// per-command files never construct an engine directly.
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
	walker := &engine.SequentialWalker[I, *renderedPrompt]{
		Cell:     cell,
		MaxFixes: maxAttempts,
		Gate:     isCheckQuery,
		OnSkip:   recordSkip(spec, builder),
	}

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
	// Logged for every attempt, including the first, so a reader can tell
	// re-validation triggered by another item's fix (a re-walk) apart from
	// one triggered by this item's own fix (the walker's restart).
	slog.Info("Walk attempt started", "attempt", attempt, "max_attempts", maxAttempts)

	if attempt <= 1 {
		return
	}

	slog.Info("RE-WALK (fixes applied, verifying)",
		"attempt", attempt, "max_attempts", maxAttempts)
}

// itemBannerDispatcher adapts the engine's index-only OnItemStart to
// the spec's typed one, looking up the live item from the captured
// slice. A nil spec hook returns nil, so unused commands pay nothing.
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

// buildQueryClosure produces the engine.QueryFn that calls the Spec's
// Evaluator and side-effects into the report builder; the shared
// *latestResult slot lets buildGenFixClosure read the failing check.
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

		var (
			result checklistmodels.ValidationResult
			err    error
		)

		if isCheckQuery(query) {
			result, err = answerCheck(spec, item, subjectID, query)
		} else {
			result, err = spec.Evaluator.EvaluateOne(
				ctx, item, subjectID, query.Prompt, spec.TmpDir, query.Index,
			)
		}

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

// buildFixClosure produces the engine.FixFn: the captured fixCount
// keeps FixApplier tmp files uniquely named per run, and the shared
// *latestResult slot also carries the apply-model tier for this cell.
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
			Check:     latestResult.Check,
			TmpDir:    spec.TmpDir,
			Iteration: fixCount,
			ModelTier: latestResult.ApplyModelTier,
		})
		if err != nil {
			return item, fmt.Errorf("fix applier failed: %w", err)
		}

		if latestResult.Check != "" && spec.PostCheckFix != nil {
			return spec.PostCheckFix(ctx, item, latestResult.Check)
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
// non-skipped prompts in file order, each with its section context and
// its position; pkg/testkit/cmd/coverage mirrors this loop for its census.
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
				Index:         len(prompts) + 1,
				Prompt:        prompt,
			})
		}
	}

	return prompts
}
