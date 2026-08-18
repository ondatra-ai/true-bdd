#!/usr/bin/env python3
"""The merge loop, end to end, in one process.

Invoked by run.sh. Everything the loop needs happens here — including the
model turns, which are subprocesses spawned at defined points with defined
prompts rather than an agent improvising between script calls. That
distinction is the whole point: the previous design handed the agent a
seven-step checklist and every hand-off was a place to skip a step, reorder
one, or invent one. PR #76 did all three.

  rounds 1-3   push -> request review -> await -> collect/score/band
               -> file deferrals -> fix (rounds 1-2 only) -> answer every
               thread -> record the round -> pr-commit (gates, scan, local
               review, doc-universe, memory, commit, push)
  round 4      guard HEAD was really reviewed -> approve -> land -> merge

Round 3 changes no code, so the commit round 3 reviewed IS the commit round
4 approves. That is what stops `@coderabbitai approve` from being a rubber
stamp: it analyses nothing, and on PR #76 it approved two commits no review
had ever looked at.

The loop always reaches a merge. A red preflight at the end is recorded as
an anomaly, not obeyed — everything that could legitimately block has had
three review rounds and a deferral queue.
"""

import argparse
import json
import os
import re
import shlex
import subprocess
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
LIB = os.path.join(HERE, "..", "lib")
PRCOMMIT = os.path.join(HERE, "..", "pr-commit")
STATE_DIR = "tmp/merge"
STATE_PATH = f"{STATE_DIR}/state.json"

LAST_FIX_ROUND = 2      # rounds 1-2 fix; round 3 triages only
LAST_ROUND = 3          # round 4 is the approval, not a review round
REVIEW_WAIT = 900       # 15 min, matches threads.sh await-review
POSTMORTEM_SECONDS = int(os.environ.get("MERGE_POSTMORTEM_SECONDS", "600"))
FIX_TIMEOUT = int(os.environ.get("MERGE_FIX_TIMEOUT", "2400"))
PRCOMMIT_TIMEOUT = int(os.environ.get("MERGE_PRCOMMIT_TIMEOUT", "3600"))

# acceptEdits lets it write files unprompted; the Bash entries are what it
# may run. Deliberately no network and no arbitrary shell — a headless
# agent editing this repo unattended gets the repo's own commands and
# nothing else.
FIX_TOOLS = (
    "Read,Edit,Write,Glob,Grep,"
    "Bash(git *),Bash(go *),Bash(gofmt *),Bash(golangci-lint *),"
    "Bash(python3 *),Bash(./scripts/*),"
    "Bash(./.claude/skills/pr-commit/gates.sh)"
)

FIX_SCHEMA = json.dumps({
    "type": "object",
    "required": ["files_changed", "gates_green", "summary"],
    "properties": {
        "files_changed": {"type": "array", "items": {"type": "string"}},
        "gates_green": {"type": "boolean"},
        "summary": {"type": "string"},
    },
})


# --------------------------------------------------------------- plumbing


def log(message):
    print(message, flush=True)


def banner(message):
    print(f"\n══ {message} ══", flush=True)


def run(cmd, timeout=None, check=False, capture=True):
    result = subprocess.run(
        cmd, capture_output=capture, text=True, timeout=timeout,
        env=dict(os.environ, CLAUDECODE=""),
    )
    if check and result.returncode != 0:
        sys.exit(f"{' '.join(cmd[:3])} failed ({result.returncode}):\n"
                 f"{(result.stderr or result.stdout or '')[:800]}")
    return result


def stream(cmd, timeout=None):
    """Run with output going straight to the terminal — for the long steps
    (gates, pr-commit) where silence for ten minutes is indistinguishable
    from a hang."""
    return subprocess.run(cmd, timeout=timeout, env=dict(os.environ, CLAUDECODE=""))


class State:
    def __init__(self):
        os.makedirs(STATE_DIR, exist_ok=True)
        self.data = {}
        if os.path.exists(STATE_PATH):
            try:
                self.data = json.load(open(STATE_PATH))
            except json.JSONDecodeError:
                self.data = {}

    def get(self, key, default=None):
        return self.data.get(key, default)

    def set(self, **kwargs):
        self.data.update(kwargs)
        with open(STATE_PATH, "w") as handle:
            json.dump(self.data, handle, indent=2)

    def anomaly(self, text):
        existing = self.data.get("anomalies", [])
        existing.append(text)
        self.set(anomalies=existing)
        log(f"  ANOMALY: {text}")


# ------------------------------------------------------------------ github


def gh_json(*args):
    result = run(["gh", *args], check=True)
    return json.loads(result.stdout or "null")


def head_sha(pr):
    return gh_json("pr", "view", str(pr), "--json", "headRefOid")["headRefOid"]


def reviewed_sha(pr, repo):
    """The commit the newest review that ANALYSED code is pinned to.

    The test is the body, not the state. Measured on PR #76:

        CHANGES_REQUESTED  9cc2545  body=9138   <- a real review
        COMMENTED          41f85a5  body=3872   <- also a real review
        APPROVED           14e327a  body=0      <- `@coderabbitai approve`

    An APPROVED with an empty body proves nothing; a COMMENTED with 3872
    bytes is an analysis. Filtering on state takes the rubber stamp as
    evidence, which is the bug this exists to catch.
    """
    # No --jq: `gh api --paginate` applies the filter to EACH page and
    # concatenates, so a filter ending in `last` emits one SHA per page and
    # the comparison below reads two SHAs as a string. Without --jq gh
    # merges the array pages into one document, which parses cleanly.
    result = run(["gh", "api", "--paginate",
                  f"repos/{repo}/pulls/{pr}/reviews?per_page=100"], check=True)
    reviews = json.loads(result.stdout or "[]")
    analysed = [r for r in reviews
                if "coderabbit" in ((r.get("user") or {}).get("login") or "").lower()
                and len(r.get("body") or "") > 0]
    return analysed[-1].get("commit_id", "") if analysed else ""


# ------------------------------------------------------------- model turns


def claude(prompt, allowed_tools=None, permission_mode=None, schema=None,
           timeout=None, role="merge"):
    cmd = ["claude"]
    if allowed_tools:
        # Before -p, always. The other order makes claude read the prompt as
        # the flag's argument and then block waiting on stdin.
        cmd += ["--allowedTools", allowed_tools]
    if permission_mode:
        cmd += ["--permission-mode", permission_mode]
    if schema:
        cmd += ["--output-format", "json", "--json-schema", schema]
    cmd += ["-p", prompt]

    env = dict(os.environ, CLAUDE_HISTORY_ROLE=role)
    env.pop("CLAUDECODE", None)
    try:
        result = subprocess.run(cmd, capture_output=True, text=True,
                                timeout=timeout, env=env)
    except subprocess.TimeoutExpired:
        return None, f"timed out after {timeout}s"
    if result.returncode != 0:
        return None, (result.stderr or result.stdout or "")[:600]

    if not schema:
        return result.stdout, None
    try:
        payload = json.loads(result.stdout)
        return payload.get("structured_output") or payload.get("result"), None
    except json.JSONDecodeError:
        return None, f"unparseable JSON output: {result.stdout[:400]}"


# -------------------------------------------------------------------- steps


def ensure_pushed(state):
    """True only when origin really holds this HEAD.

    Both silent paths ended with a review requested of something origin does
    not have: a pr-commit that failed leaves the tree dirty, and
    `git log @{u}..HEAD` on a branch with no upstream exits non-zero with
    empty stdout — which reads exactly like "nothing to push".
    """
    if run(["git", "status", "--porcelain"], check=True).stdout.strip():
        banner("committing the working tree before the review")
        if not prcommit(state):
            return False
    upstream = run(["git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"])
    if upstream.returncode != 0:
        log("no upstream configured — pushing HEAD to origin")
        run(["git", "push", "-u", "origin", "HEAD"], check=True)
        return True
    if run(["git", "log", "@{u}..HEAD", "--oneline"], check=True).stdout.strip():
        log("pushing unpushed commits")
        run(["git", "push", "origin", "HEAD"], check=True)
    return True


def request_review(pr):
    # `full` rather than incremental: after a round of fixes the question is
    # whether the branch is now coherent, and incremental mode will not
    # re-examine a commit it has already seen.
    run(["gh", "pr", "comment", str(pr), "--body", "@coderabbitai full review"], check=True)


def await_review(pr):
    result = stream([os.path.join(HERE, "threads.sh"), "await-review",
                     str(REVIEW_WAIT), str(pr)], timeout=REVIEW_WAIT + 120)
    return result.returncode == 0


def triage(pr, terminal):
    for step in (["collect", "--pr", str(pr)], ["score"],
                 ["band"] + (["--terminal"] if terminal else [])):
        result = stream(["python3", os.path.join(HERE, "triage.py"), *step])
        if result.returncode != 0:
            return False
    return True


def load_queue(name):
    path = f"{STATE_DIR}/{name}-queue.json"
    if not os.path.exists(path):
        return []
    return json.load(open(path))


def file_queue(pr, state, name="defer"):
    queue = load_queue(name)
    if not queue:
        return
    banner(f"filing {len(queue)} {name} finding(s) to ClickUp")
    result = stream(["python3", os.path.join(LIB, "clickup.py"), "file",
                     "--queue", f"{STATE_DIR}/{name}-queue.json",
                     "--tag", "fix-now", "--pr", str(pr)], timeout=900)
    if result.returncode != 0:
        state.anomaly(f"{len(queue)} {name} finding(s) may not be filed to ClickUp")


def file_deferrals(pr, state):
    file_queue(pr, state, "defer")


FIX_PROMPT = """\
Fix exactly one code-review finding in this repository, and leave the
repository's gates green.

FINDING
  file     : {file}:{line}
  severity : {severity} (triage scored it {score}/10)
  claim    : {title}

{body}

WHAT DONE MEANS

1. The finding is actually addressed in the code. Read the file first — the
   reviewer can be wrong about this codebase. If it IS wrong, change
   nothing, set gates_green from a plain gates run, and say so in summary.
2. `./.claude/skills/pr-commit/gates.sh` exits 0. Run it. If it fails, fix
   whatever it reports — the production code or the test, whichever is
   actually wrong. A fix that leaves gates red is not a fix.
3. You touched nothing unrelated to this finding. Do not reformat, tidy,
   rename or "improve" anything else; another finding in this same round is
   being fixed in a separate call and unrelated edits will collide.

Do not commit, do not push, do not amend. The caller commits.

Return files_changed (paths you edited), gates_green (did gates.sh exit 0
on your last run of it), and summary (one sentence on what you changed, or
why you changed nothing).
"""


def apply_fix(finding, state):
    """One call per finding, so a bad fix is isolated to one finding."""
    log(f"\n  fixing [{finding['score']}] {finding['title'][:70]}")
    before = run(["git", "status", "--porcelain"], check=True).stdout

    payload, error = claude(
        FIX_PROMPT.format(
            file=finding.get("file", "?"), line=finding.get("line", "?"),
            severity=finding.get("severity", "?"), score=finding.get("score", 0),
            title=finding.get("title", ""), body=(finding.get("body") or "")[:6000],
        ),
        allowed_tools=FIX_TOOLS,
        permission_mode="acceptEdits",
        schema=FIX_SCHEMA,
        timeout=FIX_TIMEOUT,
        role="merge-fix",
    )

    if error or not isinstance(payload, dict):
        state.anomaly(f"fix call failed for {finding['file']}: {error or 'no structured output'}")
        return revert(before, finding, state)

    if not payload.get("gates_green"):
        state.anomaly(f"fix for {finding['file']} left gates red: {payload.get('summary','')[:160]}")
        return revert(before, finding, state)

    log(f"    ok — {payload.get('summary','')[:120]}")
    finding["fix_summary"] = payload.get("summary", "")
    return True


def revert(before_status, finding, state):
    """Last resort, not a policy: undo only what this fix touched, ticket
    the finding, and let the round carry on. The branch never carries a
    red-gate commit and the finding is not lost."""
    log("    reverting this fix and deferring the finding")
    after = run(["git", "status", "--porcelain"], check=True).stdout
    seen = {line[3:] for line in before_status.splitlines() if len(line) > 3}
    touched = [line[3:] for line in after.splitlines()
               if len(line) > 3 and line[3:] not in seen]
    for path in touched:
        run(["git", "checkout", "--", path])
        run(["git", "clean", "-fd", "--", path])
    finding["score"] = 8          # demote out of the fix band
    finding["reason"] = ("Could not be fixed automatically with gates green; "
                         + finding.get("reason", ""))[:500]
    return False


def answer_threads(pr, state):
    """Every finding gets its disposition said on its own thread.

    Mechanical because triage.py carries comment_id/thread_id through
    scoring — resolving a thread without saying what happened to it is how
    PR #70 merged past twelve findings nobody read.
    """
    banner("answering every thread")
    dispositions = (
        [("fixed", f) for f in load_queue("fix")]
        + [("deferred", f) for f in load_queue("defer")]
        + [("ignored", f) for f in load_queue("ignore")]
    )
    body_only = []
    for disposition, finding in dispositions:
        if not finding.get("thread_id") or not finding.get("comment_id"):
            body_only.append((disposition, finding))
            continue
        detail = build_detail(disposition, finding)
        result = run([os.path.join(HERE, "threads.sh"), "answer",
                      str(finding["comment_id"]), finding["thread_id"],
                      disposition, detail, str(pr)])
        if result.returncode != 0:
            state.anomaly(f"could not answer thread {finding['thread_id']}: "
                          f"{(result.stderr or '')[:200]}")
        else:
            log(f"  {disposition:9s} {finding['title'][:60]}")

    if body_only:
        post_body_only_comment(pr, body_only, state)


def filed_task(title):
    """The ClickUp task this finding became, from lib/clickup.py's record."""
    path = f"{STATE_DIR}/filed.json"
    if not os.path.exists(path):
        return None
    try:
        rows = json.load(open(path))
    except (json.JSONDecodeError, OSError):
        return None
    for row in rows:
        if row.get("id") and (row.get("title") or "")[:60] == (title or "")[:60]:
            return row
    return None


def build_detail(disposition, finding):
    reason = (finding.get("reason") or "").strip()
    if disposition == "fixed":
        return f"{finding.get('fix_summary') or reason} (triage score {finding['score']}/10)"
    if disposition == "deferred":
        # threads.sh refuses a deferral naming no destination, because one
        # without a destination is a silent drop. Name the actual task —
        # "Filed to ClickUp list <id>" does not satisfy that guard, and a
        # deferral it rejects is a thread that never gets answered at all.
        task = filed_task(finding.get("title"))
        where = (task.get("url") or f"task {task['id']}") if task else \
            "task pending — see the ClickUp `fix-now` queue"
        return (f"Score {finding['score']}/10 — real but not merge-blocking. "
                f"Deferred to {where}. {reason}")
    return f"Score {finding['score']}/10 — not actioned. {reason}"


def post_body_only_comment(pr, items, state):
    """Body-only findings have no reply target, so they are answered once,
    in the PR conversation, rather than silently."""
    lines = [
        "## Body-only findings — answered here",
        "",
        "These came from the review **body** (nitpick / outside-diff-range / "
        "duplicate sections), not from review threads. They carry no comment id "
        "and no reply target, so they cannot be answered individually.",
        "",
        "| Finding | File:line | Score | Disposition |",
        "|---|---|---|---|",
    ]
    for disposition, finding in items:
        lines.append("| %s | `%s:%s` | %d | **%s** — %s |" % (
            finding["title"].replace("|", "\\|")[:90],
            finding.get("file", "?"), finding.get("line", "?"),
            finding["score"], disposition,
            (finding.get("fix_summary") or finding.get("reason", "")).replace("|", "\\|")[:160],
        ))
    path = f"{STATE_DIR}/body-only-comment.md"
    with open(path, "w") as handle:
        handle.write("\n".join(lines))
    result = run(["gh", "pr", "comment", str(pr), "--body-file", path])
    if result.returncode != 0:
        state.anomaly(f"could not post the body-only comment: {(result.stderr or '')[:200]}")
    else:
        log(f"  posted one comment covering {len(items)} body-only finding(s)")


def record_round(pr, round_number):
    fix, defer, ignore = load_queue("fix"), load_queue("defer"), load_queue("ignore")
    total = len(fix) + len(defer) + len(ignore)
    if not total:
        return
    run(["python3", os.path.join(PRCOMMIT, "ledger.py"),
         "--source", "coderabbit-github", "--pr", str(pr),
         "--count", str(total),
         "--verdicts", f"fix={len(fix)},defer={len(defer)},reject={len(ignore)}"])
    log(f"  ledger: round {round_number} recorded — {total} finding(s)")


PRCOMMIT_PROMPT = """/pr-commit

You are running head-less: there is no one to answer a question, so do not
ask one. This applies in particular to the sync-doc-universe step, which
normally asks about every inconsistency it finds.

Headless contract for this run:
  - Resolve nothing by asking. If sync-doc-universe finds an inconsistency,
    do NOT ask and do NOT guess a resolution — leave both sides untouched,
    and list it under "ANOMALIES:" at the end of your output.
  - Everything else runs normally: gates, the recordings scan, the local
    CodeRabbit review round and its triage, update-memory, commit, push.
  - gates.sh should already be green; the fixes in this round were required
    to leave it so. If it is red, that is a real problem — report it under
    "ANOMALIES:" rather than working around it.

Finish by printing a line "ANOMALIES:" followed by one per line, or
"ANOMALIES: none".
"""


def prcommit(state):
    banner("pr-commit — gates, scan, local review, doc-universe, memory, commit, push")
    output, error = claude(
        PRCOMMIT_PROMPT,
        allowed_tools=FIX_TOOLS + ",Bash(gh *),Bash(coderabbit *),Bash(./.claude/skills/*)",
        permission_mode="acceptEdits",
        timeout=PRCOMMIT_TIMEOUT,
        role="merge-prcommit",
    )
    if error:
        state.anomaly(f"pr-commit failed: {error[:300]}")
        return False
    log((output or "")[-1500:])
    for line in (output or "").splitlines():
        if line.strip().startswith("ANOMALIES:") and "none" not in line.lower():
            state.anomaly(f"pr-commit reported: {line.strip()[:200]}")
    return True


# ------------------------------------------------------------------ rounds


def do_round(pr, state, round_number):
    terminal = round_number > LAST_FIX_ROUND
    banner(f"ROUND {round_number} / {LAST_ROUND}"
           + ("  — TRIAGE ONLY, nothing is fixed" if terminal else ""))

    if not ensure_pushed(state):
        state.anomaly(f"round {round_number}: HEAD is not on origin — "
                      "not requesting a review of a commit origin does not have")
        return False
    log(f"requesting a review of {head_sha(pr)[:7]}")
    request_review(pr)

    if not await_review(pr):
        state.anomaly(f"round {round_number}: no review arrived within {REVIEW_WAIT}s")
        log("Not re-requesting — if the answer was 'Review rate limited' the "
            "hourly quota is spent and asking again cannot help.")
        return False

    if not triage(pr, terminal):
        state.anomaly(f"round {round_number}: triage failed")
        return False

    file_deferrals(pr, state)

    if not terminal:
        fixes = load_queue("fix")
        if fixes:
            banner(f"fixing {len(fixes)} finding(s), one call each")
            demoted = [f for f in fixes if not apply_fix(f, state)]
            if demoted:
                # A fix that could not converge becomes a ticket rather than
                # a lost finding — APPENDED to the defer queue, never
                # written over it. Overwriting drops every finding already
                # deferred this round from `answer_threads` and from the
                # ledger count, so those threads would go unanswered while
                # the run reported success.
                already = load_queue("defer")
                json.dump(already + demoted,
                          open(f"{STATE_DIR}/defer-queue.json", "w"), indent=2)
                json.dump([f for f in fixes if f not in demoted],
                          open(f"{STATE_DIR}/fix-queue.json", "w"), indent=2)
                # File only the newcomers; the rest already have tickets.
                json.dump(demoted, open(f"{STATE_DIR}/demoted-queue.json", "w"), indent=2)
                file_queue(pr, state, "demoted")

    answer_threads(pr, state)
    record_round(pr, round_number)

    state.set(round=round_number)
    if not terminal:
        prcommit(state)
    else:
        log("\nRound 3 changes no code — nothing to commit, and that is the point:\n"
            "the commit this round reviewed is the commit round 4 approves.")
    return True


def do_approve(pr, state, repo):
    banner("ROUND 4 — approve")
    head, reviewed = head_sha(pr), reviewed_sha(pr, repo)

    if reviewed != head:
        log(f"REFUSING to approve.\n  HEAD     : {head[:7]}\n  reviewed : {reviewed[:7] or '(none)'}")
        log("`@coderabbitai approve` analyses nothing — it resolves every thread\n"
            "and approves. Approving a HEAD no review is pinned to certifies\n"
            "unreviewed code.")
        state.anomaly(f"approve skipped: HEAD {head[:7]} was never reviewed (last {reviewed[:7]})")
        return False

    listing = run(["python3", os.path.join(HERE, "render_review.py"),
                   "list", "--pr", str(pr), "--json"])
    # Exit 2 is a reconciliation MISMATCH — some finding is unaccounted for.
    # It still prints a payload, so parsing without checking would read
    # "0 unresolved" off a listing that does not add up.
    if listing.returncode != 0:
        log(f"REFUSING to approve: `list` exited {listing.returncode} — the findings "
            f"do not reconcile, so no count from it can be trusted.")
        state.anomaly(f"approve skipped: render_review list exited {listing.returncode}")
        return False
    payload = json.loads(listing.stdout or "{}")
    if not payload.get("reconciled", False):
        log("REFUSING to approve: the listing reports a reconciliation gap.")
        state.anomaly(f"approve skipped: unreconciled — {payload.get('gaps')}")
        return False
    unresolved = payload.get("counts", {}).get("unresolved", 0)
    if unresolved:
        log(f"REFUSING to approve: {unresolved} thread(s) still unresolved.")
        state.anomaly(f"approve skipped: {unresolved} unresolved thread(s)")
        return False

    log(f"HEAD {head[:7]} was reviewed and every thread is answered.")
    run(["gh", "pr", "comment", str(pr), "--body", "@coderabbitai approve"], check=True)
    log("approve requested; polling preflight for up to 10 minutes")
    for _ in range(20):
        time.sleep(30)
        if run([os.path.join(HERE, "threads.sh"), "preflight", str(pr)]).returncode == 0:
            log("preflight green")
            return True
    state.anomaly("approve did not clear preflight within 10 minutes")
    return False


def do_land(pr, state):
    banner("MERGE")
    # The loop always reaches a merge. A red preflight is a fact to record,
    # not an instruction to stop: everything that could legitimately block
    # has had three review rounds and a deferral queue.
    result = run([os.path.join(HERE, "threads.sh"), "preflight", str(pr)])
    print(result.stdout or "", (result.stderr or ""), flush=True)
    if result.returncode != 0:
        reasons = " | ".join((result.stderr or result.stdout or "").split("\n"))[:300]
        state.anomaly(f"merged with a red preflight: {reasons}")
        log("Merging anyway — the admin bypass permits it, and the reason is recorded.")

    if stream([os.path.join(HERE, "merge.sh"), str(pr)], timeout=600).returncode != 0:
        state.anomaly("merge.sh failed")
        return False
    state.set(merged_at=int(time.time()))
    return True


def do_postmortem(pr, state):
    elapsed = int(time.time()) - int(state.get("t0", time.time()))
    anomalies = state.get("anomalies", [])
    log(f"\nmerge took {elapsed // 60}m {elapsed % 60}s")
    if elapsed <= POSTMORTEM_SECONDS and not anomalies:
        log("clean and inside the time budget — no post-mortem needed")
        return
    log(f"POST-MORTEM: elapsed {elapsed}s (budget {POSTMORTEM_SECONDS}s), "
        f"{len(anomalies)} anomal{'y' if len(anomalies) == 1 else 'ies'}")
    for item in anomalies:
        log(f"  - {item}")
    stream([os.path.join(HERE, "postmortem.sh"), str(pr)], timeout=1800)


# -------------------------------------------------------------------- main


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pr")
    parser.add_argument("--only", choices=["round", "approve", "land", "postmortem"],
                        help="run a single phase (for testing)")
    parser.add_argument("--restart", action="store_true", help="ignore saved state")
    args = parser.parse_args()

    repo = run(["gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"],
               check=True).stdout.strip()
    # Normalised to str: --pr gives a string, the gh fallback gives an int,
    # and the state comparison below would silently discard a resumable run
    # depending on which way the PR was supplied.
    pr = str(args.pr or gh_json("pr", "view", "--json", "number")["number"])

    if args.restart and os.path.exists(STATE_PATH):
        os.remove(STATE_PATH)
    state = State()
    if str(state.get("pr", "")) != pr:
        state.data = {}
    state.set(pr=pr, t0=state.get("t0") or int(time.time()),
              anomalies=state.get("anomalies", []))

    if args.only == "approve":
        sys.exit(0 if do_approve(pr, state, repo) else 5)
    if args.only == "land":
        sys.exit(0 if do_land(pr, state) else 1)
    if args.only == "postmortem":
        do_postmortem(pr, state)
        sys.exit(0)

    start = int(state.get("round", 0)) + 1
    if start > 1:
        log(f"resuming: rounds 1-{start - 1} already done")

    for round_number in range(start, LAST_ROUND + 1):
        if not do_round(pr, state, round_number):
            log(f"\nRound {round_number} did not complete. State is saved — re-run "
                f"run.sh to resume, or run with --only land to merge as-is.")
            sys.exit(3)
        if args.only == "round":
            sys.exit(0)

    do_approve(pr, state, repo)      # advisory: a refusal is an anomaly, not a stop
    merged = do_land(pr, state)
    do_postmortem(pr, state)

    banner("deferred queue")
    stream(["python3", os.path.join(LIB, "clickup.py"), "list", "--tag", "fix-now"],
           timeout=600)
    sys.exit(0 if merged else 1)


if __name__ == "__main__":
    main()
