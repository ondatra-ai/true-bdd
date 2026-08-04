"use client";

/**
 * Searchable feature picker (P21/P22/P23) — reused by the new-story form, a
 * story/scenario row's "change feature" control, and the unaligned bucket's
 * retro-tag control. Disambiguated by SCOPING to its container (each mount
 * is an independent instance; no shared ids). The option list FILTERS as you
 * type (a static `<select>` would fail — a searchable list is REQUIRED).
 */

import { useState } from "react";

import type { FeatureInfo } from "@/app/lib/workspace/derive";
import { slugify } from "@/app/lib/workspace/derive";

export function FeaturePicker({
  features,
  value,
  onPick,
  onCreate,
}: {
  features: FeatureInfo[];
  value?: string;
  onPick: (id: string) => void;
  onCreate: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  const filtered = features.filter((feature) => feature.id.toLowerCase().includes(query.trim().toLowerCase()));

  return (
    <div data-testid="feature-picker" className="ws-feature-picker">
      <button type="button" data-testid="feature-picker-toggle" onClick={() => setOpen((o) => !o)}>
        {value ?? "Select feature…"}
      </button>
      {open && (
        <div className="ws-picker-dropdown">
          <input
            data-testid="feature-picker-input"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search features"
          />
          {filtered.map((feature) => (
            <button
              key={feature.id}
              type="button"
              data-testid="feature-picker-option"
              data-feature={feature.id}
              className="ws-picker-option"
              onClick={() => {
                onPick(feature.id);
                setOpen(false);
                setQuery("");
              }}
            >
              {feature.id}
            </button>
          ))}
          {query.trim().length > 0 && (
            <button
              type="button"
              data-testid="feature-picker-create"
              className="ws-picker-option"
              onClick={() => {
                onCreate(slugify(query));
                setOpen(false);
                setQuery("");
              }}
            >
              + Create {query.trim()}
            </button>
          )}
        </div>
      )}
    </div>
  );
}
