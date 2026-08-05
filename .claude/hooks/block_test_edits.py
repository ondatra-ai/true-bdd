#!/usr/bin/env python3
"""PreToolUse hook: stop the implement-task-test-fixer agent from editing test files.

Wired into the `implement-task-test-fixer` agent's frontmatter `hooks.PreToolUse`
only, so it runs solely while that agent is active — the test-author and reviewer
agents (which legitimately edit tests) are unaffected. Enforced in every permission
mode, including bypassPermissions.

Two surfaces are guarded:
  * File-edit tools (Write/Edit/MultiEdit/NotebookEdit) — denied when the target
    path is under an off-limits location.
  * Bash — denied when an off-limits path is the *target* of a write: a redirect,
    `tee`, a mutating command (`sed -i`/`rm`/`mv`/`cp`/`touch`/`git restore`…), an
    in-place interpreter (`python -c`/`node -e`/…), or a snapshot-update flag
    (`playwright -u`/`--update-snapshots`). Running the tests is NOT blocked.

This Bash guard is BEST-EFFORT defense-in-depth — a determined command can evade it
(variable indirection, obfuscation, transient edit + restore). The real guarantee is
the orchestrator hashing the off-limits tree before/after the test-fixer and stopping
on any change. Matching is deliberately conservative: a rare benign command that names
a mutating verb and a test path may be denied; the agent can rephrase or escalate.

The off-limits path patterns live in the `off_limits_test_fixer.paths` list of
docs/context/paths.yaml — edit them there, not here. Missing config or unreadable
stdin fails OPEN (allows) with a loud warning; the test-fixer's prompt prohibition
and the orchestrator snapshot remain as backstops (the Bash guard cannot run without
the patterns).

Input  (stdin JSON): {"tool_name": "...", "tool_input": {"file_path"|"command": "...", ...}}
Output (stdout JSON when denying):
  {"hookSpecificOutput": {"hookEventName": "PreToolUse",
                          "permissionDecision": "deny",
                          "permissionDecisionReason": "..."}}
Every guarded invocation (allow/deny/warn) is appended, best-effort, as JSONL to
tmp/block_test_edits.log with a UTC timestamp, session id, tool, target/digest, rule.
A human-readable line is also written to stderr on deny/warn.
"""

import hashlib
import json
import os
import re
import sys
from datetime import datetime, timezone
from pathlib import Path

REPO = Path(
    os.environ.get("CLAUDE_PROJECT_DIR") or Path(__file__).resolve().parents[2]
).resolve()
PATHS_FILE = REPO / "docs" / "context" / "paths.yaml"
AUDIT_LOG = REPO / "tmp" / "block_test_edits.log"
OFFLIMITS_KEY = "off_limits_test_fixer"
FILE_EDIT_TOOLS = {"Write", "Edit", "MultiEdit", "NotebookEdit"}

# A write operator that, immediately followed by an off-limits path token, means the
# command targets that path for mutation. Named for the audit log.
WRITE_RULES = [
    ("redirect", r">>?\s*[\"']?"),
    ("tee", r"\btee\b\s+(?:-\S+\s+)*[\"']?"),
    ("cp/mv/install", r"\b(?:cp|mv|install)\s+(?:-\S+\s+)*(?:\S+\s+)+[\"']?"),
    ("rm/touch", r"\b(?:rm|touch|truncate)\s+(?:-\S+\s+)*[\"']?"),
    ("sed/perl -i", r"\b(?:sed|perl)\b[^|;]*?(?:-i|--in-place)[^|;]*?[\"']?"),
    ("git restore/checkout/rm", r"\bgit\s+(?:checkout|restore|rm)\b[^|;]*?[\"']?"),
    ("dd of=", r"\bdd\b[^|;]*?of=[\"']?"),
    ("interpreter -c/-e", r"\b(?:python[0-9.]*|node|bun|deno|ruby|php)\b[^|;]*?(?:-c|-e|--command)\b[^|;]*?[\"']?"),
]
SNAPSHOT_UPDATE_RE = re.compile(
    r"\b(?:playwright|vitest|jest|pytest|py\.test)\b[^|;]*?(?:-u|--update-snapshots)\b"
)
# Applying an external diff can modify tests off-path (the target lives in the patch
# file, not the command), so deny these outright for the test-fixer.
PATCH_RE = re.compile(r"\bgit\s+apply\b|(?:^|[;&|]\s*)patch\b")


def audit(record):
    """Best-effort JSONL append to the audit log; never raises."""
    try:
        AUDIT_LOG.parent.mkdir(parents=True, exist_ok=True)
        with AUDIT_LOG.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps(record, default=str) + "\n")
    except OSError:
        pass


def load_offlimits():
    """Return path patterns from paths.yaml's off_limits_test_fixer.paths list."""
    try:
        text = PATHS_FILE.read_text(encoding="utf-8")
    except OSError:
        return None
    try:
        import yaml  # PyYAML — present in the dev environment

        entry = (yaml.safe_load(text) or {}).get(OFFLIMITS_KEY) or {}
        patterns = [str(p).strip() for p in entry.get("paths") or [] if str(p).strip()]
        return patterns or None
    except Exception:  # noqa: BLE001 — fall through to the dependency-free parse
        pass
    # Fallback (no PyYAML / parse error): indentation scan for the entry's list.
    patterns, in_entry, in_paths = [], False, False
    for ln in text.splitlines():
        if not ln.strip() or ln.lstrip().startswith("#"):
            continue
        indented = ln[0] in " \t"
        if not indented:
            in_entry = ln.startswith(OFFLIMITS_KEY + ":")
            in_paths = False
            continue
        if not in_entry:
            continue
        s = ln.strip()
        if s.startswith("paths:"):
            in_paths = True
        elif in_paths and s.startswith("- "):
            patterns.append(s[2:].strip().strip("'\""))
        elif in_paths:
            in_paths = False
    return patterns or None


def _path_tokens(pattern):
    """Regexes matching `pattern` as a real path token (with boundaries).

    Directory patterns keep their trailing slash (so `harness/src/tests/` does NOT
    match `harness/src/tests-results`); file patterns require a boundary after. A leading
    boundary (not preceded by a word/dot/dash) prevents matching inside a longer name.
    Forms: repo-relative, `./`-prefixed, and absolute. File patterns ALSO match their
    bare basename (at start or after whitespace/quote) so `cd <parent> && > <basename>`
    is caught — directory `cd`-relative writes can't be matched this way and rely on
    the orchestrator's before/after snapshot instead.
    """
    raw = pattern.strip()
    if not raw:
        return []
    is_dir = raw.endswith("/")
    out = []
    for form in (raw, "./" + raw, str(REPO / raw)):
        esc = re.escape(form)
        if is_dir:
            out.append(rf"(?<![\w.\-]){esc}")
        else:
            out.append(rf"(?<![\w.\-]){esc}(?![\w./\-])")
    if not is_dir:
        base = raw.split("/")[-1]
        if base and base != raw:
            out.append(rf"(?<![\w./\-]){re.escape(base)}(?![\w./\-])")
    return out


def file_matches(repo_rel, patterns):
    """Prefix match: pattern 'a/b' matches 'a/b' and anything under 'a/b/'."""
    rel = repo_rel.replace("\\", "/").strip("/")
    for p in patterns:
        base = p.strip().strip("/")
        if not base:
            continue
        if rel == base or rel.startswith(base + "/"):
            return p
    return None


def bash_write_target(command, patterns):
    """Return (pattern, rule_name) if the command writes to an off-limits path."""
    if PATCH_RE.search(command):
        return "patch-application", "git apply/patch"
    for p in patterns:
        toks = _path_tokens(p)
        for rule_name, rule in WRITE_RULES:
            for tok in toks:
                if re.search(rule + tok, command):
                    return p, rule_name
    if SNAPSHOT_UPDATE_RE.search(command):
        for p in patterns:
            for tok in _path_tokens(p):
                if re.search(tok, command):
                    return p, "snapshot-update"
    return None


def now_iso():
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def deny(tool, target, rule, extra=None):
    reason = (
        f"implement-task-test-fixer must not modify test files: {tool} target '{target}' "
        f"is off-limits (matched '{rule}' in docs/context/paths.yaml). Escalate to the "
        "test-author via the orchestrator instead of editing tests."
    )
    sys.stderr.write("block_test_edits: DENY — " + reason + "\n")
    sys.stdout.write(
        json.dumps(
            {
                "hookSpecificOutput": {
                    "hookEventName": "PreToolUse",
                    "permissionDecision": "deny",
                    "permissionDecisionReason": reason,
                }
            }
        )
    )
    rec = {"ts": now_iso(), "decision": "deny", "tool": tool, "target": target, "rule": rule}
    if extra:
        rec.update(extra)
    audit(rec)
    sys.exit(0)


def allow(record):
    record.setdefault("ts", now_iso())
    record["decision"] = "allow"
    audit(record)
    sys.exit(0)


def warn_then_allow(msg):
    sys.stderr.write(f"block_test_edits: {msg}\n")
    audit({"ts": now_iso(), "decision": "warn", "message": msg})
    sys.exit(0)


def main():
    try:
        payload = json.load(sys.stdin)
    except Exception as e:
        warn_then_allow(f"unreadable stdin ({e}); allowing")

    tool = payload.get("tool_name")
    base = {"ts": now_iso(), "session": payload.get("session_id"), "cwd": payload.get("cwd"), "tool": tool}

    if tool not in FILE_EDIT_TOOLS and tool != "Bash":
        allow({**base, "target": None, "rule": None, "note": "not guarded"})

    patterns = load_offlimits()
    if not patterns:
        warn_then_allow(
            f"off-limits block not found in {PATHS_FILE.relative_to(REPO)}; failing open "
            "(only the test-fixer prompt prohibition and the orchestrator snapshot remain; "
            "the Bash guard is inactive without patterns)"
        )

    tool_input = payload.get("tool_input") or {}

    if tool in FILE_EDIT_TOOLS:
        target = tool_input.get("notebook_path" if tool == "NotebookEdit" else "file_path")
        if not target:
            allow({**base, "target": None, "rule": None})
        try:
            rel = Path(target).resolve().relative_to(REPO).as_posix()
        except ValueError:
            allow({**base, "target": target, "rule": "outside-repo"})
        hit = file_matches(rel, patterns)
        if hit:
            deny(tool, rel, hit, extra={"session": base["session"], "cwd": base["cwd"]})
        allow({**base, "target": rel, "rule": None})

    # Bash
    command = tool_input.get("command") or ""
    digest = hashlib.sha256(command.encode("utf-8", "replace")).hexdigest()[:12]
    if not command:
        allow({**base, "target": None, "rule": None, "digest": digest})
    hit = bash_write_target(command, patterns)
    if hit:
        pattern, rule = hit
        deny("Bash", pattern, rule, extra={"session": base["session"], "cwd": base["cwd"], "digest": digest})
    allow({**base, "target": None, "rule": None, "digest": digest})


if __name__ == "__main__":
    main()
