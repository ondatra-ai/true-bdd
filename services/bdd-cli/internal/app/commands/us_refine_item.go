package commands

import (
	"context"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

// refineItem wraps a loaded Story as the Item `us refine` walks.
func refineItem(loaded *story.Story, termDocuments []template.FixDocument) *template.StoryRefineData {
	return &template.StoryRefineData{Story: *loaded, TermDocuments: termDocuments}
}

// refineSubject is the GetSubject implementation: the Story's id and title.
func refineSubject(item *template.StoryRefineData) (string, string) {
	return runner.StorySubject(&item.Story)
}

// refinePostFix takes the Story body a model Prompt's fix wrote as the
// Story's next version, keeping the documents the walk seeded.
func refinePostFix(
	versionMgr *fs.StoryVersionManager,
) func(context.Context, *template.StoryRefineData, string) (*template.StoryRefineData, error) {
	postFix := runner.StoryPostFix(versionMgr)

	return func(
		ctx context.Context,
		item *template.StoryRefineData,
		applierContent string,
	) (*template.StoryRefineData, error) {
		latest, err := postFix(ctx, &item.Story, applierContent)
		if err != nil {
			return nil, err
		}

		return refineItem(latest, item.TermDocuments), nil
	}
}

// refinePostCheckFix commits the term document a closure fix edited. The
// Story keeps its version: the fix phrased its step in the catalogue.
func refinePostCheckFix(
	_ context.Context,
	item *template.StoryRefineData,
	_ string,
) (*template.StoryRefineData, error) {
	_, err := commitDocuments(item.TermDocuments, nil)
	if err != nil {
		return nil, err
	}

	return item, nil
}

// seedRefineDocuments gives the loaded Story the working copies a closure
// fix edits under --fix: one per term document the host has.
func seedRefineDocuments(
	load func(context.Context) ([]*template.StoryRefineData, error),
	resolver *docs.Resolver,
	tmpDir string,
) func(context.Context) ([]*template.StoryRefineData, error) {
	return func(ctx context.Context) ([]*template.StoryRefineData, error) {
		items, err := load(ctx)
		if err != nil {
			return nil, err
		}

		termDocuments, err := seedTermDocuments(resolver, tmpDir)
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			item.TermDocuments = termDocuments
		}

		return items, nil
	}
}

// refineFinalize hands the walk's result to the Story finalizer, which
// writes the Story's latest version over its file on convergence.
func refineFinalize(
	finalize func(*engine.Result[*story.Story]) error,
) func(*engine.Result[*template.StoryRefineData]) error {
	return func(result *engine.Result[*template.StoryRefineData]) error {
		stories := make([]*story.Story, 0, len(result.Items))
		for _, item := range result.Items {
			stories = append(stories, &item.Story)
		}

		return finalize(&engine.Result[*story.Story]{
			Items:     stories,
			AllPassed: result.AllPassed,
			Reason:    result.Reason,
		})
	}
}
