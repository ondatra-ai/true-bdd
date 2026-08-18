#!/usr/bin/env python3
"""Collect every review finding, score it, and band it — deterministically.

The loop this replaces took four rounds and two hours on PR #76, and the
ledger says why: 13 of its 16 findings were prose inconsistencies in one
skill document, each one created by the previous round's fix. Every finding
was triaged at the same weight and every one was fixed inline, so each round
authored the next round's findings.

What is deterministic lives here; only two things are not, and both are
genuinely judgement:

  * `score`  — one `claude -p` call rates each finding 1-10 on consequence.
  * the fix — written by the agent, from `fix-queue.json`.

Everything else — collecting from three different shapes of CodeRabbit
output, deduping, banding, rendering the table, deciding whether a round is
allowed to happen at all — is mechanical, and mechanical is what a script
should own.

Subcommands:
  collect --pr N [--local]   gather findings -> tmp/merge/findings.json
  score                      rate 1-10        -> tmp/merge/scored.json
  band                       split + render   -> fix/defer/ignore queues
"""

import argparse
import json
import os
import re
import subprocess
import sys

MERGE_DIR = "tmp/merge"
FINDINGS = f"{MERGE_DIR}/findings.json"
SCORED = f"{MERGE_DIR}/scored.json"
FIX_QUEUE = f"{MERGE_DIR}/fix-queue.json"
DEFER_QUEUE = f"{MERGE_DIR}/defer-queue.json"
IGNORE_QUEUE = f"{MERGE_DIR}/ignore-queue.json"

HERE = os.path.dirname(os.path.abspath(__file__))

# The bands. A finding is worth a round-trip through the fix loop only when
# getting it wrong changes what the software does; everything real but
# bounded becomes a ticket, and everything else is recorded and dropped.
FIX_FLOOR = 9
DEFER_FLOOR = 6

SCORE_TIMEOUT = int(os.environ.get("MERGE_SCORE_TIMEOUT", "900"))

RUBRIC = """\
Score each finding 1-10 by CONSEQUENCE IF LEFT UNFIXED — not by how easy it
is to fix, and not by the severity label the reviewer attached.

  9-10  Wrong behaviour reachable by a real invocation. Data loss, silent
        failure, a crash, a security hole, a check that reports success
        without having checked, or a script that aborts with no diagnostic.
        "A user or CI run would hit this and be misled or blocked."
  6-8   Real, but bounded: an inconsistency between two instructions, a
        wrong path in an error message, a missing guard on an input that
        does not occur today, a stale example. Worth doing, not worth
        blocking a merge.
  1-5   Style, wording, ordering, naming, prose taste, heading structure,
        or a suggestion that is a preference rather than a defect.

Calibration from a real review of this repository:
  - `head -n 1` closing a pipe so `sed` dies of SIGPIPE and `set -o
    pipefail` aborts the commit script with nothing printed  ->  9
  - a GraphQL query truncating at 100 items with no signal, so a count
    prints as a total  ->  9
  - a documented dismissal condition naming one required check when the
    ruleset requires two  ->  7
  - an error message hardcoding a path when `$HERE` holds the right one
    ->  6
  - "every finding gets an owner" contradicting a table that shows `—`
    for rejected findings  ->  6
  - reordering a section, rewording a comment, adding a heading  ->  2

Be skeptical. A reviewer can be wrong about this codebase. If the finding
misreads the code, score it 1 and say so in `reason`.

Return ONLY a JSON array, no prose and no code fence:
[{"id": "<id>", "score": <1-10>, "reason": "<one sentence, code-anchored>"}]
"""


def run(cmd, **kw):
    return subprocess.run(cmd, capture_output=True, text=True, **kw)


def ensure_dir():
    os.makedirs(MERGE_DIR, exist_ok=True)


def load(path, default=None):
    if not os.path.exists(path):
        return default
    with open(path) as handle:
        return json.load(handle)


def save(path, payload):
    ensure_dir()
    with open(path, "w") as handle:
        json.dump(payload, handle, indent=2)


# ------------------------------------------------------------- collection


def collect_local():
    """`coderabbit review --base main` — the bot's scope, run locally.

    This is the round that costs nothing. The GitHub bot reviews the whole
    branch against main; `pr-commit/review.sh` reviews the working tree per
    directory. Those are different questions, which is how a local round
    reported 0 findings on work the bot then raised 5 on. Same scope here,
    before the push, off the PR-review quota entirely.
    """
    result = run(["coderabbit", "review", "--base", "main", "--agent"])
    findings = []
    for line in result.stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if event.get("type") == "finding":
            findings.append(_from_cli(event, len(findings)))
        elif event.get("type") == "complete":
            for item in event.get("findings") or []:
                findings.append(_from_cli(item, len(findings)))
    if result.returncode != 0 and not findings:
        print(
            "coderabbit review exited %d and produced no findings:\n%s"
            % (result.returncode, result.stderr[:500]),
            file=sys.stderr,
        )
    return findings


def _from_cli(event, index):
    claim = (event.get("codegenInstructions") or "").split("\n\n")[-1].strip()
    return {
        # Indexed: two findings on the same file and line range are common,
        # and identical ids make them share one score silently — which reads
        # as a triaged finding and is not one.
        "id": "local:%d:%s:%s" % (index, event.get("fileName", "?"),
                                  event.get("lineRange") or "?"),
        "source": "local",
        "severity": event.get("severity", "?"),
        "file": event.get("fileName", "?"),
        "line": str(event.get("lineRange") or "?"),
        "title": (event.get("description") or claim or "")[:200],
        "body": event.get("description") or claim,
        "comment_id": None,
        "thread_id": None,
    }


# CodeRabbit writes "_🟠 Major_" — the emoji sits INSIDE the underscores, so
# a pattern anchored on `_Major_` matches nothing and every finding comes
# back severity "?". Allow anything non-underscore before the word.
# No trailing \b: `_` is a word character, so "Major_" has no boundary
# between the "r" and the "_", and anchoring on one matches nothing.
SEVERITY_RE = re.compile(r"_[^_]*(Critical|Major|Minor|Trivial)[^_]*_", re.I)

# The finding's own headline is the first **bold** line. Body-only findings
# arrive quote-prefixed ("> "), and the first non-quote line is often a
# rendered sub-block heading ("▸ Proposed change"), which is not the title.
TITLE_RE = re.compile(r"^\s*>?\s*\*\*(.+?)\*\*\s*$", re.M)


def collect_github(pr):
    """Threads and body-only findings, via the renderer that reconciles."""
    result = run(["python3", os.path.join(HERE, "render_review.py"), "list", "--pr", str(pr), "--json"])
    if result.returncode not in (0, 2):
        sys.exit("render_review.py list failed: %s" % result.stderr.strip()[:400])
    if result.returncode == 2:
        sys.exit(
            "render_review.py reported a reconciliation MISMATCH — some finding is "
            "unaccounted for. Fix the parser before triaging.\n" + result.stdout[-600:]
        )
    payload = json.loads(result.stdout or "{}")

    findings = []
    for thread in payload.get("threads", []):
        if thread.get("resolved"):
            continue
        body = thread.get("body", "")
        findings.append(
            {
                "id": "thread:%s" % thread["thread_id"],
                "source": "thread",
                "severity": _severity(body),
                "file": thread.get("path", "?"),
                "line": str(thread.get("line") or "?"),
                "title": _title(body),
                "body": body,
                "comment_id": thread.get("comment_id"),
                "thread_id": thread.get("thread_id"),
            }
        )
    for index, item in enumerate(payload.get("body_only", [])):
        body = item.get("text", "")
        findings.append(
            {
                # Indexed, because several body-only findings share one file
                # and their line range is often unparsed ("?"). Without it
                # they collide, and a collision does not look like an error:
                # every colliding finding silently takes the first one's
                # score and reason.
                "id": "body:%d:%s" % (index, item.get("path", "?")),
                "source": "body-only",
                "severity": _severity(body),
                "file": item.get("path", "?"),
                # "Outside diff range" findings carry their range inside the
                # quoted body ("> `95-95`: …") rather than in the heading.
                "line": str(item.get("lines") or "?") if item.get("lines") not in (None, "?")
                        else _line_from_body(body),
                "title": _title(body),
                "body": body,
                "comment_id": None,
                "thread_id": None,
            }
        )
    return findings


BODY_LINE_RE = re.compile(r"^\s*>?\s*`(\d+(?:-\d+)?)`\s*:", re.M)


def _line_from_body(body):
    match = BODY_LINE_RE.search(body or "")
    return match.group(1) if match else "?"


def _severity(body):
    match = SEVERITY_RE.search(body or "")
    return match.group(1).lower() if match else "?"


def _title(body):
    match = TITLE_RE.search(body or "")
    if match:
        return match.group(1).strip()[:200]
    # No bold headline: fall back to the first line that is prose rather
    # than a severity badge, a quote marker or a rendered sub-heading.
    for line in (body or "").splitlines():
        stripped = line.lstrip("> ").strip()
        if not stripped or stripped.startswith(("_", "`", "|", "▸", "-", "#")):
            continue
        return stripped.strip("*").strip()[:200]
    return (body or "").strip()[:200]


def cmd_collect(args):
    findings = collect_local() if args.local else collect_github(args.pr)

    # Dedupe on file+line+title: the same defect reaches us as a thread and
    # again inside the review body often enough to matter.
    seen, unique = set(), []
    for finding in findings:
        key = (finding["file"], finding["line"], finding["title"][:80])
        if key in seen:
            continue
        seen.add(key)
        unique.append(finding)

    # Ids key the score lookup, so a duplicate makes several findings share
    # one verdict — which reads as a triaged finding and is not one. Caught
    # here rather than in `score`, where it looks like agreement.
    ids = [f["id"] for f in unique]
    dupes = {i for i in ids if ids.count(i) > 1}
    if dupes:
        sys.exit("duplicate finding ids, scoring would collide: %s" % ", ".join(sorted(dupes)))

    save(FINDINGS, unique)
    print("collected %d finding(s) -> %s" % (len(unique), FINDINGS))
    return 0


# ---------------------------------------------------------------- scoring


def cmd_score(args):
    findings = load(FINDINGS, [])
    if not findings:
        save(SCORED, [])
        print("no findings to score")
        return 0

    payload = [
        {"id": f["id"], "file": f["file"], "line": f["line"],
         "reviewer_severity": f["severity"], "finding": f["body"][:2500]}
        for f in findings
    ]
    prompt = RUBRIC + "\n\nFindings:\n" + json.dumps(payload, indent=2)

    env = dict(os.environ, CLAUDE_HISTORY_ROLE="merge-triage")
    env.pop("CLAUDECODE", None)
    try:
        result = subprocess.run(
            ["claude", "-p", prompt], capture_output=True, text=True, env=env,
            timeout=SCORE_TIMEOUT,
        )
    except subprocess.TimeoutExpired:
        sys.exit("claude -p (scoring) timed out after %ds — a stalled call must fail "
                 "loudly, not leave every finding unscored and unbanded." % SCORE_TIMEOUT)
    if result.returncode != 0:
        sys.exit("claude -p (scoring) failed with %d:\n%s" % (result.returncode, result.stderr[:500]))

    scores = _parse_scores(result.stdout)
    by_id = {s["id"]: s for s in scores}
    missing = [f["id"] for f in findings if f["id"] not in by_id]
    if missing:
        # Never silently drop: an unscored finding would vanish from every
        # band, which is the exact failure this whole change exists to stop.
        sys.exit(
            "scoring returned no verdict for %d finding(s): %s\nRe-run `triage.py score`."
            % (len(missing), ", ".join(missing[:5]))
        )

    for finding in findings:
        # A score outside 1-10, or one that is not a number, would band the
        # finding somewhere nobody intended — an out-of-range value lands in
        # "ignore" and the finding is dropped without anyone deciding to.
        raw = by_id[finding["id"]].get("score")
        try:
            score = int(raw)
        except (TypeError, ValueError):
            sys.exit("scoring returned a non-numeric score %r for %s" % (raw, finding["id"]))
        if not 1 <= score <= 10:
            sys.exit("scoring returned %d for %s — outside 1-10" % (score, finding["id"]))
        finding["score"] = score
        finding["reason"] = by_id[finding["id"]].get("reason", "")

    save(SCORED, findings)
    print("scored %d finding(s) -> %s" % (len(findings), SCORED))
    return 0


def _parse_scores(text):
    text = re.sub(r"^\s*```(?:json)?\s*|\s*```\s*$", "", text.strip(), flags=re.M)
    start, end = text.find("["), text.rfind("]")
    if start == -1 or end == -1:
        sys.exit("scoring produced no JSON array:\n%s" % text[:500])
    try:
        return json.loads(text[start : end + 1])
    except json.JSONDecodeError as error:
        sys.exit("scoring produced invalid JSON (%s):\n%s" % (error, text[:500]))


# ---------------------------------------------------------------- banding


def cmd_band(args):
    findings = load(SCORED, [])
    fix = [f for f in findings if f["score"] >= FIX_FLOOR]
    defer = [f for f in findings if DEFER_FLOOR <= f["score"] < FIX_FLOOR]
    ignore = [f for f in findings if f["score"] < DEFER_FLOOR]

    # A terminal round fixes nothing: everything outstanding becomes a
    # ticket. Two rounds of fixing is the cap because the free-tier quota is
    # ~4 PR reviews an hour and each push spends one.
    if args.terminal:
        defer = fix + defer
        fix = []

    save(FIX_QUEUE, fix)
    save(DEFER_QUEUE, defer)
    save(IGNORE_QUEUE, ignore)

    print("| # | Score | Band | Finding | File:line | Source | Why |")
    print("|---|-------|------|---------|-----------|--------|-----|")
    rows = (
        [("Fix now", f) for f in sorted(fix, key=lambda x: -x["score"])]
        + [("Defer", f) for f in sorted(defer, key=lambda x: -x["score"])]
        + [("Ignore", f) for f in sorted(ignore, key=lambda x: -x["score"])]
    )
    for index, (band, finding) in enumerate(rows, 1):
        print(
            "| %d | %d | **%s** | %s | `%s:%s` | %s | %s |"
            % (
                index,
                finding["score"],
                band,
                finding["title"].replace("|", "\\|")[:110],
                finding["file"],
                finding["line"],
                finding["source"],
                finding["reason"].replace("|", "\\|")[:160],
            )
        )
    print()
    print(
        "**%d fix now, %d deferred, %d ignored** (%d total)%s"
        % (len(fix), len(defer), len(ignore), len(findings),
           "  — terminal round, nothing is fixed here" if args.terminal else "")
    )
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)

    collect = sub.add_parser("collect")
    collect.add_argument("--pr")
    collect.add_argument("--local", action="store_true",
                         help="run coderabbit locally instead of reading the PR")
    collect.set_defaults(func=cmd_collect)

    sub.add_parser("score").set_defaults(func=cmd_score)

    band = sub.add_parser("band")
    band.add_argument("--terminal", action="store_true",
                      help="fix nothing; defer everything outstanding")
    band.set_defaults(func=cmd_band)

    args = parser.parse_args()
    sys.exit(args.func(args))


if __name__ == "__main__":
    main()
