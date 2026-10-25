package drill_test

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/drill"
)

const specificationMenu = "[1] Apply this fix to working copy\n[2] Refine the fix prompt\n[3] Exit without applying\n"

const originalClarification = "[1/1] What is specified?\n    Options:\n" +
	"      1) Build from Specification\n      0) Other (type custom answer)\n"

func promptEngine(t *testing.T) (io.Writer, *bytes.Buffer, drill.Record, drill.Attempt) {
	t.Helper()
	runner, record, _ := prepare(t, "#!/bin/sh\nexit 0\n")
	run, err := runner.Prepare(t.Context(), record.Directory, "a", "prompts",
		[]string{buildCommand, codeArgument, fixArgument})
	must(t, err)

	input := &bytes.Buffer{}
	writer, stop := drill.NewPromptWriter(t.Context(), runner, record, run, input)
	t.Cleanup(stop)

	return writer, input, record, run
}

func TestPromptWriterPausesForSpecificationClarification(t *testing.T) {
	writer, input, record, run := promptEngine(t)
	question := "How should the service behave?\n1) Follow the Specification\n2) Restore from history\n0) Other\n"
	engineOutput(t, writer, question)

	saved, err := drill.Read(record.Directory)
	must(t, err)

	paused := saved.Runs[run.Tag]
	if paused.Status != pausedStatus {
		t.Fatalf("run status = %q; want paused", paused.Status)
	}

	assertUnansweredQuestion(t, paused)

	if paused.Questions[0].Prompt != question || paused.Questions[0].AnswerFile == "" {
		t.Fatalf("question not retained: %+v", paused.Questions[0])
	}

	if !slices.Equal(paused.Questions[0].Options,
		[]string{"1) Follow the Specification", "2) Restore from history", "0) Other"}) {
		t.Fatalf("options lost: %v", paused.Questions[0].Options)
	}

	if input.Len() != 0 {
		t.Fatalf("clarification answered automatically: %q", input.String())
	}
}

func engineOutput(t *testing.T, writer io.Writer, chunks ...string) {
	t.Helper()

	for _, chunk := range chunks {
		count, err := io.WriteString(writer, chunk)
		must(t, err)

		if count != len(chunk) {
			t.Fatalf("wrote %d of %d engine bytes", count, len(chunk))
		}
	}

	drill.WaitPromptWriter(writer)
}

func TestPromptWriterAppliesSpecificationMenu(t *testing.T) {
	writer, input, _, _ := promptEngine(t)
	engineOutput(t, writer, "FIX PROMPT GENERATED\nRepair the service.\n", specificationMenu)

	if input.String() != "1\n" {
		t.Fatalf("engine input = %q; want one apply answer", input.String())
	}
}

func TestPromptWriterAnswersMenuWithLegacyPromptOnce(t *testing.T) {
	for _, chunks := range [][]string{
		{specificationMenu + "Your choice (1/2/3): "},
		{specificationMenu, "Your choice (1/2/3): "},
	} {
		t.Run(chunks[0], func(t *testing.T) {
			writer, input, _, _ := promptEngine(t)
			engineOutput(t, writer, chunks...)

			if input.String() != "1\n" {
				t.Fatalf("engine input = %q; want one apply answer", input.String())
			}
		})
	}
}

func TestPromptWriterAnswersOriginalMenuWithSeparatorOnce(t *testing.T) {
	menu := "  [1] Apply this fix to working copy\n" +
		"  [2] Refine the fix prompt (provide feedback)\n  [3] Exit without applying\n"
	separator := "============================================================\n"

	for _, prompt := range []string{"", "Your choice (1/2/3): "} {
		t.Run("prompt="+prompt, func(t *testing.T) {
			output := menu + separator + prompt
			for name, chunks := range map[string][]string{
				"one chunk":       {output},
				"separate footer": {menu, separator, prompt},
				"bytewise":        strings.Split(output, ""),
			} {
				t.Run(name, func(t *testing.T) {
					writer, input, _, _ := promptEngine(t)
					engineOutput(t, writer, chunks...)

					if input.String() != "1\n" {
						t.Fatalf("engine input = %q; want one apply answer", input.String())
					}
				})
			}
		})
	}
}

func TestPromptWriterMenuAcrossEveryChunkBoundary(t *testing.T) {
	writer, input, _, _ := promptEngine(t)

	for _, output := range []string{specificationMenu, specificationMenu + "Your choice (1/2/3): "} {
		input.Reset()

		for split := range len(output) + 1 {
			engineOutput(t, writer, "\nNext fix plan\n", output[:split], output[split:])

			if input.String() != strings.Repeat("1\n", split+1) {
				t.Fatalf("split at %d: engine input = %q", split, input.String())
			}
		}
	}
}

func TestPromptWriterIgnoresMenuQuotedInPlan(t *testing.T) {
	writer, input, _, _ := promptEngine(t)
	for _, output := range []string{
		"The plan quotes [3] Exit without applying mid-text.\n",
		"[3] Exit without applying\n",
		"[1] Apply this fix to working copy\n[3] Exit without applying\n",
		"[2] Refine the fix prompt\n[1] Apply this fix to working copy\n[3] Exit without applying\n",
		"Example:\n" + specificationMenu + "The plan continues here.\n",
		"prefix " + specificationMenu,
	} {
		engineOutput(t, writer, "\nPlan:\n", output)

		if input.Len() != 0 {
			t.Fatalf("plan triggered an answer: %q", output)
		}
	}
}

func TestPromptWriterMenuAllowsLineFormatting(t *testing.T) {
	writer, input, _, _ := promptEngine(t)
	engineOutput(t, writer, "  [1] Apply this fix to working copy\r\n"+
		"  [2] Refine the fix prompt\r\n  [3] Exit without applying\r\n")

	if input.String() != "1\n" {
		t.Fatalf("engine input = %q; want one apply answer", input.String())
	}
}

func TestPromptWriterClarificationAllowsOriginalFormattingWithoutPrompt(t *testing.T) {
	writer, input, record, run := promptEngine(t)
	engineOutput(t, writer, "[1/1] What is specified?\r\n    Options:\r\n"+
		"      1) Build from Specification\r\n      0) Other (type custom answer)\r\n")

	saved, err := drill.Read(record.Directory)
	must(t, err)

	if saved.Runs[run.Tag].Status != pausedStatus || input.Len() != 0 {
		t.Fatalf("did not pause: %+v, engine input %q", saved.Runs[run.Tag], input.String())
	}

	assertUnansweredQuestion(t, saved.Runs[run.Tag])
}

func TestPromptWriterClarificationOneByteAtATime(t *testing.T) {
	writer, input, record, run := promptEngine(t)

	output := originalClarification + "\n    Your answer: "
	engineOutput(t, writer, strings.Split(output, "")...)

	saved, err := drill.Read(record.Directory)
	must(t, err)
	assertUnansweredQuestion(t, saved.Runs[run.Tag])

	if saved.Runs[run.Tag].Status != pausedStatus || input.Len() != 0 {
		t.Fatalf("did not pause: %+v, engine input %q", saved.Runs[run.Tag], input.String())
	}

	assertOriginalClarification(t, saved.Runs[run.Tag].Questions[0])
}

func TestPromptWriterClarificationWaitsForCompleteOtherLine(t *testing.T) {
	for _, ending := range []string{"\n", "Your answer: "} {
		t.Run("ending="+ending, func(t *testing.T) {
			writer, input, record, run := promptEngine(t)

			question := strings.TrimSuffix(originalClarification, "\n")
			for _, chunk := range []string{strings.TrimSuffix(question, " (type custom answer)"), " (type custom answer)"} {
				_, err := io.WriteString(writer, chunk)
				must(t, err)

				saved, err := drill.Read(record.Directory)
				must(t, err)

				if len(saved.Runs[run.Tag].Questions) != 0 {
					t.Fatal("paused before the Other line was complete")
				}
			}

			engineOutput(t, writer, ending)

			saved, err := drill.Read(record.Directory)
			must(t, err)

			paused := saved.Runs[run.Tag]
			assertUnansweredQuestion(t, paused)

			if ending == "\n" {
				question = originalClarification
			}

			if paused.Status != pausedStatus || input.Len() != 0 || paused.Questions[0].Prompt != question {
				t.Fatalf("did not retain the complete clarification: %+v, engine input %q", paused, input.String())
			}
		})
	}
}

func TestPromptWriterClarificationAcrossEveryChunkBoundary(t *testing.T) {
	for _, prompt := range []string{"", "\n    Your answer: "} {
		t.Run("prompt="+prompt, func(t *testing.T) {
			writer, input, record, run := promptEngine(t)

			output := originalClarification + prompt
			for split := range len(output) + 1 {
				engineOutput(t, writer, "\n", output[:split], output[split:])

				saved, err := drill.Read(record.Directory)
				must(t, err)

				paused := saved.Runs[run.Tag]
				if paused.Status != pausedStatus || len(paused.Questions) != split+1 || input.Len() != 0 {
					t.Fatalf("split at %d: run %+v, engine input %q", split, paused, input.String())
				}

				assertOriginalClarification(t, paused.Questions[split])
			}
		})
	}
}

func assertOriginalClarification(t *testing.T, question drill.Question) {
	t.Helper()

	if question.Prompt != originalClarification {
		t.Fatalf("saved prompt = %q; want %q", question.Prompt, originalClarification)
	}

	if !slices.Equal(question.Options,
		[]string{"      1) Build from Specification", "      0) Other (type custom answer)"}) {
		t.Fatalf("saved options = %q; want the complete printed option lines", question.Options)
	}
}

func TestPromptWriterIgnoresExactPlanBeforeMenu(t *testing.T) {
	plan := "Serves: `TestScenCheck001/002/005`, `TestUsRefineVerdicts001…003`, `TestEngineRoutes003…" +
		"006`, `TestEngineModel001Dispatch`, and the validation halves of the `scen check` qualit" +
		"y Prompts.\n" +
		"\n" +
		"### Stage 5 — the fix loop and the apply plumbing\n" +
		"\n" +
		"The interactive loop: clarifying questions numbered from 1 plus `0) Other (type custom a" +
		"nswer)`, the `Your answer: ` prompt, free text retained after trimming; then `[1] Apply " +
		"this fix to working copy`, `[2] Refine the fix prompt (provide feedback)`, `[3] Exit wit" +
		"hout applying`, the `Your choice (1/2/3): ` prompt, one line read, end of input reading " +
		"as exit. Refinement reads feedback up to an empty line and stores it at `_user_refinemen" +
		"t`. Five clarification rounds at most. The walker's `max_apply_attempts` allowance, the " +
		"re-walk (`RE-WALK (fixes applied, verifying)`), `Hit max apply attempts without converge" +
		"nce.` and `Walk stopped … gave up after N applied fix(es)`. Scratch term/registry docume" +
		"nts seeded under the run directory and committed on convergence (`Fix recorded in the do" +
		"cument document=…`) or put back (`Fix undone in the document`). Fix and apply answer ext" +
		"raction: JSON when the provider is structured, else the FILE_START/FILE_END marker form " +
		"with a wrapping code fence stripped.\n" +
		"\n" +
		"Serves: `TestFixLoopBounds001…004`, `TestFixLoopChoices001…006`, `TestScenCheckFix001…00" +
		"7`, `TestScenCheckQualityFix001…007`, `TestUsRefineFix001…007`, `TestCodexApply001`.\n" +
		"\n" +
		"### Stage 6 — `us refine` and `us apply`\n" +
		"\n" +
		"Story discovery (`<id>-*.yaml` under `paths.stories_dir`, the declared `id` deciding amo" +
		"ng shared prefixes, refusing none/several/another with the pinned messages), the Story m" +
		"odel and its YAML round trip (field order `id, feature, title, as_a, i_want, so_that, st" +
		"atus, acceptance_criteria`; `feature` omitted when empty; four-space indentation; modifi" +
		"er steps as one-key `and`/`but` maps), criterion id validation `<NNN>-<slug>`, the legac" +
		"y `scenarios.test_scenarios[]` refusal, and `us apply`'s merge into the Registry keyed b" +
		"y the lineage id `<story-id>-<ac-id>` with `user_stories[]` lineage, duplicate collapse," +
		" and the `Scenario keyed apart from its lineage` refusal that leaves the Registry unchan" +
		"ged.\n" +
		"\n" +
		"Serves: `TestUsRefine*`, `TestUsApply*`, `TestEngine003LocationsComeFromConfig`.\n"
	writer, input, record, run := promptEngine(t)
	engineOutput(t, writer, plan, specificationMenu, "Your choice (1/2/3): ")

	saved, err := drill.Read(record.Directory)
	must(t, err)

	if len(saved.Runs[run.Tag].Questions) != 0 || input.String() != "1\n" {
		t.Fatalf("plan mistaken for input: questions=%+v, engine input=%q", saved.Runs[run.Tag].Questions, input.String())
	}
}

func TestPromptWriterIgnoresFormsFollowedByMoreOutputAcrossChunks(t *testing.T) {
	for name, form := range map[string]string{
		"clarification":        originalClarification,
		"legacy clarification": originalClarification + "Your answer: ",
		"menu":                 specificationMenu,
		"legacy menu":          specificationMenu + "Your choice (1/2/3): ",
	} {
		t.Run(name, func(t *testing.T) {
			writer, input, record, run := promptEngine(t)
			engineOutput(t, writer, "Example in a plan:\n", form, "The plan continues.\n")

			saved, err := drill.Read(record.Directory)
			must(t, err)

			if len(saved.Runs[run.Tag].Questions) != 0 || input.Len() != 0 {
				t.Fatalf("ongoing output mistaken for input: questions=%+v, input=%q",
					saved.Runs[run.Tag].Questions, input.String())
			}
		})
	}
}

func TestPromptWriterClarificationWithoutFinalNewline(t *testing.T) {
	writer, input, record, run := promptEngine(t)
	question := strings.TrimSuffix(originalClarification, "\n")
	engineOutput(t, writer, question)

	saved, err := drill.Read(record.Directory)
	must(t, err)
	assertUnansweredQuestion(t, saved.Runs[run.Tag])

	if saved.Runs[run.Tag].Status != pausedStatus || input.Len() != 0 ||
		saved.Runs[run.Tag].Questions[0].Prompt != question {
		t.Fatalf("quiet clarification not retained: %+v", saved.Runs[run.Tag])
	}
}
