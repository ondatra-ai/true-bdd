"use client";

import { inventoryDocTestId } from "../lib/view-model/ui-ids";
import { statusTone, type DocumentChip } from "../lib/view-model/inventory";

/**
 * Document/checklist inventory chips. Each chip carries its
 * `inventory-doc-<key>` testid and a `data-status` attribute — the
 * protocol contract (README-testids); the colour lives only in the chip's
 * status square (S&F: colour in chips only).
 */
export function InventoryChips({ chips }: { chips: DocumentChip[] }) {
  return (
    <div className="sf-chips">
      {chips.map((chip) => (
        <span
          key={chip.key}
          className="sf-chip"
          data-testid={inventoryDocTestId(chip.key)}
          data-status={chip.status}
          data-tone={statusTone(chip.status)}
          title={chip.error}
        >
          <span className="sq" />
          <span className="sf-key">{chip.key}</span>
          <span>{chip.status}</span>
        </span>
      ))}
    </div>
  );
}
