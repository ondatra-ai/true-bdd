#!/usr/bin/env python3
"""PreToolUse write-guard for crush runs driven by the test-author + fixer agents.

Crush's `run` mode has NO permission gate — this hook is the ONLY enforcement.
The invoking wrapper (.claude/scripts/crush-run.sh) sets
CRUSH_GUARD_ROLE to select the sandbox:

  author — pins findings as e2e specs: file writes ONLY under tests/harness/
           (plus the tmp/crush/ scratch area).
  fixer  — task-blind production fixer: file writes ONLY under harness/
           (plus tmp/crush/).

No role (or an unknown one) fails CLOSED: every write and every bash command
is denied, so a bare `crush run` outside the wrapper stays read-only.

Policy:
  - File-writing tools (edit/write/multiedit/patch/create/download): allowed
    only under the role's write roots.
  - bash: anchored full-command whitelist per role — test/typecheck runs plus
    simple read-only commands. Everything else is denied so shell can't bypass
    the file-tool path rule. The only sanctioned redirect writes chatty test
    output to tmp/crush/*.log (crush's shell deadlocks on large child output).
  - Every other tool is DEFAULT-DENIED unless it is on the read-only allowlist
    (the `download` tool writes files and was nearly missed once — unknown
    tools are treated as writers until proven otherwise).
  - Everything is logged (payload + verdict) to tmp/crush/hook.log.

Exit codes per crush hooks contract: 0 = allow, 2 = deny (stderr = reason).
"""

import json
import os
import re
import sys

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
LOG = os.path.join(REPO, "tmp", "crush", "hook.log")
SCRATCH_ROOT = os.path.join(REPO, "tmp", "crush") + os.sep

FILE_TOOLS = {"edit", "write", "multiedit", "patch", "create", "download"}

# Read-only tools crush may always use. Anything not listed here and not a
# file tool / bash is denied — default-deny, never enumerate-dangerous.
READONLY_TOOLS = {"view", "ls", "grep", "glob", "sourcegraph", "diagnostics", "fetch"}
READONLY_TOOL_PREFIXES = ("mcp_context7", "context7")

_R = re.escape(REPO)
_LOG_REDIRECT = rf"( > {_R}/tmp/crush/[\w.-]+\.log 2>&1)?"
_ENV_PREFIX = r"([A-Z][A-Z0-9_]*=[\w.-]+ )*"  # e.g. TRUE_BDD_E2E_KEEP_STACK=1

# Anchored full-command whitelists. Patterns are full-match regexes whose
# character classes exclude ; | & < > ` $( except where explicitly written,
# so compound/redirect syntax can't smuggle a write past the path rule.
BASH_SHARED = [
    rf"cd {_R}/tests/harness && {_ENV_PREFIX}npx playwright test[ \w./=-]*{_LOG_REDIRECT}",
    rf"cd {_R}/tests/harness && npm run [\w:.-]+{_LOG_REDIRECT}",
    rf"{_ENV_PREFIX}npx playwright test[ \w./=-]*",
    r"(ls|cat|head|tail|wc|grep|rg|find|tree|pwd|node --version|npm --version|git status|git diff[ \w./=-]*)[ \w./\"'*=-]*",
]
BASH_BY_ROLE = {
    "author": [
        rf"cd {_R}/tests/harness && npx tsc --noEmit( -p [\w./-]+)?",
    ],
    "fixer": [
        rf"cd {_R}/harness && npx tsc( --noEmit)?( -p [\w./-]+)?",
        rf"cd {_R}/harness && npx next build",
        rf"cd {_R}/harness && npm run [\w:.-]+{_LOG_REDIRECT}",
        rf"cd {_R}/harness && npm install[ \w@^~:./=-]*",
    ],
}
WRITE_ROOT_BY_ROLE = {
    "author": os.path.join(REPO, "tests", "harness") + os.sep,
    "fixer": os.path.join(REPO, "harness") + os.sep,
}


def log(entry):
    try:
        os.makedirs(os.path.dirname(LOG), exist_ok=True)
        with open(LOG, "a", encoding="utf-8") as f:
            f.write(json.dumps(entry) + "\n")
    except OSError:
        pass


def deny(payload, reason, role):
    log({"verdict": "DENY", "role": role, "reason": reason, "payload": payload})
    print(reason, file=sys.stderr)
    sys.exit(2)


def allow(role, **fields):
    log({"verdict": "ALLOW", "role": role, **fields})
    sys.exit(0)


def main():
    raw = sys.stdin.read()
    role = os.environ.get("CRUSH_GUARD_ROLE", "")
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        # Fail closed: an unreadable payload gets no write access.
        log({"verdict": "DENY", "role": role, "note": "unparseable payload", "raw": raw[:2000]})
        print("guard: unparseable hook payload; denying", file=sys.stderr)
        sys.exit(2)

    tool = payload.get("tool_name", "")
    tool_input = payload.get("tool_input") or {}

    if role not in WRITE_ROOT_BY_ROLE:
        # No sanctioned role: read-only tools still work, everything else dies.
        if tool in READONLY_TOOLS or tool.startswith(READONLY_TOOL_PREFIXES):
            allow(role, tool=tool, note="read-only, roleless")
        deny(
            payload,
            "guard: CRUSH_GUARD_ROLE is not set (or unknown) — this repo only permits "
            "crush writes via .claude/scripts/crush-run.sh, which "
            "sets the role. Read-only tools remain available.",
            role,
        )

    write_roots = (WRITE_ROOT_BY_ROLE[role], SCRATCH_ROOT)

    if tool in FILE_TOOLS:
        candidates = [
            v
            for k, v in tool_input.items()
            if isinstance(v, str)
            and ("path" in k.lower() or "file" in k.lower())
        ]
        if not candidates:
            deny(payload, "file tool call carried no recognizable path field; refusing", role)
        for p in candidates:
            resolved = os.path.realpath(
                p if os.path.isabs(p) else os.path.join(payload.get("cwd", REPO), p)
            )
            ok = any(
                (resolved + os.sep).startswith(root) or resolved == root.rstrip(os.sep)
                for root in write_roots
            )
            if not ok:
                deny(
                    payload,
                    f"write blocked: {resolved} is outside your sandbox — the '{role}' role "
                    f"may write ONLY under {write_roots[0]} (and tmp/crush/ scratch). "
                    "Everything else is read-only for you.",
                    role,
                )
        allow(role, tool=tool, paths=candidates)

    if tool == "bash":
        cmd = (tool_input.get("command") or "").strip()
        for pattern in BASH_SHARED + BASH_BY_ROLE[role]:
            if re.fullmatch(pattern, cmd):
                allow(role, tool="bash", cmd=cmd)
        deny(
            payload,
            f"bash blocked for role '{role}': only the whitelisted test/typecheck runs and "
            "simple read-only commands (ls/cat/grep/rg/find/head/tail) are allowed, as plain "
            "commands without ; | & or redirects (sole exception: '> "
            + os.path.join("tmp", "crush", "<name>.log")
            + " 2>&1' on a test run). File changes must go through edit/write tools inside "
            "your sandbox.",
            role,
        )

    if tool in READONLY_TOOLS or tool.startswith(READONLY_TOOL_PREFIXES):
        allow(role, tool=tool)

    deny(
        payload,
        f"tool '{tool}' is not on this sandbox's allowlist (default-deny: unknown tools are "
        "treated as writers). Use view/ls/grep/glob for reading, edit/write under your "
        "sandbox root for changes, and whitelisted bash for test runs.",
        role,
    )


if __name__ == "__main__":
    main()
