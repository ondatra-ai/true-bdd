"use client";

import { inventoryDocTestId } from "../lib/view-model/ui-ids";
import type { DocumentChip } from "../lib/view-model/inventory";
import { chip as chipStyle, statusColor } from "./styles";

/**
 * Document/checklist inventory chips. Each chip carries its
 * `inventory-doc-<key>` testid and a `data-status` attribute — the
 * protocol contract (README-testids); the status text and any parse error
 * are shown for the human.
 */
export function InventoryChips({ chips }: { chips: DocumentChip[] }) {
  return (
    <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
      {chips.map((chip) => (
        <span
          key={chip.key}
          data-testid={inventoryDocTestId(chip.key)}
          data-status={chip.status}
          title={chip.error}
          style={{ ...chipStyle, borderColor: statusColor(chip.status) }}
        >
          <strong style={{ fontWeight: 600 }}>{chip.key}</strong>
          <span style={{ color: statusColor(chip.status) }}>{chip.status}</span>
        </span>
      ))}
    </div>
  );
}
