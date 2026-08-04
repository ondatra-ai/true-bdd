/**
 * YAML parsing helpers, client-side (plan: "the FilesProvider ... derives
 * outline + validity + line positions client-side from the buffer using a
 * real YAML parser"). Uses the `yaml` npm package's line-counter for
 * exact-line jumps (never the prototype's line-heuristic regex parsing).
 */

import { isMap, isSeq, LineCounter, parseDocument, parse as yamlParse } from "yaml";

/** Parses YAML text, returning null on any parse error (never throws). */
export function safeParseYAML<T = unknown>(text: string): T | null {
  try {
    return yamlParse(text) as T;
  } catch {
    return null;
  }
}

/** Whether `text` parses as YAML (the P10a/P10b client-side gate). */
export function isValidYAML(text: string): boolean {
  try {
    yamlParse(text);

    return true;
  } catch {
    return false;
  }
}

interface DocWithErrors {
  contents: unknown;
  errors: unknown[];
}

function parseWithLines(text: string, lineCounter: LineCounter): DocWithErrors | null {
  try {
    const doc = parseDocument(text, { keepSourceTokens: true, lineCounter }) as unknown as DocWithErrors;

    return doc.errors.length > 0 ? null : doc;
  } catch {
    return null;
  }
}

interface YamlPairLike {
  key?: { value?: unknown; range?: [number, number, number] } | unknown;
  value?: unknown;
}

function keyString(pair: YamlPairLike): string {
  const key = pair.key as { value?: unknown } | undefined;

  return String(key?.value ?? key ?? "");
}

/**
 * The 0-based buffer line of a nested map KEY (e.g. `["services",
 * "postgres"]` → the line `postgres:` appears on). Returns null when any
 * segment is missing or the document does not parse (P10b: callers fall back
 * to the last-valid content).
 */
export function lineOfMapKey(text: string, path: string[]): number | null {
  if (path.length === 0) {
    return null;
  }

  const lineCounter = new LineCounter();
  const doc = parseWithLines(text, lineCounter);
  if (doc === null) {
    return null;
  }

  let node = doc.contents as { items?: YamlPairLike[] } | null;

  for (let i = 0; i < path.length - 1; i++) {
    if (node === null || !isMap(node)) {
      return null;
    }
    const pair = node.items?.find((p) => keyString(p) === path[i]);
    if (pair === undefined) {
      return null;
    }
    node = pair.value as { items?: YamlPairLike[] } | null;
  }

  if (node === null || !isMap(node)) {
    return null;
  }

  const lastKey = path[path.length - 1];
  const pair = node.items?.find((p) => keyString(p) === lastKey);
  const keyRange = (pair?.key as { range?: [number, number, number] } | undefined)?.range;
  if (keyRange === undefined) {
    return null;
  }

  return lineCounter.linePos(keyRange[0]).line - 1;
}

/**
 * The 0-based buffer line of a sequence-item map matching `field === value`
 * under `seqPath` (e.g. terms: `[{name: "Vocabulary", ...}, ...]`).
 */
export function lineOfSeqItemByField(text: string, seqPath: string[], field: string, value: string): number | null {
  const lineCounter = new LineCounter();
  const doc = parseWithLines(text, lineCounter);
  if (doc === null) {
    return null;
  }

  let node = doc.contents as { items?: YamlPairLike[] } | null;

  for (const seg of seqPath) {
    if (node === null || !isMap(node)) {
      return null;
    }
    const pair = node.items?.find((p) => keyString(p) === seg);
    if (pair === undefined) {
      return null;
    }
    node = pair.value as { items?: YamlPairLike[] } | null;
  }

  if (node === null || !isSeq(node)) {
    return null;
  }

  const items = (node as unknown as { items: unknown[] }).items;
  for (const item of items) {
    if (!isMap(item)) {
      continue;
    }
    const fieldPair = (item.items as YamlPairLike[]).find((p) => keyString(p) === field);
    const fieldValue = (fieldPair?.value as { value?: unknown } | undefined)?.value;
    if (fieldValue !== undefined && String(fieldValue) === value) {
      const range = (item as unknown as { range?: [number, number, number] }).range;
      if (range === undefined) {
        return null;
      }

      return lineCounter.linePos(range[0]).line - 1;
    }
  }

  return null;
}
