/**
 * Fixture oracles for the story review modal (P9, plan §4.3). The modal's
 * expected statement / AC / step / raw values are READ FROM THE
 * MATERIALIZED FIXTURE FILES rather than duplicated as literals in the
 * spec — so a fixture edit can never silently drift from its assertions.
 *
 * These extractors are deliberately dependency-free (no YAML runtime): the
 * harness ships no YAML library, and the fixture shapes are the small,
 * stable, flow-free 2-space YAML this repo authors. The strongest oracle —
 * the Raw panel — is a byte-exact `fs.readFileSync` comparison and needs no
 * parsing at all; the Review-panel extractors below pull the specific
 * scalar fields and Gherkin steps the modal renders.
 */

import fs from "node:fs";
import path from "node:path";

/** Reads a fixture-relative file from a materialized host folder, verbatim. */
export function readFixtureFile(target: string, relPath: string): string {
  return fs.readFileSync(path.join(target, relPath), "utf8");
}

/** Strips a single matching pair of surrounding single/double quotes. */
function unquote(value: string): string {
  const trimmed = value.trim();
  if (
    trimmed.length >= 2 &&
    ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'")))
  ) {
    return trimmed.slice(1, -1);
  }

  return trimmed;
}

/**
 * The slice of an epic document covering one declared story block —
 * from its `- id: "<declaredId>"` line up to the next story declaration
 * (a `- id:` whose value starts with a digit) or EOF. Lets the missing /
 * ambiguous / malformed cases read the EPIC-DECLARED content of a specific
 * story rather than the first block in the file.
 */
export function storyBlockById(raw: string, declaredId: string): string {
  const lines = raw.split("\n");
  const startRe = new RegExp(`^\\s*-\\s+id:\\s*["']?${declaredId.replace(".", "\\.")}["']?\\s*$`);
  const nextStoryRe = /^\s*-\s+id:\s*["']?\d/;

  const start = lines.findIndex((line) => startRe.test(line));
  if (start === -1) {
    throw new Error(`declared story block ${declaredId} not found`);
  }

  let end = lines.length;
  for (let index = start + 1; index < lines.length; index += 1) {
    if (nextStoryRe.test(lines[index])) {
      end = index;
      break;
    }
  }

  return lines.slice(start, end).join("\n");
}

/** The single-line scalar value of `name:` (first occurrence), unquoted. */
export function scalarField(raw: string, name: string): string {
  const match = new RegExp(`^\\s*${name}:\\s*(.+?)\\s*$`, "m").exec(raw);
  if (match === null) {
    throw new Error(`fixture field ${name}: not found`);
  }

  return unquote(match[1]);
}

export interface StatementFixture {
  asA: string;
  iWant: string;
  soThat: string;
}

/** The as-a / i-want / so-that statement of the first story block in `raw`. */
export function statementOf(raw: string): StatementFixture {
  return {
    asA: scalarField(raw, "as_a"),
    iWant: scalarField(raw, "i_want"),
    soThat: scalarField(raw, "so_that"),
  };
}

/** Every acceptance-criterion `description:` value, in source order. */
export function acDescriptions(raw: string): string[] {
  const out: string[] = [];
  const re = /^\s*description:\s*(.+?)\s*$/gm;
  let match: RegExpExecArray | null = re.exec(raw);
  while (match !== null) {
    out.push(unquote(match[1]));
    match = re.exec(raw);
  }

  return out;
}

export interface StepFixture {
  kind: "given" | "when" | "then" | "and" | "but";
  text: string;
}

/**
 * The flattened Gherkin steps of the story block, in source order — the
 * same flattening the modal renders: each `given:`/`when:`/`then:` leaf
 * carries the block's kind, and an `and:`/`but:` modifier carries its own
 * kind. Reset at each new acceptance-criterion (`- id:`).
 */
export function steps(raw: string): StepFixture[] {
  const out: StepFixture[] = [];
  let currentKind: StepFixture["kind"] | null = null;

  for (const line of raw.split("\n")) {
    const modifier = /^\s*-\s+(and|but):\s*(.+?)\s*$/.exec(line);
    if (modifier !== null) {
      out.push({ kind: modifier[1] as StepFixture["kind"], text: unquote(modifier[2]) });
      continue;
    }

    const opener = /^\s*-?\s*(given|when|then):\s*$/.exec(line);
    if (opener !== null) {
      currentKind = opener[1] as StepFixture["kind"];
      continue;
    }

    // A new acceptance criterion ends the current step block.
    if (/^\s*-\s+id:\s*/.test(line)) {
      currentKind = null;
      continue;
    }

    const leaf = /^\s*-\s+(.+?)\s*$/.exec(line);
    if (leaf !== null && currentKind !== null) {
      out.push({ kind: currentKind, text: unquote(leaf[1]) });
    }
  }

  return out;
}

/** The first step of a given kind, or undefined. */
export function firstStep(raw: string, kind: StepFixture["kind"]): StepFixture | undefined {
  return steps(raw).find((step) => step.kind === kind);
}
