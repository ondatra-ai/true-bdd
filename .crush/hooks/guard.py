#!/usr/bin/env python3
"""PreToolUse guard for the crush-as-test-fixer experiment.

Policy (mirrors the repo's task-blind fixer contract):
  - File-writing tools (edit/write/multiedit/...): allowed ONLY under <repo>/harness/.
  - bash: allowed only for an anchored whitelist — playwright test runs from
    tests/harness, npm test scripts, and simple read-only commands. Everything
    else is denied so shell can't be used to bypass the file-tool path rule.
  - Everything is logged (payload + verdict) to hook.log for the experiment report.

Exit codes per crush hooks contract: 0 = no opinion / allow, 2 = deny (stderr = reason).
"""

import json
import os
import re
import sys

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
ALLOWED_WRITE_ROOT = os.path.join(REPO, "harness") + os.sep
LOG = os.path.join(REPO, "tmp", "crush-experiment", "hook.log")

FILE_TOOLS = {"edit", "write", "multiedit", "patch", "create", "download"}

# Anchored full-command whitelist for bash. No ; | & > < ` $( allowed anywhere,
# except the single sanctioned redirect shape: playwright output into a log
# file under harness/ (crush's writable sandbox) — its shell deadlocks on
# large child output, so big runs must go to a file it then reads.
_R = re.escape(REPO)
BASH_ALLOWED = [
    rf"cd {_R}/tests/harness && npx playwright test[ \w./=-]*( > {_R}/harness/[\w.-]+\.log 2>&1)?",
    rf"cd {_R}/tests/harness && npm run [\w:.-]+",
    rf"cd {_R}/harness && npx tsc( --noEmit)?( -p [\w./-]+)?",
    rf"cd {_R}/harness && npx next build",
    r"npx playwright test[ \w./=-]*",
    r"(ls|cat|head|tail|wc|grep|rg|find|tree|pwd|node --version|npm --version|git status|git diff[ \w./=-]*)[ \w./\"'*=-]*",
]
BASH_FORBIDDEN_CHARS = re.compile(r"[;|&<>`]|\$\(")


def log(entry):
    try:
        os.makedirs(os.path.dirname(LOG), exist_ok=True)
        with open(LOG, "a", encoding="utf-8") as f:
            f.write(json.dumps(entry) + "\n")
    except OSError:
        pass


def deny(payload, reason):
    log({"verdict": "DENY", "reason": reason, "payload": payload})
    print(reason, file=sys.stderr)
    sys.exit(2)


def main():
    raw = sys.stdin.read()
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        log({"verdict": "PASS", "note": "unparseable payload", "raw": raw[:2000]})
        sys.exit(0)

    tool = payload.get("tool_name", "")
    tool_input = payload.get("tool_input") or {}

    if tool in FILE_TOOLS:
        # Find every path-looking field the tool was given.
        candidates = [
            v
            for k, v in tool_input.items()
            if isinstance(v, str)
            and ("path" in k.lower() or "file" in k.lower())
        ]
        if not candidates:
            deny(payload, "file tool call carried no recognizable path field; refusing")
        for p in candidates:
            resolved = os.path.realpath(
                p if os.path.isabs(p) else os.path.join(payload.get("cwd", REPO), p)
            )
            if not (resolved + os.sep).startswith(ALLOWED_WRITE_ROOT) and resolved != ALLOWED_WRITE_ROOT.rstrip(os.sep):
                deny(
                    payload,
                    f"write blocked: {resolved} is outside {ALLOWED_WRITE_ROOT} — "
                    "this experiment allows file edits ONLY under harness/. "
                    "Everything else is read-only for you.",
                )
        log({"verdict": "ALLOW", "tool": tool, "paths": candidates})
        sys.exit(0)

    if tool == "bash":
        cmd = (tool_input.get("command") or "").strip()
        if BASH_FORBIDDEN_CHARS.search(cmd) and not cmd.startswith(f"cd {REPO}/tests/harness && "):
            deny(payload, "bash blocked: compound/redirect shell syntax is not allowed in this experiment. Use plain commands; use edit/write tools (under harness/ only) for file changes.")
        for pattern in BASH_ALLOWED:
            if re.fullmatch(pattern, cmd):
                log({"verdict": "ALLOW", "tool": "bash", "cmd": cmd})
                sys.exit(0)
        deny(
            payload,
            "bash blocked: only playwright test runs (cd .../tests/harness && npx playwright test ...) "
            "and simple read-only commands (ls/cat/grep/rg/find/head/tail) are allowed. "
            "File changes must go through edit/write tools under harness/.",
        )

    # All other tools (view/ls/grep/glob/lsp/mcp/...) — no opinion, normal flow.
    log({"verdict": "PASS", "tool": tool})
    sys.exit(0)


if __name__ == "__main__":
    main()
