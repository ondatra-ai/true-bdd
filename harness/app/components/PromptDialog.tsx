"use client";

import { useEffect, useRef, useState } from "react";

import type { PromptView } from "../lib/view-model/run";

/** The result of an answer RPC — a failure keeps the dialog open (plan §4). */
export interface AnswerResult {
  ok: boolean;
  error: string | null;
}

/**
 * The interactive prompt DIALOG (plan §4 — PROMPTS BECOME DIALOGS). A native
 * `<dialog>` opened with showModal() gives real modality (focus containment,
 * a backdrop) — the same pattern as the story modal. Unlike the story modal,
 * Escape does NOT answer/close and there is no backdrop-close: the dialog
 * stays until the prompt is answered or the run ends. A FAILED answer RPC
 * keeps the dialog OPEN with a visible `prompt-dialog-error`.
 *
 * Choice → apply/refine/exit buttons that answer "1"/"2"/"3"; clarify → a
 * single-line number input (+ numbered option chips); freetext → a multiline
 * textarea. The component is KEYED by prompt id upstream, so each new prompt
 * re-mounts a fresh dialog (its error/input state resets).
 */
export function PromptDialog({
  prompt,
  onAnswer,
}: {
  prompt: NonNullable<PromptView>;
  onAnswer: (promptId: string, value: string) => Promise<AnswerResult>;
}) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const titleId = "prompt-dialog-title-heading";

  // Show the dialog modally once mounted (native focus containment).
  useEffect(() => {
    const dialog = dialogRef.current;
    if (dialog !== null && !dialog.open) {
      dialog.showModal();
    }
  }, []);

  async function submit(value: string): Promise<void> {
    if (pending) {
      return;
    }
    setPending(true);
    setError(null);
    const result = await onAnswer(prompt.promptId, value);
    setPending(false);
    if (!result.ok) {
      // Keep the dialog OPEN with a visible error (plan §4).
      setError(result.error ?? "The answer was rejected.");
    }
  }

  return (
    <dialog
      ref={dialogRef}
      className="sf-modal"
      data-testid="prompt-dialog"
      data-kind={prompt.kind}
      data-prompt-id={prompt.promptId}
      aria-labelledby={titleId}
      // Escape must NOT answer/close — the dialog stays until the prompt is
      // answered or the run ends (plan §4 / P14).
      onCancel={(event) => event.preventDefault()}
    >
      <div className="sf-modal-panel" data-testid="prompt-dialog-panel">
        <h2 id={titleId} className="sf-modal-title" data-testid="prompt-dialog-title">
          {promptTitle(prompt)}
        </h2>

        {prompt.kind === "choice" ? <ChoiceBody onSubmit={submit} disabled={pending} /> : null}
        {prompt.kind === "clarify" ? <ClarifyBody prompt={prompt} onSubmit={submit} disabled={pending} /> : null}
        {prompt.kind === "freetext" ? <FreetextBody prompt={prompt} onSubmit={submit} disabled={pending} /> : null}

        {error !== null ? (
          <p className="sf-modal-error" data-testid="prompt-dialog-error" role="alert">
            {error}
          </p>
        ) : null}
      </div>
    </dialog>
  );
}

function promptTitle(prompt: NonNullable<PromptView>): string {
  switch (prompt.kind) {
    case "choice":
      return "Choose a fix action";
    case "clarify":
      return prompt.question || "Answer the clarifying question";
    case "freetext":
      return "Refinement feedback";
    default:
      return "Prompt";
  }
}

/** apply=1, refine=2, exit=3 (the collector reads the option number). */
const CHOICE_ANSWER: Record<"apply" | "refine" | "exit", string> = {
  apply: "1",
  refine: "2",
  exit: "3",
};

const CHOICE_TESTID: Record<"apply" | "refine" | "exit", string> = {
  apply: "prompt-choice-apply",
  refine: "prompt-choice-refine",
  exit: "prompt-choice-exit",
};

function ChoiceBody({ onSubmit, disabled }: { onSubmit: (value: string) => void; disabled: boolean }) {
  const actions = ["apply", "refine", "exit"] as const;

  return (
    <div className="sf-actions" style={{ marginTop: "0.5rem" }}>
      {actions.map((action) => (
        <button
          key={action}
          type="button"
          className="sf-btn"
          data-testid={CHOICE_TESTID[action]}
          disabled={disabled}
          onClick={() => onSubmit(CHOICE_ANSWER[action])}
        >
          {action[0].toUpperCase() + action.slice(1)}
        </button>
      ))}
    </div>
  );
}

function ClarifyBody({
  prompt,
  onSubmit,
  disabled,
}: {
  prompt: Extract<PromptView, { kind: "clarify" }>;
  onSubmit: (value: string) => void;
  disabled: boolean;
}) {
  const [value, setValue] = useState("");

  return (
    <div>
      {prompt.options.length > 0 ? (
        <ol style={{ margin: "0.25rem 0 0.75rem", paddingLeft: "1.25rem" }}>
          {prompt.options.map((option, index) => (
            <li key={`${index}-${option}`} data-testid="prompt-clarify-option" data-index={index + 1}>
              {option}
            </li>
          ))}
        </ol>
      ) : null}
      <div className="sf-actions">
        <input
          data-testid="prompt-answer-input"
          className="sf-input"
          value={value}
          disabled={disabled}
          placeholder="option number"
          onChange={(event) => setValue(event.target.value)}
        />
        <button
          type="button"
          className="sf-btn"
          data-testid="prompt-answer-submit"
          disabled={disabled}
          onClick={() => onSubmit(value)}
        >
          Submit
        </button>
      </div>
    </div>
  );
}

function FreetextBody({
  prompt,
  onSubmit,
  disabled,
}: {
  prompt: Extract<PromptView, { kind: "freetext" }>;
  onSubmit: (value: string) => void;
  disabled: boolean;
}) {
  const [value, setValue] = useState("");

  return (
    <div>
      {prompt.prompt ? <p className="sf-muted" style={{ margin: "0.25rem 0" }}>{prompt.prompt}</p> : null}
      <textarea
        data-testid="prompt-freetext-input"
        className="sf-textarea"
        rows={5}
        value={value}
        disabled={disabled}
        onChange={(event) => setValue(event.target.value)}
      />
      <div style={{ marginTop: "0.5rem" }}>
        <button
          type="button"
          className="sf-btn"
          data-testid="prompt-freetext-submit"
          disabled={disabled}
          onClick={() => onSubmit(value)}
        >
          Submit feedback
        </button>
      </div>
    </div>
  );
}
