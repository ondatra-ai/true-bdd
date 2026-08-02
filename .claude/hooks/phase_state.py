#!/usr/bin/env python3
"""Phase-state enforcement for the implement-task workflow.

Records which implement-task agents actually ran (spawns from PreToolUse[Agent],
completions from PostToolUse[Agent] + SubagentStop) into a single state file, and
gates the three actions that historically let the orchestrator skip the reviewer:

- spawning agents out of order            (PreToolUse[Agent]  -> permission deny)
- ending the turn after coder, no review  (Stop               -> decision block)
- committing an unreviewed task           (PreToolUse[Bash]   -> permission deny)

Design rules (mirrors block_test_edits.py):
- Every hook mode FAILS OPEN: any internal error warns to stderr and allows.
  The orchestrator's manifests + definition-of-done checklist stay the backstop.
- Gates read ONE file: tmp/implement-task/active.json. Closing a task archives
  it to tmp/implement-task/<slug>/state.<ts>.json and removes active.json.
- Writes are atomic (tmp + os.replace) under an fcntl lock — Pre/Post/Stop/
  UserPromptSubmit hooks run concurrently.
- The Stop gate arms PER TURN (same prompt_id as the agent activity), so normal
  conversation between phases is never blocked. Cross-turn pressure comes from
  the commit gate + the UserPromptSubmit/SessionStart nag banner instead.
- Escape hatches are deliberate state transitions (`block`, `close --abandoned`,
  `skip-gate` — all user-visible and audited), never silence. After 3 blocked
  stops in one turn the gate yields with status=auto_blocked: the stop is
  allowed but the commit gate and banner stay armed.

Non-goal: this guarantees the reviewer RAN, not that the review PASSED — the
definition-of-done checklist remains prompt-level.

Hook wiring (.claude/settings.json): see `hook` subcommand modes below.
CLI (run by the orchestrator per implement-task SKILL.md):
    phase_state.py start <slug> [--lane tiny|easy|hard]   # lane per complexity-matrix.md
    phase_state.py escalate --reason "<trigger>"          # bump lane one level, audited
    phase_state.py block --reason "<why>"
    phase_state.py skip-gate --phase <planner|test_author|coder|reviewer> \
                   --approved-by-user "<verbatim user quote>"
    phase_state.py close --done
    phase_state.py close --abandoned --approved-by-user "<verbatim user quote>"
    phase_state.py status
"""

from __future__ import annotations

import fcntl
import json
import os
import re
import sys
from datetime import datetime, timezone
from pathlib import Path

REPO = Path(os.environ.get("CLAUDE_PROJECT_DIR") or Path(__file__).resolve().parents[2])
STATE_DIR = REPO / "tmp" / "implement-task"
ACTIVE = STATE_DIR / "active.json"
LOCK = STATE_DIR / ".lock"
AUDIT = STATE_DIR / "phase-state.log"
METRICS = REPO / "docs" / "context" / "skill-metrics.jsonl"

AGENT_PHASE = {
    "implement-task-planner": "planner",
    "implement-task-test-author": "test_author",
    "implement-task-coder": "coder",
    "implement-task-reviewer": "reviewer",
}
PHASE_ORDER = ["planner", "test_author", "coder", "reviewer"]
# Per-lane phase -> predecessor that must have >=1 completion (>=1, never ==0:
# re-runs of any phase are legitimate — blocker guidance, approved test fixes,
# re-review). Lanes per references/complexity-matrix.md: tiny/easy run no planner
# agent (plan is inline / folded into the test-author), so test_author has no
# required predecessor there. Unknown/undeclared lane = hard (the safe default).
LANES = ["tiny", "easy", "hard"]
PREDECESSOR_BY_LANE = {
    "hard": {"test_author": "planner", "coder": "test_author", "reviewer": "coder"},
    "easy": {"coder": "test_author", "reviewer": "coder"},
    "tiny": {"coder": "test_author", "reviewer": "coder"},
}


def predecessor(state: dict, phase: str) -> str | None:
    lane = state.get("lane") if state.get("lane") in LANES else "hard"
    return PREDECESSOR_BY_LANE[lane].get(phase)

GIT_COMMIT_RE = re.compile(r"\bgit\s+(-C\s+\S+\s+)?(-{1,2}\S+\s+)*commit\b")
PR_SCRIPT_RE = re.compile(r"pr-commit/commit\.sh")
GH_PR_RE = re.compile(r"\bgh\s+pr\s+(create|merge)\b")
BLOCK_MARKER_RE = re.compile(r"^TASK BLOCKED:(.*)$", re.MULTILINE)
SLUG_RE = re.compile(r"^\s*slug:\s*([A-Za-z0-9._-]+)", re.MULTILINE)

MAX_STOP_BLOCKS = 2  # blocks per turn before yielding as auto_blocked


def now() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def audit(record: dict) -> None:
    try:
        STATE_DIR.mkdir(parents=True, exist_ok=True)
        with AUDIT.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps({"ts": now(), **record}, ensure_ascii=False) + "\n")
    except OSError:
        pass


class Lock:
    def __enter__(self):
        STATE_DIR.mkdir(parents=True, exist_ok=True)
        self.fh = LOCK.open("w")
        fcntl.flock(self.fh, fcntl.LOCK_EX)
        return self

    def __exit__(self, *exc):
        fcntl.flock(self.fh, fcntl.LOCK_UN)
        self.fh.close()
        return False


def load_state() -> dict | None:
    """Return the active state, or None. Corrupt files are renamed aside (self-heal)."""
    if not ACTIVE.exists():
        return None
    try:
        return json.loads(ACTIVE.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        try:
            ACTIVE.rename(ACTIVE.with_suffix(f".corrupt-{now().replace(':', '')}"))
        except OSError:
            pass
        print(f"phase_state: corrupt state moved aside ({exc})", file=sys.stderr)
        return None


def save_state(state: dict) -> None:
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    tmp = ACTIVE.with_suffix(".tmp")
    tmp.write_text(json.dumps(state, indent=1, ensure_ascii=False), encoding="utf-8")
    os.replace(tmp, ACTIVE)


def new_state(slug: str, lane: str = "hard") -> dict:
    return {
        "slug": slug,
        "lane": lane,
        "branch": current_branch(),
        "session_id": None,
        "created_ts": now(),
        "status": "in_progress",
        "phases": {p: {"spawns": [], "completions": []} for p in PHASE_ORDER},
        "turn": {"prompt_id": None, "activity": False, "stop_blocks": 0},
        "block_reason": None,
        "events": [],
    }


def add_event(state: dict, kind: str, **extra) -> None:
    state["events"].append({"ts": now(), "event": kind, **extra})


def current_branch() -> str:
    try:
        import subprocess

        return subprocess.run(
            ["git", "-C", str(REPO), "rev-parse", "--abbrev-ref", "HEAD"],
            capture_output=True, text=True, timeout=5, check=False,
        ).stdout.strip()
    except Exception:  # noqa: BLE001
        return ""


def completions(state: dict, phase: str) -> int:
    return len(state["phases"][phase]["completions"])


def reviewer_pending(state: dict) -> bool:
    return completions(state, "coder") >= 1 and completions(state, "reviewer") == 0


def archive(state: dict) -> None:
    slug_dir = STATE_DIR / state.get("slug", "unknown")
    slug_dir.mkdir(parents=True, exist_ok=True)
    dest = slug_dir / f"state.{now().replace(':', '')}.json"
    dest.write_text(json.dumps(state, indent=1, ensure_ascii=False), encoding="utf-8")
    ACTIVE.unlink(missing_ok=True)


# ---------------------------------------------------------------------------
# hook outputs

def allow() -> int:
    return 0


def deny(reason: str) -> int:
    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": reason,
        }
    }))
    return 0


def block_stop(reason: str) -> int:
    print(json.dumps({"decision": "block", "reason": reason}))
    return 0


def context(text: str, event: str) -> int:
    print(json.dumps({
        "hookSpecificOutput": {"hookEventName": event, "additionalContext": text}
    }))
    return 0


def banner(state: dict) -> str:
    lines = [
        f"[phase-state] implement-task '{state['slug']}' (lane: {state.get('lane', 'hard')}) "
        f"on branch '{state['branch']}' is {state['status']}: "
        + ", ".join(
            f"{p}={completions(state, p)}/{len(state['phases'][p]['spawns'])} done/spawned"
            for p in PHASE_ORDER
        )
    ]
    if reviewer_pending(state):
        lines.append(
            "The coder has completed but implement-task-reviewer has NOT run. The task "
            "is not reviewable-complete: spawn implement-task-reviewer (minimal prompt: "
            "slug, plan path, diff artifact path) before closing or committing. NEVER "
            "review inline."
        )
    if state["status"] in ("blocked", "auto_blocked"):
        lines.append(f"Recorded blocker: {state.get('block_reason') or '(no reason recorded)'} "
                     "— resolve it or ask the user; `phase_state.py close --abandoned "
                     "--approved-by-user \"<quote>\"` only with explicit user approval.")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# hook modes (all fail-open via main()'s try/except)

def hook_agent_pre(payload: dict) -> int:
    subagent = (payload.get("tool_input") or {}).get("subagent_type", "")
    phase = AGENT_PHASE.get(subagent)
    if phase is None:
        return allow()
    with Lock():
        state = load_state()
        if state is None:
            prompt = (payload.get("tool_input") or {}).get("prompt", "")
            m = SLUG_RE.search(prompt)
            slug = m.group(1).rstrip(".") if m else f"unknown-{now()[:10]}"
            state = new_state(slug)
            add_event(state, "auto_created", by=subagent)
        state["session_id"] = payload.get("session_id")
        pred = predecessor(state, phase)
        if pred and completions(state, pred) == 0:
            skipped = next(
                (e for e in state["events"]
                 if e["event"] == "skip_gate" and e.get("phase") == pred), None)
            if not skipped:
                add_event(state, "order_deny", phase=phase, missing=pred)
                save_state(state)
                audit({"mode": "agent-pre", "decision": "deny", "phase": phase})
                return deny(
                    f"implement-task phase order (lane: {state.get('lane', 'hard')}): "
                    f"cannot spawn {subagent} — predecessor '{pred}' has no recorded "
                    f"completion in tmp/implement-task/active.json. Run the {pred} phase "
                    f"first. If the task's lane was classified lighter (see "
                    f"references/complexity-matrix.md), declare it: "
                    f".claude/hooks/phase_state.py start {state.get('slug', '<slug>')} "
                    f"--lane tiny|easy — then re-spawn. If this is a legitimate resume "
                    f"whose state was reset, ask the user and record their approval via: "
                    f".claude/hooks/phase_state.py skip-gate --phase {pred} "
                    f"--approved-by-user \"<their verbatim words>\"."
                )
        state["phases"][phase]["spawns"].append({"ts": now(), "prompt_id": payload.get("prompt_id")})
        state["turn"] = {"prompt_id": payload.get("prompt_id"), "activity": True,
                        "stop_blocks": state["turn"].get("stop_blocks", 0)}
        if state["status"] in ("blocked", "auto_blocked"):
            state["status"] = "in_progress"
            state["block_reason"] = None
            add_event(state, "unblocked_by_spawn", phase=phase)
        add_event(state, "spawn", phase=phase)
        save_state(state)
    audit({"mode": "agent-pre", "decision": "allow", "phase": phase})
    return allow()


def hook_agent_post(payload: dict) -> int:
    subagent = (payload.get("tool_input") or {}).get("subagent_type", "")
    phase = AGENT_PHASE.get(subagent)
    if phase is None:
        return allow()
    resp = payload.get("tool_response") or {}
    if resp.get("status") != "completed":
        return allow()  # background launch etc. — SubagentStop records it later
    with Lock():
        state = load_state()
        if state is None:
            return allow()
        agent_id = resp.get("agentId")
        existing = next((c for c in state["phases"][phase]["completions"]
                         if agent_id and c.get("agent_id") == agent_id), None)
        if existing is not None:
            # SubagentStop fires before PostToolUse and records the completion
            # without usage — enrich it rather than dropping the token data.
            existing.update({
                "model": resp.get("resolvedModel"),
                "total_tokens": resp.get("totalTokens"),
                "duration_ms": resp.get("totalDurationMs"),
            })
            save_state(state)
            return allow()
        state["phases"][phase]["completions"].append({
            "ts": now(),
            "agent_id": agent_id,
            "source": "post-tool-use",
            "model": resp.get("resolvedModel"),
            "total_tokens": resp.get("totalTokens"),
            "duration_ms": resp.get("totalDurationMs"),
        })
        state["turn"] = {"prompt_id": payload.get("prompt_id"), "activity": True,
                        "stop_blocks": state["turn"].get("stop_blocks", 0)}
        add_event(state, "completion", phase=phase)
        save_state(state)
    audit({"mode": "agent-post", "phase": phase})
    return allow()


def hook_subagent_stop(payload: dict) -> int:
    phase = AGENT_PHASE.get(payload.get("agent_type", ""))
    if phase is None:
        return allow()
    with Lock():
        state = load_state()
        if state is None:
            return allow()
        agent_id = payload.get("agent_id")
        if agent_id and any(c.get("agent_id") == agent_id
                            for c in state["phases"][phase]["completions"]):
            return allow()  # already recorded by agent-post
        state["phases"][phase]["completions"].append(
            {"ts": now(), "agent_id": agent_id, "source": "subagent-stop"})
        add_event(state, "completion", phase=phase, source="subagent-stop")
        save_state(state)
    audit({"mode": "subagent-stop", "phase": phase})
    return allow()


def hook_prompt_submit(payload: dict) -> int:
    if payload.get("agent_id"):  # a subagent's own prompt, not a user turn
        return allow()
    with Lock():
        state = load_state()
        if state is None:
            return allow()
        state["turn"] = {"prompt_id": payload.get("prompt_id"), "activity": False,
                        "stop_blocks": 0}
        save_state(state)
        if reviewer_pending(state) or state["status"] in ("blocked", "auto_blocked"):
            return context(banner(state), "UserPromptSubmit")
    return allow()


def hook_session_start(payload: dict) -> int:
    state = load_state()
    if state is None:
        return allow()
    if reviewer_pending(state) or state["status"] in ("blocked", "auto_blocked"):
        return context(banner(state), "SessionStart")
    return allow()


def hook_stop_gate(payload: dict) -> int:
    with Lock():
        state = load_state()
        if state is None or state["status"] != "in_progress":
            return allow()
        marker = BLOCK_MARKER_RE.search(payload.get("last_assistant_message") or "")
        if marker:
            state["status"] = "blocked"
            state["block_reason"] = marker.group(1).strip() or "TASK BLOCKED (no reason given)"
            add_event(state, "blocked_by_marker")
            save_state(state)
            audit({"mode": "stop-gate", "decision": "allow", "via": "marker"})
            return allow()
        armed = (
            reviewer_pending(state)
            and state["turn"].get("activity")
            and state["turn"].get("prompt_id") == payload.get("prompt_id")
        )
        if not armed:
            return allow()
        state["turn"]["stop_blocks"] = state["turn"].get("stop_blocks", 0) + 1
        blocks = state["turn"]["stop_blocks"]
        add_event(state, "stop_block", n=blocks)
        if blocks > MAX_STOP_BLOCKS:
            state["status"] = "auto_blocked"
            state["block_reason"] = "stop gate yielded after repeated blocks; reviewer still pending"
            save_state(state)
            print(
                "phase_state: YIELDING after repeated stop blocks — task is now "
                "auto_blocked; commit gate stays armed until the reviewer runs.",
                file=sys.stderr,
            )
            audit({"mode": "stop-gate", "decision": "yield-auto-blocked"})
            return allow()
        save_state(state)
        audit({"mode": "stop-gate", "decision": "block", "n": blocks})
        if blocks == 1:
            return block_stop(
                f"implement-task '{state['slug']}': the coder phase is complete but "
                "implement-task-reviewer has not run. Spawn implement-task-reviewer NOW "
                "with a minimal prompt (first line 'slug: <slug>', then the plan path and "
                "the diff artifact path — never the inline diff). Reviewing inline is "
                "prohibited. If the spawn just failed, retry once with that minimal prompt."
            )
        return block_stop(
            "Reviewer still not run after a retry window. If it cannot run, record the "
            "blocker deliberately: run `.claude/hooks/phase_state.py block --reason "
            "\"<exact failure>\"`, then report the situation to the user and ask how to "
            "proceed. Do NOT review inline; do NOT close the task."
        )


def hook_bash_gate(payload: dict) -> int:
    if not ACTIVE.exists():  # fast path: fires on every Bash call
        return allow()
    cmd = (payload.get("tool_input") or {}).get("command", "")
    if not (GIT_COMMIT_RE.search(cmd) or PR_SCRIPT_RE.search(cmd) or GH_PR_RE.search(cmd)):
        return allow()
    state = load_state()
    if state is None or state["status"].startswith("closed"):
        return allow()
    if not reviewer_pending(state):
        return allow()
    if state.get("branch") and current_branch() != state["branch"]:
        return allow()  # unrelated work on another branch is never blocked
    audit({"mode": "bash-gate", "decision": "deny", "cmd": cmd[:200]})
    return deny(
        f"implement-task '{state['slug']}' is not reviewable-complete: the coder finished "
        "but implement-task-reviewer never ran (tmp/implement-task/active.json). Spawn the "
        "reviewer first, or — only with explicit user approval — abandon via "
        "`.claude/hooks/phase_state.py close --abandoned --approved-by-user \"<quote>\"`."
    )


HOOK_MODES = {
    "agent-pre": hook_agent_pre,
    "agent-post": hook_agent_post,
    "subagent-stop": hook_subagent_stop,
    "prompt-submit": hook_prompt_submit,
    "session-start": hook_session_start,
    "stop-gate": hook_stop_gate,
    "bash-gate": hook_bash_gate,
}


# ---------------------------------------------------------------------------
# CLI subcommands (run by the orchestrator)

def phase_metrics(state: dict) -> dict:
    out = {}
    for phase, data in state["phases"].items():
        spawns, comps = data["spawns"], data["completions"]
        tokens = sum(c.get("total_tokens") or 0 for c in comps)
        duration = sum(c.get("duration_ms") or 0 for c in comps)
        out[phase] = {
            "spawns": len(spawns),
            "completions": len(comps),
            "total_tokens": tokens or None,
            "total_duration_ms": duration or None,
        }
    return out


def cli_start(args: list[str]) -> int:
    slug = args[0] if args and not args[0].startswith("--") else None
    lane = _flag(args, "--lane")
    if not slug or (lane is not None and lane not in LANES):
        print("usage: phase_state.py start <slug> [--lane tiny|easy|hard]",
              file=sys.stderr)
        return 1
    with Lock():
        old = load_state()
        if old is not None and old.get("slug") != slug:
            old["status"] = "closed_abandoned"
            add_event(old, "abandoned_by_new_start", new_slug=slug)
            archive(old)
            print(f"phase_state: archived previous task '{old.get('slug')}' as abandoned")
            old = None
        if old is not None:
            if lane and old.get("lane") != lane:
                add_event(old, "lane_change", frm=old.get("lane"), to=lane)
                old["lane"] = lane
                save_state(old)
                print(f"phase_state: task '{slug}' lane set to {lane}")
            else:
                print(f"phase_state: task '{slug}' already active "
                      f"(lane: {old.get('lane', 'hard')})")
            return 0
        state = new_state(slug, lane or "hard")
        add_event(state, "started", lane=state["lane"])
        save_state(state)
    print(f"phase_state: started '{slug}' on branch '{state['branch']}' "
          f"(lane: {state['lane']})")
    return 0


def cli_escalate(args: list[str]) -> int:
    reason = _flag(args, "--reason")
    if not reason:
        print("usage: phase_state.py escalate --reason \"<trigger>\"", file=sys.stderr)
        return 1
    with Lock():
        state = load_state()
        if state is None:
            print("phase_state: no active task", file=sys.stderr)
            return 1
        cur = state.get("lane") if state.get("lane") in LANES else "hard"
        if cur == "hard":
            print("phase_state: already at lane 'hard' — nothing to escalate to")
            return 0
        nxt = LANES[LANES.index(cur) + 1]
        state["lane"] = nxt
        add_event(state, "escalate", frm=cur, to=nxt, reason=reason)
        save_state(state)
    print(f"phase_state: lane escalated {cur} -> {nxt}: {reason}")
    return 0


def cli_block(args: list[str]) -> int:
    reason = _flag(args, "--reason")
    if not reason:
        print("usage: phase_state.py block --reason \"<why>\"", file=sys.stderr)
        return 1
    with Lock():
        state = load_state()
        if state is None:
            print("phase_state: no active task", file=sys.stderr)
            return 1
        state["status"] = "blocked"
        state["block_reason"] = reason
        add_event(state, "blocked", reason=reason)
        save_state(state)
    print(f"phase_state: task '{state['slug']}' marked blocked: {reason}")
    return 0


def cli_skip_gate(args: list[str]) -> int:
    phase = _flag(args, "--phase")
    quote = _flag(args, "--approved-by-user")
    if phase not in PHASE_ORDER or not quote:
        print("usage: phase_state.py skip-gate --phase <phase> --approved-by-user \"<quote>\"",
              file=sys.stderr)
        return 1
    with Lock():
        state = load_state()
        if state is None:
            print("phase_state: no active task", file=sys.stderr)
            return 1
        add_event(state, "skip_gate", phase=phase, approved_by_user=quote)
        save_state(state)
    print(f"phase_state: order gate for '{phase}' waived (user-approved, audited)")
    return 0


def cli_close(args: list[str]) -> int:
    done = "--done" in args
    abandoned = "--abandoned" in args
    if done == abandoned:
        print("usage: phase_state.py close --done | --abandoned --approved-by-user \"<quote>\"",
              file=sys.stderr)
        return 1
    with Lock():
        state = load_state()
        if state is None:
            print("phase_state: no active task", file=sys.stderr)
            return 1
        if done:
            if completions(state, "reviewer") == 0:
                print(
                    "phase_state: REFUSING close --done — implement-task-reviewer has no "
                    "recorded completion. Spawn the reviewer, or close --abandoned with "
                    "explicit user approval.",
                    file=sys.stderr,
                )
                return 1
            state["status"] = "closed_done"
        else:
            quote = _flag(args, "--approved-by-user")
            if not quote:
                print("phase_state: --abandoned requires --approved-by-user \"<quote>\"",
                      file=sys.stderr)
                return 1
            state["status"] = "closed_abandoned"
            add_event(state, "abandoned", approved_by_user=quote)
        add_event(state, "closed", status=state["status"])
        metrics = {
            "slug": state["slug"],
            "lane": state.get("lane", "hard"),
            "branch": state["branch"],
            "created_ts": state["created_ts"],
            "closed_ts": now(),
            "verdict": state["status"],
            "phases": phase_metrics(state),
            "stop_blocks": sum(1 for e in state["events"] if e["event"] == "stop_block"),
            "order_denies": sum(1 for e in state["events"] if e["event"] == "order_deny"),
            "escalations": [
                {"from": e.get("frm"), "to": e.get("to"), "reason": e.get("reason")}
                for e in state["events"] if e["event"] == "escalate"
            ],
            "block_events": sum(1 for e in state["events"] if e["event"] in
                                ("blocked", "blocked_by_marker")),
        }
        try:
            METRICS.parent.mkdir(parents=True, exist_ok=True)
            with METRICS.open("a", encoding="utf-8") as fh:
                fh.write(json.dumps(metrics, ensure_ascii=False) + "\n")
        except OSError as exc:
            print(f"phase_state: metrics append failed: {exc}", file=sys.stderr)
        archive(state)
    print(f"phase_state: closed '{state['slug']}' as {state['status']}; "
          f"metrics appended to {METRICS.relative_to(REPO)}")
    return 0


def cli_status(_args: list[str]) -> int:
    state = load_state()
    if state is None:
        print("phase_state: no active task")
        return 0
    print(json.dumps(state, indent=1, ensure_ascii=False))
    return 0


def _flag(args: list[str], name: str) -> str | None:
    try:
        i = args.index(name)
        return args[i + 1]
    except (ValueError, IndexError):
        return None


def main() -> int:
    argv = sys.argv[1:]
    if not argv:
        print(__doc__, file=sys.stderr)
        return 1
    cmd, rest = argv[0], argv[1:]
    if cmd == "hook":
        mode = rest[0] if rest else ""
        fn = HOOK_MODES.get(mode)
        if fn is None:
            print(f"phase_state: unknown hook mode '{mode}'", file=sys.stderr)
            return 0  # fail open
        try:
            payload = json.load(sys.stdin)
        except Exception as exc:  # noqa: BLE001
            print(f"phase_state: unreadable payload ({exc}); allowing", file=sys.stderr)
            return 0
        try:
            return fn(payload)
        except Exception as exc:  # noqa: BLE001 — hooks must fail open
            print(f"phase_state: {mode} error ({exc}); allowing", file=sys.stderr)
            audit({"mode": mode, "error": str(exc)})
            return 0
    cli = {"start": cli_start, "block": cli_block, "skip-gate": cli_skip_gate,
           "escalate": cli_escalate, "close": cli_close, "status": cli_status}.get(cmd)
    if cli is None:
        print(__doc__, file=sys.stderr)
        return 1
    return cli(rest)


if __name__ == "__main__":
    sys.exit(main())
