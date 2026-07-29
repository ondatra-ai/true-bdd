"use client";

import { useState } from "react";

import type { PromptView } from "../lib/view-model/run";
import { button, MONO } from "./styles";

/**
 * The interactive prompt panel (plan §3.5, §4.3 A0–A4). One panel per
 * pending prompt, scoped by data-kind + data-prompt-id (the AI specs bind
 * their clicks to a specific prompt id). Choice → apply/refine/exit
 * buttons; clarify → numbered options + a single-line number answer;
 * freetext → a MULTILINE textarea (distinct from clarify so the newline
 * transport is exercised). Selector contract: README-testids.
 */
export function PromptPanel({
  prompt,
  onAnswer,
}: {
  prompt: PromptView;
  onAnswer: (promptId: string, value: string) => void | Promise<void>;
}) {
  if (prompt === null) {
    return null;
  }

  return (
    <div
      data-testid="prompt-panel"
      data-kind={prompt.kind}
      data-prompt-id={prompt.promptId}
      style={{ border: "2px solid #2d6cdf", borderRadius: 6, padding: "0.75rem", marginTop: "1rem", background: "#f4f8ff" }}
    >
      {prompt.kind === "choice" ? <ChoicePrompt prompt={prompt} onAnswer={onAnswer} /> : null}
      {prompt.kind === "clarify" ? <ClarifyPrompt prompt={prompt} onAnswer={onAnswer} /> : null}
      {prompt.kind === "freetext" ? <FreetextPrompt prompt={prompt} onAnswer={onAnswer} /> : null}
    </div>
  );
}

const CHOICE_TESTID: Record<string, string> = {
  apply: "prompt-choice-apply",
  refine: "prompt-choice-refine",
  exit: "prompt-choice-exit",
};

function ChoicePrompt({
  prompt,
  onAnswer,
}: {
  prompt: Extract<PromptView, { kind: "choice" }>;
  onAnswer: (promptId: string, value: string) => void | Promise<void>;
}) {
  // Always render the three canonical actions so the testids are stable
  // even if the payload's action list is ordered differently.
  const actions = ["apply", "refine", "exit"] as const;

  return (
    <div>
      <p style={{ margin: "0 0 0.5rem" }}>Choose a fix action:</p>
      <div style={{ display: "flex", gap: 8 }}>
        {actions.map((action) => (
          <button
            key={action}
            type="button"
            data-testid={CHOICE_TESTID[action]}
            onClick={() => void onAnswer(prompt.promptId, action)}
            style={button}
            disabled={!prompt.actions.includes(action)}
          >
            {action[0].toUpperCase() + action.slice(1)}
          </button>
        ))}
      </div>
    </div>
  );
}

function ClarifyPrompt({
  prompt,
  onAnswer,
}: {
  prompt: Extract<PromptView, { kind: "clarify" }>;
  onAnswer: (promptId: string, value: string) => void | Promise<void>;
}) {
  const [value, setValue] = useState("");

  return (
    <div>
      {prompt.question ? <p style={{ margin: "0 0 0.5rem" }}>{prompt.question}</p> : null}
      <ol style={{ margin: "0 0 0.5rem", paddingLeft: "1.25rem" }}>
        {prompt.options.map((option, index) => (
          <li key={`${index}-${option}`} data-testid="prompt-clarify-option" data-index={index + 1}>
            {option}
          </li>
        ))}
      </ol>
      <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
        <input
          data-testid="prompt-answer-input"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          placeholder="option number"
          style={{ fontFamily: MONO, fontSize: 12, width: 120, padding: "3px 6px" }}
        />
        <button type="button" data-testid="prompt-answer-submit" onClick={() => void onAnswer(prompt.promptId, value)} style={button}>
          Submit
        </button>
      </div>
    </div>
  );
}

function FreetextPrompt({
  prompt,
  onAnswer,
}: {
  prompt: Extract<PromptView, { kind: "freetext" }>;
  onAnswer: (promptId: string, value: string) => void | Promise<void>;
}) {
  const [value, setValue] = useState("");

  return (
    <div>
      {prompt.prompt ? <p style={{ margin: "0 0 0.5rem", color: "#555" }}>{prompt.prompt}</p> : null}
      <textarea
        data-testid="prompt-freetext-input"
        value={value}
        onChange={(event) => setValue(event.target.value)}
        rows={5}
        style={{ fontFamily: MONO, fontSize: 12, width: "100%", padding: "6px", boxSizing: "border-box" }}
      />
      <div style={{ marginTop: 6 }}>
        <button type="button" data-testid="prompt-freetext-submit" onClick={() => void onAnswer(prompt.promptId, value)} style={button}>
          Submit feedback
        </button>
      </div>
    </div>
  );
}
