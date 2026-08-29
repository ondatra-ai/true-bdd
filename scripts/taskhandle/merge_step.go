package taskhandle

import (
	"github.com/ondatra-ai/true-bdd/scripts/merge"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// mergeStep lands the PR: merge.Embed IS merge.Start(nil).Main(), in this
// process, with the render suppressed. The mandate is re-read first — after
// this point the squash is not reversible.
func (r *Run) mergeStep() error {
	defer report.Open(StepMerge.Label())()

	err := r.requireMandate(StepMerge)
	if err != nil {
		return err
	}

	err = merge.Embed()
	if err != nil {
		r.list.mark(StepMerge, markFail, "merge stopped: "+firstLine(err.Error()))

		return halt(StepMerge, err)
	}

	// merge ends on a freshly pulled trunk, so HEAD is the squash commit.
	sha, err := line(gitBin, "rev-parse", "--short", "HEAD")
	if err != nil {
		r.list.mark(StepMerge, markWarn, "merged; the squash sha could not be read")

		return nil
	}

	r.sha = sha
	r.list.mark(StepMerge, markDone, r.pullRequest+" `"+sha+"`")

	return nil
}
