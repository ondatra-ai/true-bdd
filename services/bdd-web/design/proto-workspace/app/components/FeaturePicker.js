"use client";

import { useState } from "react";

// ONE shared compact "feature" combobox, reused everywhere a story or
// scenario's `feature:` tag can be assigned/reassigned: the new-story form,
// the slim Feature strip on a story page, and each row of a
// /feature/<slug> aggregation page. Collapsed = a small pill showing the
// current value (click to edit); expanded = a searchable dropdown over the
// feature catalog (features.yaml only has id/description, so the id IS the
// searchable label) with a trailing `+ Create "<query>"` option whenever
// the typed text has no exact id match — reuses the tech-picker's visual
// language (public/proto-extra.css) rather than inventing new chrome.
export default function FeaturePicker({ features, value, onSelect, onCreate, label = "Feature", testId = "feature-picker" }) {
  const [editing, setEditing] = useState(false);
  const [query, setQuery] = useState("");

  const q = query.trim().toLowerCase();
  const matches = features.filter((f) => f.id.toLowerCase().includes(q));
  const exact = features.some((f) => f.id.toLowerCase() === q);
  const showCreate = q.length > 0 && !exact;

  function pick(id) {
    setEditing(false);
    setQuery("");
    onSelect(id);
  }

  function create() {
    const name = query.trim();
    setEditing(false);
    setQuery("");
    onCreate(name);
  }

  if (!editing) {
    return (
      <button
        type="button"
        className="feature-pill"
        onClick={() => setEditing(true)}
        data-testid={`${testId}-toggle`}
      >
        <span className="feature-pill__label">{label}:</span>{" "}
        <span className="feature-pill__value">{value || "(none)"}</span>
        <span className="feature-pill__change">change</span>
      </button>
    );
  }

  return (
    <div className="tech-picker feature-picker" data-testid={testId}>
      <input
        type="text"
        autoFocus
        placeholder="Search or create a feature…"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onBlur={() => setTimeout(() => setEditing(false), 150)}
        className="tech-picker__input"
        aria-label={`${label} — search or create`}
        data-testid={`${testId}-input`}
      />
      <ul className="tech-picker__list" role="listbox">
        {matches.map((f) => (
          <li
            key={f.id}
            role="option"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => pick(f.id)}
            data-testid={`${testId}-option`}
          >
            {f.id}
          </li>
        ))}
        {showCreate && (
          <li
            className="tech-picker__create"
            onMouseDown={(e) => e.preventDefault()}
            onClick={create}
            data-testid={`${testId}-create-option`}
          >
            + Create &quot;{query.trim()}&quot;
          </li>
        )}
        {matches.length === 0 && !showCreate && (
          <li className="tech-picker__empty" aria-disabled="true">
            No match
          </li>
        )}
      </ul>
    </div>
  );
}
