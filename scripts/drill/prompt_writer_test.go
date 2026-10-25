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

	output := "[1/1] What is specified?\n    Options:\n" +
		"      1) Build from Specification\n      0) Other (type custom answer)\n\n    Your answer: "
	for _, value := range []byte(output) {
		engineOutput(t, writer, string(value))
	}

	saved, err := drill.Read(record.Directory)
	must(t, err)
	assertUnansweredQuestion(t, saved.Runs[run.Tag])

	if saved.Runs[run.Tag].Status != pausedStatus || input.Len() != 0 {
		t.Fatalf("did not pause: %+v, engine input %q", saved.Runs[run.Tag], input.String())
	}
}

func TestPromptWriterClarificationAcrossEveryChunkBoundary(t *testing.T) {
	writer, input, record, run := promptEngine(t)

	output := "[1/1] What is specified?\n1) Follow the Specification\n0) Other\nYour answer: "
	for split := range len(output) + 1 {
		engineOutput(t, writer, "\n", output[:split], output[split:])

		saved, err := drill.Read(record.Directory)
		must(t, err)

		paused := saved.Runs[run.Tag]
		if paused.Status != pausedStatus || len(paused.Questions) != split+1 || input.Len() != 0 {
			t.Fatalf("split at %d: run %+v, engine input %q", split, paused, input.String())
		}
	}
}
