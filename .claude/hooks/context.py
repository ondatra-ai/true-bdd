#!/usr/bin/env python3
"""Context archivist: distill task transcripts into docs/context/requirements.md.

Subcommands:
  sweep            — one pass, two jobs: (1) fully process every finished,
                     unprocessed history file (finished = any docs/history/*.md
                     that is not the active task file named in
                     docs/history/hook-state); (2) incrementally process the
                     ACTIVE file's newest chunk — everything appended since the
                     last pass, tracked as a byte offset in
                     docs/history/context-processed/<file>.offset — when it grew
                     by at least MIN_DELTA_BYTES. Triggered in the background
                     by the Stop hook (chained after history.py appends the
                     finished turn, so every response updates the requirements
                     tree) and by /new-task (which deletes the state file first,
                     so the just-closed task finalizes immediately); safe to run
                     manually at any time. No-ops under GITHUB_ACTIONS or
                     CLAUDE_HISTORY_ROLE so pipeline / worker claude -p
                     sessions never burn codex calls.
  process <file>   — process one history file (path or bare filename) even if
                     it is the active one; still skips if already marked done.

Per chunk: codex (read-only sandbox, --output-schema-forced JSON) reads the
transcript (or its newest chunk) plus docs/context/requirements.md and
CLAUDE.md, and returns a list of operations (add/update/delete) against the
requirements tree; this script applies them to docs/context/requirements.md.
The tree has three flat sections — # Harness, # System, # Product — each a list
of `## <requirement>` headings; an operation names its `perspective`
(harness/system/product) to pick the section. An operation's `match` names a
unique existing requirement to update or delete (ambiguous/missing matches are
skipped and logged); an `add` appends a new requirement to its section. The raw
reply is stored as the done-marker at docs/history/context-processed/<file>.json;
markers and offsets advance only on success, so a failed codex run is retried
by the next sweep. Empty operations still advance — empty is the normal case.

A flock on docs/history/context-sweep.lock keeps concurrent sweeps from
double-writing; a second sweep exits immediately, and the holder loops until
stable so turns that land mid-codex-run are never lost wakeups. `process` waits
on the same lock. Log: docs/history/context-sweep.log.
"""

import fcntl
import json
import os
import subprocess
import sys
import time
from pathlib import Path

REPO = Path(
    os.environ.get("CLAUDE_PROJECT_DIR") or Path(__file__).resolve().parents[2]
).resolve()
HISTORY_DIR = REPO / "docs" / "history"
STATE_FILE = HISTORY_DIR / "hook-state"
PROCESSED_DIR = HISTORY_DIR / "context-processed"
LOCK_FILE = HISTORY_DIR / "context-sweep.lock"
LOG_FILE = HISTORY_DIR / "context-sweep.log"
CONTEXT_DIR = REPO / "docs" / "context"
REQUIREMENTS_FILE = CONTEXT_DIR / "requirements.md"
PROMPT_FILE = Path(__file__).resolve().parent / "context-prompt.md"
SCHEMA_FILE = Path(__file__).resolve().parent / "context-schema.json"
CODEX_TIMEOUT = 480
MIN_DELTA_BYTES = 300  # active-file ticks skip turns smaller than this

_VALID_ACTIONS = ("add", "update", "delete")
SECTIONS = ("harness", "system", "product")
SECTION_HEADINGS = {"harness": "# Harness", "system": "# System", "product": "# Product"}


def log(msg: str) -> None:
    line = f"{time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())} {msg}"
    print(line, file=sys.stderr)
    try:
        HISTORY_DIR.mkdir(parents=True, exist_ok=True)
        with LOG_FILE.open("a") as f:
            f.write(line + "\n")
    except OSError:
        pass


def _active_file() -> str:
    try:
        return STATE_FILE.read_text().strip()
    except FileNotFoundError:
        return ""


def _finished_unprocessed() -> list:
    active = _active_file()
    out = []
    for p in sorted(HISTORY_DIR.glob("*.md")):
        if p.name == active:
            continue
        if (PROCESSED_DIR / (p.name + ".json")).exists():
            continue
        out.append(p)
    return out


def _offset_path(name: str) -> Path:
    return PROCESSED_DIR / (name + ".offset")


def _read_offset(name: str) -> int:
    try:
        return max(0, int(_offset_path(name).read_text().strip()))
    except (OSError, ValueError):
        return 0


def _write_atomic(path: Path, text: str) -> None:
    tmp = path.with_name(path.name + ".tmp")
    tmp.write_text(text)
    os.replace(tmp, path)


def _write_offset(name: str, offset: int) -> None:
    PROCESSED_DIR.mkdir(parents=True, exist_ok=True)
    _write_atomic(_offset_path(name), f"{offset}\n")


def _acquire_lock(blocking: bool):
    """Returns an open, flocked file handle, or None when non-blocking and
    the lock is held elsewhere."""
    HISTORY_DIR.mkdir(parents=True, exist_ok=True)
    lock = LOCK_FILE.open("w")
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | (0 if blocking else fcntl.LOCK_NB))
    except OSError:
        lock.close()
        return None
    return lock


def _empty_reply(summary: str) -> dict:
    return {"task_summary": summary, "operations": []}


def _valid_op(op) -> bool:
    """Structural + semantic check for one operation. Lenient on stray null
    fields, strict on the fields each action needs."""
    if not isinstance(op, dict):
        return False
    if op.get("action") not in _VALID_ACTIONS:
        return False
    if op.get("perspective") not in SECTIONS:
        return False
    requirement = op.get("requirement")
    match = op.get("match")
    if not (requirement is None or isinstance(requirement, str)):
        return False
    if not (match is None or isinstance(match, str)):
        return False
    if op["action"] == "add" and not (requirement and requirement.strip()):
        return False
    if op["action"] == "update" and not (requirement and requirement.strip()):
        return False
    if op["action"] in ("update", "delete") and not (match and match.strip()):
        return False
    return True


def _run_codex(target: Path):
    """Run codex on one transcript (or chunk) file. Returns the parsed
    reply dict, or None on any failure."""
    rel = target.relative_to(REPO)
    prompt = PROMPT_FILE.read_text().replace("{HISTORY_FILE}", str(rel))
    PROCESSED_DIR.mkdir(parents=True, exist_ok=True)
    out_file = PROCESSED_DIR / (target.name + ".reply.tmp")
    cmd = [
        "codex", "exec", "-s", "read-only", "--ephemeral",
        "-C", str(REPO),
        "--output-schema", str(SCHEMA_FILE),
        "-o", str(out_file),
        "--color", "never",
        "-",
    ]
    try:
        r = subprocess.run(
            cmd, input=prompt, capture_output=True, text=True,
            timeout=CODEX_TIMEOUT,
        )
    except subprocess.TimeoutExpired:
        log(f"codex TIMEOUT ({CODEX_TIMEOUT}s) on {target.name}")
        return None
    except FileNotFoundError:
        log("codex CLI not found on PATH — skipping sweep")
        return None
    if r.returncode != 0:
        tail = (r.stderr or r.stdout or "").strip()[-500:]
        log(f"codex exit {r.returncode} on {target.name}: {tail}")
        return None
    try:
        reply = json.loads(out_file.read_text())
    except (OSError, ValueError) as e:
        log(f"unparseable codex reply for {target.name}: {e}")
        return None
    finally:
        out_file.unlink(missing_ok=True)
    if not isinstance(reply, dict) or not isinstance(reply.get("task_summary"), str):
        log(f"malformed codex reply for {target.name}")
        return None
    ops = reply.get("operations")
    if not isinstance(ops, list) or any(not _valid_op(o) for o in ops):
        log(f"malformed operations for {target.name}")
        return None
    return reply


# --- requirements tree: parse / serialize / mutate ---------------------------

def _empty_tree() -> dict:
    return {name: [] for name in SECTIONS}


def _parse_tree(text: str) -> dict:
    """Parse into {harness: [...], system: [...], product: [...]}, each a list
    of `## <requirement>` heading texts in document order."""
    tree = _empty_tree()
    cur = None
    for line in text.splitlines():
        s = line.strip()
        if s in SECTION_HEADINGS.values():
            cur = next(k for k, v in SECTION_HEADINGS.items() if v == s)
            continue
        if cur and s.startswith("## ") and not s.startswith("### "):
            tree[cur].append(s[3:].strip())
    return tree


def _serialize(tree: dict) -> str:
    blocks = []
    for name in SECTIONS:
        lines = [SECTION_HEADINGS[name]]
        for r in tree[name]:
            lines.append(f"## {r}")
        blocks.append("\n".join(lines))
    return "\n\n".join(blocks) + "\n"


def _find_unique(items: list, match: str):
    """Index of the unique item containing `match`, or None when not found or
    ambiguous."""
    hits = [i for i, r in enumerate(items) if match in r]
    if len(hits) == 1:
        return hits[0]
    if not hits:
        log(f"match not found: {match[:80]!r}")
    else:
        log(f"match ambiguous ({len(hits)}), skipped: {match[:80]!r}")
    return None


def _apply_operations(text: str, ops: list):
    """Apply requirement operations to the tree. Returns (new_text, n_applied)."""
    tree = _parse_tree(text)
    applied = 0
    for op in ops:
        bucket = tree[op["perspective"]]
        action = op["action"]
        if action == "add":
            req = (op.get("requirement") or "").strip()
            if req in bucket:
                log(f"add dup, skipped: {req[:80]!r}")
            else:
                bucket.append(req)
                applied += 1
        elif action == "update":
            new = (op.get("requirement") or "").strip()
            idx = _find_unique(bucket, (op.get("match") or "").strip())
            if idx is not None:
                bucket[idx] = new
                applied += 1
        elif action == "delete":
            idx = _find_unique(bucket, (op.get("match") or "").strip())
            if idx is not None:
                del bucket[idx]
                applied += 1
    return _serialize(tree), applied


def _render(reply: dict) -> int:
    """Apply the reply's operations to requirements.md.
    Returns the number of operations applied."""
    ops = reply.get("operations") or []
    if not ops:
        return 0
    CONTEXT_DIR.mkdir(parents=True, exist_ok=True)
    text = REQUIREMENTS_FILE.read_text() if REQUIREMENTS_FILE.exists() else _serialize(_empty_tree())
    new_text, applied = _apply_operations(text, ops)
    if new_text != text:
        _write_atomic(REQUIREMENTS_FILE, new_text)
    return applied


def _mark_done(history_filename: str, reply: dict) -> None:
    PROCESSED_DIR.mkdir(parents=True, exist_ok=True)
    marker = PROCESSED_DIR / (history_filename + ".json")
    marker.write_text(json.dumps(reply, ensure_ascii=False, indent=2) + "\n")


def process_one(history_path: Path) -> bool:
    """Finalize one history file: process everything after its offset (the
    whole file when no incremental passes ran), then mark it done."""
    name = history_path.name
    if (PROCESSED_DIR / (name + ".json")).exists():
        log(f"already processed: {name}")
        return True
    data = history_path.read_bytes()
    offset = _read_offset(name)
    if offset > len(data):
        offset = 0
    delta = data[offset:]
    if offset and not delta.strip():
        _mark_done(name, _empty_reply("(finalized — no content after last incremental pass)"))
        _offset_path(name).unlink(missing_ok=True)
        log(f"done {name}: nothing new after offset {offset}")
        return True
    if offset:
        target = PROCESSED_DIR / (name + ".delta.md")
        PROCESSED_DIR.mkdir(parents=True, exist_ok=True)
        target.write_bytes(delta)
    else:
        target = history_path
    log(f"processing {name} (bytes {offset}..{len(data)})")
    try:
        reply = _run_codex(target)
    finally:
        if target is not history_path:
            target.unlink(missing_ok=True)
    if reply is None:
        return False
    n = _render(reply)
    _mark_done(name, reply)
    _offset_path(name).unlink(missing_ok=True)
    log(f"done {name}: {n} op(s) -> docs/context/requirements.md")
    return True


def _process_active() -> bool:
    """Incrementally process the active file's newest chunk, if it grew.
    Returns True only when a chunk was distilled and the offset advanced."""
    name = _active_file()
    if not name:
        return False
    path = HISTORY_DIR / name
    if not path.exists():
        return False
    if (PROCESSED_DIR / (name + ".json")).exists():
        return False  # manually finalized via `process` — don't double-write
    data = path.read_bytes()
    offset = _read_offset(name)
    if offset > len(data):
        # File shrank (should never happen with an append-only writer) —
        # persist the reset so a later regrowth can't resume mid-void.
        offset = 0
        _write_offset(name, 0)
    delta = data[offset:]
    if len(delta) < MIN_DELTA_BYTES or not delta.strip():
        return False
    PROCESSED_DIR.mkdir(parents=True, exist_ok=True)
    target = PROCESSED_DIR / (name + ".delta.md")
    target.write_bytes(delta)
    log(f"active tick {name}: bytes {offset}..{len(data)}")
    try:
        reply = _run_codex(target)
    finally:
        target.unlink(missing_ok=True)
    if reply is None:
        return False
    n = _render(reply)
    _write_offset(name, len(data))
    log(f"tick {name}: {n} op(s) -> docs/context/requirements.md")
    return True


def sweep() -> None:
    # Pipeline runs and headless claude -p workers fire the same Stop hook;
    # a codex call per worker turn is waste — the interactive session owns
    # context extraction.
    if os.environ.get("GITHUB_ACTIONS") or os.environ.get("CLAUDE_HISTORY_ROLE"):
        return
    lock = _acquire_lock(blocking=False)
    if lock is None:
        log("another sweep is running — exiting")
        return
    try:
        # Loop until stable: turns (or /new-task rollovers) that land while a
        # codex run is in flight are picked up before the lock is released —
        # otherwise their own sweeps, having bounced off the lock, would be
        # lost wakeups. Files already attempted this sweep are not retried
        # (a persistently failing codex must not spin the loop).
        attempted = set()
        while True:
            progressed = False
            for p in _finished_unprocessed():
                if p.name in attempted:
                    continue
                attempted.add(p.name)
                process_one(p)
                progressed = True
            if _process_active():
                progressed = True
            if not progressed:
                break
    finally:
        lock.close()


def main() -> None:
    if len(sys.argv) < 2 or sys.argv[1] not in ("sweep", "process"):
        print(__doc__, file=sys.stderr)
        sys.exit(2)
    if sys.argv[1] == "sweep":
        sweep()
        return
    if len(sys.argv) < 3:
        print("usage: context.py process <history-file>", file=sys.stderr)
        sys.exit(2)
    arg = Path(sys.argv[2])
    path = arg if arg.is_absolute() else (
        arg if arg.exists() else HISTORY_DIR / arg.name
    )
    if not path.exists():
        print(f"no such history file: {sys.argv[2]}", file=sys.stderr)
        sys.exit(1)
    lock = _acquire_lock(blocking=True)  # wait out any in-flight sweep
    try:
        ok = process_one(path.resolve())
    finally:
        lock.close()
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
