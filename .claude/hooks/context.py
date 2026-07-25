#!/usr/bin/env python3
"""Context archivist: distill task transcripts into docs/context/ ledgers.

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
                     finished turn, so every response updates the ledgers) and
                     by /new-task (which deletes the state file first, so the
                     just-closed task finalizes immediately); safe to run
                     manually at any time. No-ops under GITHUB_ACTIONS or
                     CLAUDE_HISTORY_ROLE so pipeline / worker claude -p
                     sessions never burn codex calls.
  process <file>   — process one history file (path or bare filename) even if
                     it is the active one; still skips if already marked done.

Per chunk: codex (read-only sandbox, --output-schema-forced JSON) reads the
transcript (or its newest chunk) plus the existing ledgers and CLAUDE.md, and
returns categorized findings; this script renders them into docs/context/*.md.
An item may carry a `supersedes` substring naming an older ledger bullet it
overrides — the old line is struck through (~~…~~ _(superseded DATE)_), never
deleted, and the new item appended. Consecutive chunks of the same task append
into one dated section instead of stacking headings. The raw reply is stored
as the done-marker at docs/history/context-processed/<file>.json; markers and
offsets advance only on success, so a failed codex run is retried by the next
sweep. All-empty findings still advance — empty is the normal case.

A flock on docs/history/context-sweep.lock keeps concurrent sweeps from
double-appending; a second sweep exits immediately, and the holder loops until
stable so turns that land mid-codex-run are never lost wakeups. `process` waits
on the same lock. Log: docs/history/context-sweep.log.
"""

import fcntl
import json
import os
import re
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
PROMPT_FILE = Path(__file__).resolve().parent / "context-prompt.md"
SCHEMA_FILE = Path(__file__).resolve().parent / "context-schema.json"
CODEX_TIMEOUT = 480
MIN_DELTA_BYTES = 300  # active-file ticks skip turns smaller than this

CATEGORIES = ("requirement", "decision", "correction", "fact", "follow_up")

# correction items land in requirements.md: a correction IS a requirement
# discovered the hard way (they keep a [correction] prefix for traceability).
LEDGER_FOR = {
    "requirement": "requirements.md",
    "correction": "requirements.md",
    "decision": "decisions.md",
    "fact": "facts.md",
    "follow_up": "follow-ups.md",
}

LEDGER_HEADERS = {
    "requirements.md": (
        "# Requirements — conversational ledger\n\n"
        "Standing requirements and corrections extracted from task transcripts"
        " by the\ncontext archivist (`.claude/hooks/context.py`). Append-only;"
        " newest at the\nbottom. A superseded entry is struck through, never"
        " deleted.\n\nNot the BDD requirements registry: product scenarios live"
        " in a host project's\n`docs/requirements.yaml`; this file holds"
        " engine-development knowledge from\nconversations.\n"
    ),
    "decisions.md": (
        "# Decisions — conversational ledger\n\n"
        "Choices made in conversation — what was chosen, why, and what was"
        " rejected —\nextracted from task transcripts by the context archivist"
        " (`.claude/hooks/context.py`).\nAppend-only; newest at the bottom."
        " A superseded entry is struck through, never\ndeleted.\n"
    ),
    "facts.md": (
        "# Facts — conversational ledger\n\n"
        "Dated empirical discoveries about external systems, extracted from"
        " task\ntranscripts by the context archivist"
        " (`.claude/hooks/context.py`). Append-only;\nnewest at the bottom."
        " A superseded entry is struck through, never deleted.\n"
    ),
    "follow-ups.md": (
        "# Follow-ups — conversational ledger\n\n"
        "Work explicitly deferred or requested but not done in its task,"
        " extracted from\ntask transcripts by the context archivist"
        " (`.claude/hooks/context.py`).\nAppend-only; newest at the bottom."
        " Strike through or remove items once done.\n"
    ),
}


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
    reply = {"task_summary": summary}
    for cat in CATEGORIES:
        reply[cat] = []
    return reply


def _valid_item(item) -> bool:
    if isinstance(item, str):
        return True
    return (
        isinstance(item, dict)
        and isinstance(item.get("text"), str)
        and (item.get("supersedes") is None or isinstance(item["supersedes"], str))
    )


def _run_codex(target: Path):
    """Run codex on one transcript (or chunk) file. Returns the parsed
    findings dict, or None on any failure."""
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
    for cat in CATEGORIES:
        items = reply.get(cat)
        if not isinstance(items, list) or any(not _valid_item(i) for i in items):
            log(f"malformed category '{cat}' for {target.name}")
            return None
    return reply


def _entry_date(filename: str) -> str:
    m = re.match(r"(\d{4})(\d{2})(\d{2})-", filename)
    if m:
        return "-".join(m.groups())
    return time.strftime("%Y-%m-%d", time.gmtime())


def _strike(text: str, matches: list, date: str) -> str:
    """Strike through the bullet line each match names — only when the match
    is unambiguous (exactly one un-struck bullet contains it)."""
    lines = text.split("\n")
    for m in matches:
        hits = [
            i for i, ln in enumerate(lines)
            if ln.strip().startswith("- ") and "~~" not in ln and m in ln
        ]
        if len(hits) == 1:
            ln = lines[hits[0]]
            indent = ln[: len(ln) - len(ln.lstrip())]
            lines[hits[0]] = (
                f"{indent}- ~~{ln.strip()[2:]}~~ _(superseded {date})_"
            )
        elif not hits:
            if any(m in ln and "~~" in ln for ln in lines):
                log(f"supersedes target already struck: {m[:80]!r}")
            else:
                log(f"supersedes target not found: {m[:80]!r}")
        else:
            log(f"supersedes ambiguous ({len(hits)} matches), skipped: {m[:80]!r}")
    return "\n".join(lines)


def _render(reply: dict, history_filename: str) -> int:
    """Apply supersedes strikes and append findings to their ledgers.
    Returns the number of items written."""
    date = _entry_date(history_filename)
    per_ledger = {}
    for cat in CATEGORIES:
        for item in reply[cat]:
            if isinstance(item, str):
                item = {"text": item}
            text = (item.get("text") or "").strip()
            if not text:
                continue
            if cat == "correction":
                text = f"[correction] {text}"
            d = per_ledger.setdefault(
                LEDGER_FOR[cat], {"items": [], "supersedes": []}
            )
            d["items"].append(text)
            sup = (item.get("supersedes") or "").strip()
            if sup:
                d["supersedes"].append(sup)
    if not per_ledger:
        return 0
    CONTEXT_DIR.mkdir(parents=True, exist_ok=True)
    marker = f"_transcript: {history_filename}_"
    for ledger, d in per_ledger.items():
        path = CONTEXT_DIR / ledger
        text = path.read_text() if path.exists() else LEDGER_HEADERS[ledger]
        text = _strike(text, d["supersedes"], date)
        block = "\n".join(f"- {t}" for t in d["items"])
        # A later chunk of the task that owns the ledger's last section
        # continues that section instead of stacking a new heading.
        last_heading = text.rfind("\n## ")
        if last_heading != -1 and marker in text[last_heading:]:
            text = text.rstrip("\n") + f"\n{block}\n"
        else:
            heading = (
                f"## {date} — {reply['task_summary'].strip() or 'untitled task'}"
            )
            text = text.rstrip("\n") + f"\n\n{heading}\n\n{marker}\n\n{block}\n"
        _write_atomic(path, text)
    return sum(len(d["items"]) for d in per_ledger.values())


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
    n = _render(reply, name)
    _mark_done(name, reply)
    _offset_path(name).unlink(missing_ok=True)
    log(f"done {name}: {n} item(s) -> docs/context/")
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
    n = _render(reply, name)
    _write_offset(name, len(data))
    log(f"tick {name}: {n} item(s) -> docs/context/")
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
