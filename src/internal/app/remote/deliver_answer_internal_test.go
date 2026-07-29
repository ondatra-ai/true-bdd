package remote

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/events"
)

// newTestExecutor builds a RunExecutor wired to an in-memory outbox and an
// OS pipe standing in for the child's stdin, for exercising DeliverAnswer.
func newTestExecutor(t *testing.T, promptKind string) (*RunExecutor, *os.File) {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	t.Cleanup(func() { _ = reader.Close() })

	executor := &RunExecutor{
		run:            RunSpec{RunID: testRunID},
		sender:         newOutbox(newStubEvents(), testSessionID, testRunID, t.TempDir(), fastOpts()),
		stdin:          writer,
		lastPromptKind: promptKind,
	}

	return executor, reader
}

// TestDeliverAnswerMemoizeBeforeStdin proves the plan §3.3 guarantee: the
// same answer_seq redelivered (a lost-ack retry) writes to the child's
// stdin exactly once, and a distinct higher answer_seq writes again.
func TestDeliverAnswerMemoizeBeforeStdin(t *testing.T) {
	t.Parallel()

	executor, reader := newTestExecutor(t, string(events.KindChoice))

	executor.DeliverAnswer(AnswerMsg{PromptID: "p1", Value: answerExit, AnswerSeq: 1})
	// Redelivery of the same answer_seq must not write to stdin twice.
	executor.DeliverAnswer(AnswerMsg{PromptID: "p1", Value: answerExit, AnswerSeq: 1})
	executor.DeliverAnswer(AnswerMsg{PromptID: "p2", Value: answerApply, AnswerSeq: 2})

	// A post-answer event flushes the memoized consumed watermark to the
	// server, which confirms it back (answers_consumed_through round-trip).
	err := executor.sender.Send(context.Background(), OutEvent{Type: eventTypeOutput, Data: "post"})
	if err != nil {
		t.Fatalf("flush after answers: %v", err)
	}

	_ = executor.stdin.Close()

	written, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read child stdin: %v", err)
	}

	if string(written) != "exit\napply\n" {
		t.Fatalf("child stdin = %q, want %q", written, "exit\napply\n")
	}

	if executor.sender.AnswersConsumedThrough() != 2 {
		t.Fatalf("answers_consumed_through = %d, want 2", executor.sender.AnswersConsumedThrough())
	}
}

// TestDeliverAnswerRejectsInvalid proves remote-side hygiene: a malformed
// answer is not written and does not advance the consumed watermark, so a
// later valid answer still lands.
func TestDeliverAnswerRejectsInvalid(t *testing.T) {
	t.Parallel()

	executor, reader := newTestExecutor(t, string(events.KindChoice))

	executor.DeliverAnswer(AnswerMsg{PromptID: "p1", Value: "delete-everything", AnswerSeq: 1}) // off-enum
	executor.DeliverAnswer(AnswerMsg{PromptID: "p2", Value: answerExit, AnswerSeq: 2})

	_ = executor.stdin.Close()

	written, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read child stdin: %v", err)
	}

	if string(written) != "exit\n" {
		t.Fatalf("child stdin = %q, want %q (invalid answer must be dropped)", written, "exit\n")
	}
}
