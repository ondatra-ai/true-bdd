"use client";

import { useEffect, useRef } from "react";

import { statusTone, storyModalModel } from "../lib/view-model/inventory";
import type { InventoryEpic, InventoryStory } from "../lib/view-model/types";

/**
 * The story review popup (plan §3, README-testids "Story review modal"). A
 * native <dialog> opened with showModal() gives real modality — focus
 * containment, Escape-to-close, a backdrop, and automatic focus restoration
 * to the opener — instead of a hand-written trap. A click on the backdrop
 * field (the dialog area around the panel) closes it; a click on the panel
 * does not. Tabs are a real tablist/tab/tabpanel with keyboard navigation.
 */
export function StoryModal({
  story,
  epic,
  activeTab,
  onTab,
  onClose,
  changedOnDisk = false,
}: {
  story: InventoryStory;
  epic: InventoryEpic;
  activeTab: "review" | "raw";
  onTab: (tab: "review" | "raw") => void;
  onClose: () => void;
  /** True when the story vanished from a fresher snapshot (finding 4). */
  changedOnDisk?: boolean;
}) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const reviewTabRef = useRef<HTMLButtonElement>(null);
  const rawTabRef = useRef<HTMLButtonElement>(null);
  const model = storyModalModel(story, epic, { changedOnDisk });
  const titleId = "story-modal-title-heading";

  // Show the dialog modally once it is mounted (native focus containment +
  // focus restoration on close).
  useEffect(() => {
    const dialog = dialogRef.current;
    if (dialog !== null && !dialog.open) {
      dialog.showModal();
    }
  }, []);

  // The Raw tab is unavailable when there is no retained raw (missing file /
  // omitted). Keep the effective tab within what exists.
  const rawAvailable = model.raw.available;
  const reviewEnabled = model.review.enabled;
  const effectiveTab: "review" | "raw" = pickTab(activeTab, reviewEnabled, rawAvailable);

  function tabKeyDown(event: React.KeyboardEvent): void {
    if (event.key !== "ArrowRight" && event.key !== "ArrowLeft") {
      return;
    }
    event.preventDefault();
    const next = effectiveTab === "review" ? "raw" : "review";
    if (next === "raw" && !rawAvailable) {
      return;
    }
    if (next === "review" && !reviewEnabled) {
      return;
    }
    onTab(next);
    // Roving focus: move focus to the newly-selected tab (finding 5b), so
    // arrow-key navigation matches a real tablist rather than only shifting
    // aria-selected while focus stays on the origin tab.
    const destination = next === "raw" ? rawTabRef : reviewTabRef;
    queueMicrotask(() => destination.current?.focus());
  }

  return (
    <dialog
      ref={dialogRef}
      className="sf-modal"
      data-testid="story-modal"
      data-story-id={story.create_id}
      aria-labelledby={titleId}
      onClose={onClose}
      onCancel={onClose}
      onClick={(event) => {
        if (event.target === dialogRef.current) {
          dialogRef.current?.close();
        }
      }}
    >
      <div className="sf-modal-panel" data-testid="story-modal-panel">
        <div className="sf-modal-head">
          <h2 id={titleId} className="sf-modal-title" data-testid="story-modal-title">
            {model.storyId}
            {model.title ? ` — ${model.title}` : ""}
          </h2>
          <button
            type="button"
            className="sf-modal-close"
            data-testid="story-modal-close"
            aria-label="Close story review"
            onClick={() => dialogRef.current?.close()}
          >
            ×
          </button>
        </div>

        <div className="sf-modal-meta">
          <span className="sf-chip" data-tone={statusTone(model.status)} data-testid="story-modal-status">
            <span className="sq" />
            {model.status}
          </span>
          <span className="sf-chip" data-tone={statusTone(model.chips.created)} data-testid="story-modal-created">
            <span className="sq" />
            created {model.chips.created}
          </span>
          <span className="sf-chip" data-tone={statusTone(model.chips.applied.status)} data-testid="story-modal-applied">
            <span className="sq" />
            applied {model.chips.applied.text}
          </span>
          <span className="sf-chip" data-tone="neutral" data-testid="story-modal-refined">
            <span className="sq" />
            refined {model.chips.refined}
          </span>
        </div>

        <div className="sf-modal-identity" data-testid="story-modal-identity">
          declared {story.declared_id || "—"} · file {story.file_id || "—"}
        </div>

        {model.errorMode ? (
          <div className="sf-modal-error" data-testid="story-modal-error">
            {story.error || "This story file could not be parsed."}
          </div>
        ) : null}

        {model.notices.map((notice) => (
          <div
            key={notice.reason}
            className="sf-modal-notice"
            data-testid="story-modal-notice"
            data-reason={notice.reason}
          >
            {noticeText(notice.reason, story.match_count)}
          </div>
        ))}

        <div className="sf-tablist" role="tablist" aria-label="Story views" data-testid="story-modal-tablist">
          <button
            ref={reviewTabRef}
            type="button"
            role="tab"
            id="story-modal-tab-review"
            className="sf-tab"
            data-testid="story-modal-tab-review"
            aria-selected={effectiveTab === "review"}
            aria-controls="story-modal-panel-review"
            aria-disabled={!reviewEnabled}
            tabIndex={effectiveTab === "review" ? 0 : -1}
            onClick={() => reviewEnabled && onTab("review")}
            onKeyDown={tabKeyDown}
          >
            Review
          </button>
          {rawAvailable ? (
            <button
              ref={rawTabRef}
              type="button"
              role="tab"
              id="story-modal-tab-raw"
              className="sf-tab"
              data-testid="story-modal-tab-raw"
              aria-selected={effectiveTab === "raw"}
              aria-controls="story-modal-panel-raw"
              tabIndex={effectiveTab === "raw" ? 0 : -1}
              onClick={() => onTab("raw")}
              onKeyDown={tabKeyDown}
            >
              Raw
            </button>
          ) : null}
        </div>

        {effectiveTab === "review" ? (
          <div
            className="sf-tabpanel"
            role="tabpanel"
            id="story-modal-panel-review"
            aria-labelledby="story-modal-tab-review"
            data-testid="story-modal-panel-review"
          >
            {model.review.statement ? (
              <div className="sf-statement" data-testid="story-modal-statement">
                <div>
                  <span>As a</span>
                  {model.review.statement.asA}
                </div>
                <div>
                  <span>I want</span>
                  {model.review.statement.iWant}
                </div>
                <div>
                  <span>So that</span>
                  {model.review.statement.soThat}
                </div>
              </div>
            ) : (
              <p className="sf-empty">No reviewable content is available for this story.</p>
            )}

            {model.review.acceptanceCriteria.map((ac) => (
              <div key={ac.id} className="sf-ac" data-testid="story-modal-ac" data-ac-id={ac.id}>
                <div className="sf-ac-id">{ac.id}</div>
                <div className="sf-ac-desc">{ac.description}</div>
                <ul className="sf-steps">
                  {ac.steps.map((step, index) => (
                    <li key={`${ac.id}-${index}`} className="sf-step">
                      <span className="kind" aria-hidden="true">
                        {step.kind}
                      </span>
                      <span data-testid="story-modal-step" data-kind={step.kind}>
                        {step.text}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        ) : (
          <div
            className="sf-tabpanel"
            role="tabpanel"
            id="story-modal-panel-raw"
            aria-labelledby="story-modal-tab-raw"
            data-testid="story-modal-panel-raw"
          >
            <pre className="sf-raw" data-testid="story-modal-raw">{model.raw.text}</pre>
          </div>
        )}
      </div>
    </dialog>
  );
}

/** The effective tab, falling back to whichever panel actually exists. */
function pickTab(requested: "review" | "raw", reviewEnabled: boolean, rawAvailable: boolean): "review" | "raw" {
  if (requested === "raw") {
    return rawAvailable ? "raw" : "review";
  }

  return reviewEnabled || !rawAvailable ? "review" : "raw";
}

/** Human text for an availability / omission / freshness notice. */
function noticeText(reason: string, matchCount?: number): string {
  switch (reason) {
    case "not_created":
      return "This story has not been created yet — showing the epic-declared content.";
    case "ambiguous":
      return `${matchCount ?? "Multiple"} files match this story id — showing the epic-declared content. Resolve to a single file to open it.`;
    case "changed_on_disk":
      return "This story changed on disk since the modal opened.";
    case "raw_truncated":
      return "The raw file was truncated to fit the inventory budget.";
    case "raw_omitted":
      return "The raw file was omitted to fit the inventory budget.";
    case "content_omitted":
      return "The parsed content was omitted to fit the inventory budget.";
    case "both_omitted":
      return "The raw file and parsed content were omitted to fit the inventory budget.";
    case "invalid_without_raw":
      return "This story is invalid and its raw file is unavailable.";
    default:
      return reason;
  }
}
