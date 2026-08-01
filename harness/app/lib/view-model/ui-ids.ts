/**
 * Dynamic data-testid builders shared by the views. These MUST stay in
 * lock-step with tests/harness/helpers/ui.ts (the binding selector contract):
 * the harness app cannot import the test helper, so the two derive the
 * identical strings from the same rule.
 */

/** Inventory chip for a document/directory/checklist key. */
export function inventoryDocTestId(key: string): string {
  return `inventory-doc-${key}`;
}

/** Story row, keyed by the position-derived create id `<epic>.<pos>`. */
export function storyRowTestId(createId: string): string {
  return `story-row-${createId}`;
}
