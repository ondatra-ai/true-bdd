#!/usr/bin/env python3
"""Conversation history capture for Claude Code hooks.

Two subcommands:
  prompt-submit  — wired to BOTH UserPromptSubmit and Stop hooks
                   with the same command.
  new-task       — invoked from .claude/commands/new-task.md.

  UserPromptSubmit (payload has a `prompt`): append it under a heading
  keyed on the writer's role. The main interactive session (default
  role "claude") logs a real human turn as "## user"; a headless
  worker (CLAUDE_HISTORY_ROLE=branch-name|commit-msg|pr-content ...)
  is claude prompting that worker via `claude -p`, so it logs as
  "## claude to @<role>".
  Stop (no prompt): append the whole assistant turn — every text
  block since the last user prompt, read from the transcript — under
  the writer's role heading (CLAUDE_HISTORY_ROLE, default "claude").
  The payload's `last_assistant_message` backstops the final block in
  case the transcript's tail hasn't been flushed yet. A per-session
  cursor (tmp/history-cursor/<session8>.json, keyed by prompt_id)
  records how many blocks of the current turn are already logged, so
  a blocking Stop hook that forces the turn to continue doesn't make
  the next Stop re-append the whole turn — only the continuation's
  new blocks land.

State file: docs/history/hook-state
    A single line: the current task file's name. Nothing else.
    Shared across sessions — a new session continues the same file.

History file: docs/history/<UTC-ts>-<session8>-<slug>.md

Off switch: CLAUDE_HISTORY_ROLE=0 skips all logging.
Rollover: /new-task removes the state file so the next prompt opens
    a fresh task file. Its own UserPromptSubmit (prompt == "/new-task")
    is filtered so it doesn't recreate the state file it just deleted.
"""

import json
import os
import re
import subprocess
import sys
import time
from pathlib import Path


# CLAUDE_PROJECT_DIR is set when Claude Code invokes the hooks, but NOT
# for the `!`-invoked /new-task slash command — fall back to the script's
# own location (<repo>/.claude/hooks/history.py).
REPO = Path(
    os.environ.get("CLAUDE_PROJECT_DIR") or Path(__file__).resolve().parents[2]
).resolve()
HISTORY_DIR = REPO / "docs" / "history"
STATE_FILE = HISTORY_DIR / "hook-state"
CURSOR_DIR = REPO / "tmp" / "history-cursor"
ROLE = os.environ.get("CLAUDE_HISTORY_ROLE", "") or "claude"


def _slugify(s: str) -> str:
    s = s[:120].lower()
    s = re.sub(r"[^a-z0-9]+", "-", s).strip("-")[:40].rstrip("-")
    return s or "msg"


def _now() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def _git_sha() -> str:
    try:
        r = subprocess.run(
            ["git", "-C", str(REPO), "rev-parse", "--short", "HEAD"],
            capture_output=True, text=True, timeout=2,
        )
        return (r.stdout.strip() or "-") if r.returncode == 0 else "-"
    except Exception:
        return "-"


# ---------- state: the current file's name, and nothing else ----------

def _load_current() -> str:
    try:
        return STATE_FILE.read_text().strip()
    except FileNotFoundError:
        return ""


def _save_current(filename: str) -> None:
    HISTORY_DIR.mkdir(parents=True, exist_ok=True)
    tmp = STATE_FILE.with_name(STATE_FILE.name + f".tmp.{os.getpid()}")
    tmp.write_text(filename)
    tmp.replace(STATE_FILE)


def _open_new_file(sid: str, first_prompt: str) -> str:
    HISTORY_DIR.mkdir(parents=True, exist_ok=True)
    ts = time.strftime("%Y%m%d-%H%M%S", time.gmtime())
    name = f"{ts}-{sid[:8]}-{_slugify(first_prompt)}.md"
    (HISTORY_DIR / name).touch(exist_ok=True)
    return name


def _append(filename: str, heading: str, body: str) -> None:
    line = f"## {heading}\n\n_{_now()} · {_git_sha()}_\n\n{body}\n\n"
    with (HISTORY_DIR / filename).open("a") as f:
        f.write(line)


# ---------- transcript ----------

def _content(entry: dict):
    c = (entry.get("message") or {}).get("content")
    return c if c is not None else (entry.get("content") or [])


def _read_jsonl(path: str):
    """Read JSONL; drop malformed lines (partial tail — payload backstops)."""
    out = []
    try:
        for line in Path(path).read_text().splitlines():
            line = line.strip()
            if line:
                try:
                    out.append(json.loads(line))
                except Exception:
                    pass
    except FileNotFoundError:
        pass
    return out


def _is_prompt(entry: dict) -> bool:
    c = _content(entry)
    if isinstance(c, str):
        return bool(c.strip())
    if isinstance(c, list):
        return any(isinstance(b, dict) and b.get("type") == "text"
                   and (b.get("text") or "").strip() for b in c)
    return False


def _turn_blocks(entries) -> list:
    """Every assistant text block after the last user prompt, in order."""
    start = 0
    for i, e in enumerate(entries):
        if e.get("isMeta") or e.get("isSidechain"):
            continue
        if e.get("type") == "user" and _is_prompt(e):
            start = i + 1
    blocks = []
    for e in entries[start:]:
        if e.get("isMeta") or e.get("isSidechain"):
            continue
        if e.get("type") != "assistant":
            continue
        c = _content(e)
        if isinstance(c, str):
            c = [{"type": "text", "text": c}]
        for b in c or []:
            if (isinstance(b, dict) and b.get("type") == "text"
                    and (b.get("text") or "").strip()):
                blocks.append(b["text"].strip())
    return blocks


# ---------- turn cursor (survives a blocking Stop hook) ----------

def _cursor_file(sid: str) -> Path:
    return CURSOR_DIR / f"{(sid[:8] or 'unknown')}.json"


def _cursor_read(sid: str, pid: str) -> int:
    """Blocks of the current turn already logged (0 if a different turn)."""
    if not pid:
        return 0
    try:
        d = json.loads(_cursor_file(sid).read_text())
        return int(d.get("blocks", 0)) if d.get("prompt_id") == pid else 0
    except Exception:
        return 0


def _cursor_write(sid: str, pid: str, blocks: int) -> None:
    if not pid:
        return
    try:
        CURSOR_DIR.mkdir(parents=True, exist_ok=True)
        tmp = _cursor_file(sid).with_suffix(f".tmp.{os.getpid()}")
        tmp.write_text(json.dumps({"prompt_id": pid, "blocks": blocks}))
        tmp.replace(_cursor_file(sid))
    except Exception:
        pass


# ---------- event handlers ----------

def new_task(_payload: dict) -> None:
    """Drop the state file so the next prompt opens a fresh task file."""
    STATE_FILE.unlink(missing_ok=True)


def prompt_submit(payload: dict) -> None:
    # Sub-agent invocations fire UserPromptSubmit too; skip them.
    if payload.get("agent_id"):
        return
    prompt = (payload.get("prompt") or payload.get("user_message") or "").strip()

    # /new-task fires UserPromptSubmit like any prompt, but its whole job is
    # to roll history over — logging it would recreate the state file it just
    # deleted (a fresh "...-new-task.md" holding only the command + ack). Drop
    # it; the Stop that follows finds no active file and is skipped too, so the
    # ack response is dropped for free.
    if prompt.split(maxsplit=1)[:1] == ["/new-task"]:
        return

    if prompt:
        # UserPromptSubmit: open a task file if none is active, log the prompt.
        # The main interactive session runs with the default role ("claude"),
        # so its prompt is a real human turn -> "## user". A headless worker
        # (CLAUDE_HISTORY_ROLE=branch-name|commit-msg|pr-content ...) is driven
        # by claude via `claude -p`, so its prompt is claude addressing that
        # worker -> "## claude to @<role>".
        heading = "user" if ROLE == "claude" else f"claude to @{ROLE}"
        filename = _load_current()
        if not filename:
            filename = _open_new_file(payload.get("session_id") or "", prompt)
            _save_current(filename)
        _append(filename, heading, prompt)
        return

    # Stop: append the whole assistant turn (all text blocks since the
    # last prompt). The payload's final message backstops the tail in
    # case the transcript hasn't flushed it yet.
    filename = _load_current()
    if not filename:
        return
    tp = payload.get("transcript_path") or ""
    blocks = _turn_blocks(_read_jsonl(tp)) if tp else []
    last = (payload.get("last_assistant_message") or "").strip()
    if last and (not blocks or blocks[-1] != last):
        blocks.append(last)
    sid = payload.get("session_id") or ""
    pid = payload.get("prompt_id") or ""
    done = _cursor_read(sid, pid)
    text = "\n\n".join(blocks[done:])
    if text:
        _append(filename, ROLE, text)
    _cursor_write(sid, pid, len(blocks))


HANDLERS = {
    "prompt-submit": prompt_submit,  # wired to BOTH UserPromptSubmit and Stop
    "new-task": new_task,
}


def main() -> None:
    if os.environ.get("CLAUDE_HISTORY_ROLE") == "0":
        return
    if len(sys.argv) < 2 or sys.argv[1] not in HANDLERS:
        return
    cmd = sys.argv[1]
    # new-task ignores its payload — never touch stdin. The `!`-invoked slash
    # command may inherit an interactive stdin, and json.load() would block on
    # it forever, hanging the command so the state file is never deleted.
    if cmd == "new-task":
        HANDLERS[cmd]({})
        return
    try:
        payload = json.load(sys.stdin)
    except Exception:
        payload = {}
    HANDLERS[cmd](payload)


if __name__ == "__main__":
    main()
