#!/usr/bin/env python3
"""Merge a pull request: up to three review rounds, then land it.

    python3 .claude/skills/pr-merge/merge.py

One script, one straight line, no resume, no flags:

    for round in 1, 2, 3:
        request_review()        # and wait out CodeRabbit's rate limit
        findings = read_comments()
        scored   = triage_comments(findings)
        fix / file a ticket / ignore, by score
        resolve every thread
        break if this round changed nothing, else commit
    merge
    postmortem

Rounds 1 and 2 fix what scores 9-10 and file 6-8 as ClickUp tickets. Round 3
fixes nothing — everything >= 6 becomes a ticket.

A round that changes no file ends the loop, because the next round would buy
a review of a byte-identical tree at a quarter of the hourly quota. That is
also what makes the approval honest, whichever round it happens in: the loop
only ever exits after a round that left HEAD where its own review found it,
so the commit that was reviewed is the commit that gets approved.

Nothing here is swallowed. The predecessor wrapped every round in
`except (Exception, SystemExit)` so the loop always reached a merge; it
reached one on PR #76 with eight recorded anomalies and a red preflight.
This one stops and says why. The single exception is CodeRabbit's rate
limit, which is not a failure — it is a wait.
"""

import concurrent.futures
import json
import os
import re
import shutil
import subprocess
import sys
import time
from datetime import datetime, timezone

HERE = os.path.dirname(os.path.abspath(__file__))
LIB = os.path.join(os.path.dirname(HERE), "lib")
CLICKUP = os.path.join(LIB, "clickup.py")
GATES = "./.claude/skills/pr-commit/gates.sh"

STATE_DIR = "tmp/merge"
FILED = f"{STATE_DIR}/filed.json"

LAST_FIX_ROUND = 2
LAST_ROUND = 3

FIX_FLOOR = 9
TICKET_FLOOR = 6

RATE_LIMIT_SLEEP = 900          # ~4 PR reviews an hour on the free tier
ACK_BUDGET = 300                # the bot answers a request in seconds
REVIEW_BUDGET = 900             # then takes minutes to post the review
POLL = 30
APPROVE_BUDGET = 600

SCORE_TIMEOUT = int(os.environ.get("MERGE_SCORE_TIMEOUT", "900"))
FIX_TIMEOUT = int(os.environ.get("MERGE_FIX_TIMEOUT", "2400"))
CLICKUP_TIMEOUT = int(os.environ.get("MERGE_CLICKUP_TIMEOUT", "900"))
GATES_TIMEOUT = int(os.environ.get("MERGE_GATES_TIMEOUT", "3600"))

DIFF_BUDGET_BYTES = int(os.environ.get("DIFF_BUDGET_BYTES", "200000"))
HISTORY_BUDGET_BYTES = 300_000

# The run's context: set once by start(), never reassigned. One invocation
# merges one PR from one checkout, so these are properties of the run rather
# than parameters of it. The round number is NOT one of them — it changes
# every iteration, and a value that changes is one a reader has to trace, so
# it is passed to each function that needs it.
REPO = ""
PR = 0
STARTED_AT = ""


# --------------------------------------------------------------- plumbing


def log(message):
    print(f"  {message}", flush=True)


def banner(message):
    print(f"\n══ {message} ══", flush=True)


def die(message):
    sys.exit(f"\nSTOPPED: {message}")


def sh(cmd, timeout=None, check=False, capture=True):
    """Run an argv list. No shell, ever.

    A timeout is diagnosed here rather than raised: the callers that pass one
    are the long ones (the gates, the ClickUp filing), and a bare
    TimeoutExpired traceback is exactly the "aborts with no diagnostic" this
    script exists not to do.
    """
    try:
        result = subprocess.run(
            cmd, capture_output=capture, text=True, timeout=timeout,
            env=dict(os.environ, CLAUDECODE=""),
        )
    except subprocess.TimeoutExpired:
        die(f"{' '.join(cmd[:4])} timed out after {timeout}s")
    if check and result.returncode != 0:
        die(f"{' '.join(cmd[:4])} failed ({result.returncode}):\n"
            f"{(result.stderr or result.stdout or '')[:800]}")
    return result


def git(*args, **kw):
    return sh(["git", *args], **kw)


def worktree_changes():
    """What git reports as uncommitted — empty when the tree is clean.

    Asked in several places for the same reason: may we proceed, and what is
    in the way. Returns the raw listing because every caller prints it;
    `changed_paths()` is the same question answered as a set.
    """
    return git("status", "--porcelain", check=True).stdout.strip()


def changed_paths():
    """The set of paths git reports as uncommitted.

    `-z` because the default format QUOTES any path holding a space or a
    non-ASCII byte, and a quoted name matches nothing it is compared
    against.
    """
    out = git("status", "--porcelain", "-z", check=True).stdout
    paths, entries = set(), iter(out.split("\0"))
    for entry in entries:
        if not entry:
            continue
        if entry[0] == "R":
            next(entries, None)      # a rename emits its source as a second record
        paths.add(os.path.normpath(entry[3:]))
    return paths


def gh(*args, check=True):
    return sh(["gh", *args], check=check).stdout


def gh_json(*args):
    return json.loads(gh(*args) or "null")


def claude(prompt, allowed_tools=None, permission_mode=None, schema=None,
           timeout=None, role="merge"):
    """One headless turn. Returns (payload, error) — never raises.

    `--allowedTools` goes BEFORE `-p`: the other order makes claude read the
    prompt as the flag's argument and then block forever on stdin.
    """
    cmd = ["claude"]
    if allowed_tools:
        cmd += ["--allowedTools", allowed_tools]
    if permission_mode:
        cmd += ["--permission-mode", permission_mode]
    if schema:
        cmd += ["--output-format", "json", "--json-schema", schema]
    cmd += ["-p", prompt]

    env = dict(os.environ, CLAUDE_HISTORY_ROLE=role)
    env.pop("CLAUDECODE", None)          # removed, not blanked: headless nesting
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


def parse_json_array(text, what):
    """A model asked for a JSON array. Anything else is a stop."""
    text = re.sub(r"^\s*```(?:json)?\s*|\s*```\s*$", "", (text or "").strip(), flags=re.M)
    start, end = text.find("["), text.rfind("]")
    if start == -1 or end == -1:
        die(f"{what} produced no JSON array:\n{text[:500]}")
    try:
        return json.loads(text[start:end + 1])
    except json.JSONDecodeError as error:
        die(f"{what} produced invalid JSON ({error}):\n{text[start:start + 500]}")


def save(path, payload):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as handle:
        json.dump(payload, handle, indent=2)


# ------------------------------------------------------- requesting a review


ACK_RATE_LIMITED = "Review rate limited"
ACK_ACCEPTED = ("Full review triggered", "Action performed")
# "Your next included review will be available in 18 minutes." — the bot
# says how long, so there is no reason to guess.
ACK_WAIT_RE = re.compile(r"available in (\d+)\s*minute")


def newest_comment_id(repo, pr):
    ids = gh("api", "--paginate", f"repos/{repo}/issues/{pr}/comments?per_page=100",
             "--jq", ".[].id").split()
    return max((int(i) for i in ids), default=0)


def newest_review_id(repo, pr):
    """`gh api --paginate` applies --jq to EACH page and concatenates, so a
    filter ending in `max` emits one number per page. Reduce here instead."""
    ids = gh("api", "--paginate", f"repos/{repo}/pulls/{pr}/reviews?per_page=100",
             "--jq", ".[].id").split()
    return max((int(i) for i in ids), default=0)


def bot_replies_after(repo, pr, since_id):
    payload = gh_json("api", "--paginate", f"repos/{repo}/issues/{pr}/comments?per_page=100")
    return [c for c in (payload or [])
            if c["id"] > since_id and is_bot((c.get("user") or {}).get("login"))]


def request_review(round_number):
    """Ask for a full review, waiting out the rate limit for as long as it takes.

    CodeRabbit answers a request with a comment saying which happened:
      ✅ Action performed / Full review triggered.   -> the review is coming
      ⚠️ Action not completed / Review rate limited. -> the quota is spent

    Nothing in the predecessor read that answer, so PR #76 requested four
    reviews in 25 minutes, was refused, and waited 900s each time for a
    review that was never going to arrive.

    Being pushed already is a precondition of asking: a review of a commit
    origin does not have is a review of the wrong thing.
    """
    banner(f"round {round_number} of {LAST_ROUND}"
           + (" — triage only, nothing is fixed" if round_number > LAST_FIX_ROUND else ""))
    check_pushed()

    attempt = 0
    while True:
        attempt += 1
        baseline_review = newest_review_id(REPO, PR)
        baseline_comment = newest_comment_id(REPO, PR)
        log(f"requesting a full review of {head_sha()[:7]} (attempt {attempt})")
        sh(["gh", "pr", "comment", str(PR), "--body", "@coderabbitai full review"], check=True)

        verdict, ack_id = await_acknowledgement(REPO, PR, baseline_comment)
        if verdict == "silent":
            die(f"CodeRabbit did not answer the review request within {ACK_BUDGET}s. "
                f"Check https://github.com/{REPO}/pull/{PR} — the bot may be down.")
        if verdict == "accepted" and await_review(REPO, PR, baseline_review, ack_id):
            return True
        sleep_off_rate_limit(comment_body(REPO, PR, ack_id))


def comment_body(repo, pr, comment_id):
    if not comment_id:
        return ""
    payload = gh_json("api", f"repos/{repo}/issues/comments/{comment_id}")
    return (payload or {}).get("body") or ""


def sleep_off_rate_limit(body):
    """Wait out the quota, for as long as the bot says it needs."""
    match = ACK_WAIT_RE.search(body or "")
    seconds = int(match.group(1)) * 60 + 60 if match else RATE_LIMIT_SLEEP
    log(f"rate limited — the review quota is spent. Sleeping {seconds // 60} min"
        + (" (the bot named the wait)." if match else "."))
    time.sleep(seconds)


def await_acknowledgement(repo, pr, baseline_comment):
    """-> (verdict, ack_comment_id), verdict in accepted | rate-limited | silent."""
    waited = 0
    while waited < ACK_BUDGET:
        time.sleep(POLL)
        waited += POLL
        for comment in bot_replies_after(repo, pr, baseline_comment):
            body = comment.get("body") or ""
            if ACK_RATE_LIMITED in body:
                return "rate-limited", comment["id"]
            if any(marker in body for marker in ACK_ACCEPTED):
                log(f"review accepted after {waited}s")
                return "accepted", comment["id"]
    return "silent", None


def await_review(repo, pr, baseline_review, ack_id):
    """Wait for a review OBJECT, not an acknowledgement. -> True, or False if
    the acknowledgement turned out to be a rate limit after all.

    The acknowledgement is re-read on every poll because **CodeRabbit edits
    it in place**: on PR #77 comment 5330633865 was posted at 15:48:15 saying
    the review was triggered and rewritten at 15:48:47 to say the quota was
    spent. Reading it once caught the optimistic version and then waited the
    full 900s for a review that was never coming.
    """
    waited = 0
    while waited < REVIEW_BUDGET:
        time.sleep(POLL)
        waited += POLL
        if newest_review_id(repo, pr) > baseline_review:
            log(f"review posted after {waited}s")
            return True
        if ACK_RATE_LIMITED in comment_body(repo, pr, ack_id):
            log(f"the acknowledgement was edited to a rate limit after {waited}s")
            return False
    die(f"no review object within {REVIEW_BUDGET}s of an accepted request. "
        f"The bot acknowledged and did not deliver — read the PR before re-running.")


# ------------------------------------------------------------ reading comments
#
# CodeRabbit puts findings in two places and only one of them is a thread.
# "Actionable comments" become review threads: they have an id, a reply
# target and a resolvable state. "Nitpick / Outside diff range / Duplicate"
# comments live ONLY inside the review body, with none of those. A helper
# that queries reviewThreads alone sees 16 of 28 findings, which is what
# happened on PR #70. Everything below exists to see the other 12.


NOISE_SUMMARIES = (
    "Analysis chain", "Prompt for AI Agents", "Skipped due to learnings",
    "Learnings used", "Review details", "Review info", "Run configuration",
    "Files selected for processing", "Files with no reviewable changes",
    "Files skipped from review", "Commits", "Autofix", "Tip",
)
BODY_ONLY_SECTIONS = ("Nitpick comments", "Outside diff range comments", "Duplicate comments")

SECTION_RE = re.compile(r"(%s)\s*\((\d+)\)" % "|".join(re.escape(s) for s in BODY_ONLY_SECTIONS))
FILE_HEADING_RE = re.compile(r"^(?P<path>\S.*?)\s*\((?P<count>\d+)\)$")
# Findings after the first in a file block are preceded by a markdown rule.
LINE_RANGE_RE = re.compile(r"^(?:\s*-{3,}\s*)*\s*`(?P<lines>[\d\-]+)`\s*:")
ACTIONABLE_RE = re.compile(r"Actionable comments posted:\s*(\d+)")
MARKER_RE = re.compile(r"<!--\s*cr-comment:v\d+:[0-9a-f]+\s*-->")
SUMMARY_RE = re.compile(r"<summary>(.*?)</summary>", re.DOTALL)
TAG_RE = re.compile(r"<[^>]+>")
BLOCKQUOTE_RE = re.compile(r"</?blockquote>")
HTML_COMMENT_RE = re.compile(r"<!--.*?-->", re.DOTALL)
# "_🟠 Major_" — the emoji sits INSIDE the underscores, and `_` is a word
# character so a trailing \b matches nothing.
SEVERITY_RE = re.compile(r"_[^_]*(Critical|Major|Minor|Trivial)[^_]*_", re.I)
TITLE_RE = re.compile(r"^\s*>?\s*\*\*(.+?)\*\*\s*$", re.M)
BODY_LINE_RE = re.compile(r"^\s*>?\s*`(\d+(?:-\d+)?)`\s*:", re.M)

OPEN, CLOSE = "<details>", "</details>"

THREAD_QUERY = """
query($owner:String!,$repo:String!,$pr:Int!){
  repository(owner:$owner,name:$repo){
    pullRequest(number:$pr){
      reviewThreads(first:100){
        pageInfo{ hasNextPage }
        nodes{
          id isResolved isOutdated path line
          comments(first:20){ pageInfo{ hasNextPage } nodes{ databaseId author{login} body } }
        }
      }
    }
  }
}
"""


def is_bot(login):
    return "coderabbit" in (login or "").lower()


def author_of(comment):
    """GitHub returns author: null for a deleted account, and reaching
    through it would abort the one routine whose job is to miss nothing."""
    return ((comment or {}).get("author") or {}).get("login") or "(deleted account)"


def fetch_threads(repo, pr):
    owner, name = repo.split("/", 1)
    data = gh_json("api", "graphql", "-f", f"query={THREAD_QUERY}",
                   "-F", f"owner={owner}", "-F", f"repo={name}", "-F", f"pr={pr}")
    threads = data["data"]["repository"]["pullRequest"]["reviewThreads"]
    # A truncated page turns every count below into a floor while still
    # reading as a total. "Could not answer" is not "nothing to report".
    if threads["pageInfo"]["hasNextPage"]:
        die("reviewThreads is truncated at 100 — paginate the query before trusting any count")
    for thread in threads["nodes"]:
        if thread["comments"]["pageInfo"]["hasNextPage"]:
            die(f"thread {thread['id']} has more than 20 comments — paginate before trusting it")
    return threads["nodes"]


def find_block(text, start):
    """Content span of the <details> opening at `start`, nesting-aware.

    Returns (content_start, content_end, block_end) or None if never closed.
    CodeRabbit nests <details> three deep.
    """
    depth = 0
    i = start
    while i < len(text):
        nxt_open = text.find(OPEN, i)
        nxt_close = text.find(CLOSE, i)
        if nxt_close == -1:
            return None
        if nxt_open != -1 and nxt_open < nxt_close:
            depth += 1
            i = nxt_open + len(OPEN)
            continue
        depth -= 1
        if depth == 0:
            return start + len(OPEN), nxt_close, nxt_close + len(CLOSE)
        i = nxt_close + len(CLOSE)
    return None


def summary_of(inner):
    match = SUMMARY_RE.search(inner)
    return TAG_RE.sub("", match.group(1)).strip() if match else ""


def is_noise(summary):
    return any(noise in summary for noise in NOISE_SUMMARIES)


def clean(body, depth=0):
    """Drop the machinery blocks, keep everything else headed by its summary.

    A denylist rather than an allowlist: the block carrying the proposed fix
    has at least four different summaries ("Proposed change", "Proposed
    guard", "Committable suggestion", "Refactor suggestion"), and an
    allowlist would silently drop the one that matters. Truncating at the
    first <details> is worse still — CodeRabbit routinely opens a comment
    with its verification transcript and puts the finding after it.
    """
    out = []
    i = 0
    while True:
        start = body.find(OPEN, i)
        if start == -1:
            out.append(body[i:])
            break
        block = find_block(body, start)
        if block is None:
            out.append(body[i:])
            break
        content_start, content_end, block_end = block
        inner = body[content_start:content_end]
        out.append(body[i:start])
        summary = summary_of(inner)
        if not is_noise(summary):
            stripped = SUMMARY_RE.sub("", inner, count=1)
            if summary:
                out.append("\n%s▸ %s\n" % ("  " * (depth + 1), summary))
            out.append(clean(stripped, depth + 1))
        i = block_end
    text = "".join(out)
    text = HTML_COMMENT_RE.sub("", text)
    text = BLOCKQUOTE_RE.sub("", text)
    return re.sub(r"\n{3,}", "\n\n", text).strip()


def claimed_counts(reviews):
    """What the review bodies say about themselves, summed across reviews."""
    counts = {"actionable": 0}
    counts.update({section: 0 for section in BODY_ONLY_SECTIONS})
    for review in reviews:
        if not is_bot((review.get("user") or {}).get("login")):
            continue
        body = review.get("body") or ""
        for match in ACTIONABLE_RE.finditer(body):
            counts["actionable"] += int(match.group(1))
        for match in SECTION_RE.finditer(body):
            counts[match.group(1)] += int(match.group(2))
    return counts


def extract_body_findings(review):
    findings = []
    body = review.get("body") or ""
    for match in SECTION_RE.finditer(body):
        block_start = body.rfind(OPEN, 0, match.start())
        if block_start == -1:
            continue
        block = find_block(body, block_start)
        if block is None:
            continue
        content_start, content_end, _ = block
        findings.extend(
            _findings_in_section(body[content_start:content_end], match.group(1), review["id"])
        )
    return findings


def _findings_in_section(section_body, section, review_id):
    findings = []
    i = 0
    while True:
        start = section_body.find(OPEN, i)
        if start == -1:
            break
        block = find_block(section_body, start)
        if block is None:
            break
        content_start, content_end, block_end = block
        inner = section_body[content_start:content_end]
        heading = FILE_HEADING_RE.match(summary_of(inner))
        if heading:
            stripped = SUMMARY_RE.sub("", inner, count=1)
            # A finding ENDS at its cr-comment marker, so only chunks that
            # precede one are findings. What trails the last marker is the
            # block's footer, and counting it inflates the total per file.
            chunks = MARKER_RE.split(stripped)
            chunks = chunks[:-1] if len(chunks) > 1 else chunks
            for chunk in chunks:
                text = clean(chunk)
                if not text:
                    continue
                lines = LINE_RANGE_RE.match(BLOCKQUOTE_RE.sub("", chunk).strip())
                findings.append({
                    "section": section,
                    "path": heading.group("path"),
                    "lines": lines.group("lines") if lines else "?",
                    "text": text,
                })
        i = block_end
    return findings


def _severity(body):
    match = SEVERITY_RE.search(body or "")
    return match.group(1).lower() if match else "?"


def _title(body):
    match = TITLE_RE.search(body or "")
    if match:
        return match.group(1).strip()[:200]
    for line in (body or "").splitlines():
        stripped = line.lstrip("> ").strip()
        if not stripped or stripped.startswith(("_", "`", "|", "▸", "-", "#")):
            continue
        return stripped.strip("*").strip()[:200]
    return (body or "").strip()[:200]


def _line_from_body(body):
    match = BODY_LINE_RE.search(body or "")
    return match.group(1) if match else "?"


def read_comments():
    """Every finding on the PR: unresolved threads plus body-only findings."""
    threads = fetch_threads(REPO, PR)
    reviews = gh_json("api", "--paginate", f"repos/{REPO}/pulls/{PR}/reviews?per_page=100") or []

    findings = []
    for thread in threads:
        if thread["isResolved"]:
            continue
        head = thread["comments"]["nodes"][0]
        body = clean(head.get("body") or "")
        findings.append({
            "id": f"thread:{thread['id']}",
            "source": "thread",
            "severity": _severity(body),
            "file": thread.get("path") or "?",
            "line": str(thread.get("line") or "?"),
            "title": _title(body),
            "body": body,
            "comment_id": head.get("databaseId"),
            "thread_id": thread["id"],
            "author": author_of(head),
        })

    body_only = [item for review in reviews if is_bot((review.get("user") or {}).get("login"))
                 for item in extract_body_findings(review)]
    for index, item in enumerate(body_only):
        text = item["text"]
        findings.append({
            # Indexed: several body-only findings share a file and often have
            # no parsed line, and a collision silently gives every colliding
            # finding the first one's score.
            "id": f"body:{index}:{item['path']}",
            "source": "body-only",
            "severity": _severity(text),
            "file": item["path"],
            "line": item["lines"] if item["lines"] != "?" else _line_from_body(text),
            "title": _title(text),
            "body": text,
            "comment_id": None,
            "thread_id": None,
            "author": "coderabbitai[bot]",
        })

    # Dedupe: the same defect reaches us as a thread and again in the body
    # often enough to matter.
    seen, unique = set(), []
    for finding in findings:
        key = (finding["file"], finding["line"], finding["title"][:80])
        if key in seen:
            continue
        seen.add(key)
        unique.append(finding)

    reconcile(threads, body_only, reviews, len(unique))
    return unique


def reconcile(threads, body_only, reviews, unique_count):
    """Compare what we extracted against what the reviews claim.

    A WARNING, never a stop. CodeRabbit over-counts — review 4960672409
    announced "Actionable comments posted: 6" and posted 5 — so treating the
    gap as fatal made the guard unsatisfiable and stalled PR #76's merge.
    A gap can mean a missed finding OR a bot over-count; both get printed.
    """
    claimed = claimed_counts(reviews)
    bot_threads = [t for t in threads if is_bot(author_of(t["comments"]["nodes"][0]))]
    claimed_body = sum(claimed[s] for s in BODY_ONLY_SECTIONS)

    gaps = []
    if len(bot_threads) != claimed["actionable"]:
        gaps.append(f"actionable: {len(bot_threads)} thread(s) found, {claimed['actionable']} claimed")
    if len(body_only) != claimed_body:
        gaps.append(f"body-only: {len(body_only)} extracted, {claimed_body} claimed")

    log(f"{len(threads)} thread(s), {len(body_only)} body-only, {unique_count} after dedupe")
    if gaps:
        log(f"! reconciliation gap — {'; '.join(gaps)}")
        log("  (CodeRabbit over-counts; continuing with what is actually there)")
    else:
        log("reconciled against the reviews' own counts")


# ------------------------------------------------------------------- triage


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


def triage_comments(findings, round_number):
    """One model call scores every finding. Anything unscored is a stop."""
    if not findings:
        return []

    payload = [{"id": f["id"], "file": f["file"], "line": f["line"],
                "reviewer_severity": f["severity"], "finding": f["body"][:2500]}
               for f in findings]
    text, error = claude(RUBRIC + "\n\nFindings:\n" + json.dumps(payload, indent=2),
                         timeout=SCORE_TIMEOUT, role="merge-triage")
    if error:
        die(f"scoring failed: {error}")

    verdicts = {}
    for row in parse_json_array(text, "scoring"):
        if isinstance(row, dict) and row.get("id"):
            verdicts[row["id"]] = row

    missing = [f["id"] for f in findings if f["id"] not in verdicts]
    if missing:
        die(f"scoring returned no verdict for {len(missing)} finding(s): {missing[:5]}")

    for finding in findings:
        row = verdicts[finding["id"]]
        try:
            score = int(row.get("score"))
        except (TypeError, ValueError):
            die(f"scoring returned a non-numeric score {row.get('score')!r} for {finding['id']}")
        if not 1 <= score <= 10:
            die(f"scoring returned {score} for {finding['id']} — outside 1-10")
        finding["score"] = score
        finding["reason"] = str(row.get("reason") or "")
    save(f"{STATE_DIR}/round-{round_number}/scored.json", findings)
    return findings


def split_comments(issues, round_number):
    """-> (to_fix, to_create, to_ignore).

    Rounds 1-2: >=9 fixed, 6-8 ticketed, <6 dropped.
    Round 3   : nothing fixed, >=6 ticketed, <6 dropped. An empty fix band
                is what ends the loop, so this is also what guarantees the
                run never exceeds three rounds — `main()` stops on its own
                once a round plans no fixes.

    A body-only finding is never fixed inline whatever it scores: it has no
    thread to answer and no diff position, so the fix could not be reported
    back even if it were made.
    """
    terminal = round_number > LAST_FIX_ROUND
    to_fix, to_create, to_ignore = [], [], []
    for finding in issues:
        score = finding["score"]
        if score < TICKET_FLOOR:
            to_ignore.append(finding)
        elif score >= FIX_FLOOR and not terminal and finding["source"] == "thread":
            to_fix.append(finding)
        else:
            to_create.append(finding)
    print_table(to_fix, to_create, to_ignore)
    return to_fix, to_create, to_ignore


def print_table(to_fix, to_create, to_ignore):
    rows = ([("Fix now", f) for f in to_fix]
            + [("Ticket", f) for f in to_create]
            + [("Ignore", f) for f in to_ignore])
    if not rows:
        log("no findings")
        return
    print()
    print("| Score | Band | Finding | File:line | Source |")
    print("|-------|------|---------|-----------|--------|")
    for band, finding in rows:
        title = finding["title"][:80].replace("|", r"\|")
        print(f"| {finding['score']} | {band} | {title} | "
              f"`{finding['file']}:{finding['line']}` | {finding['source']} |")
    print(f"\n**{len(to_fix)} to fix, {len(to_create)} ticketed, {len(to_ignore)} ignored**\n",
          flush=True)


# --------------------------------------------------------------------- fixing


FIX_TOOLS = (
    "Read,Edit,Write,Glob,Grep,"
    "Bash(git *),Bash(go *),Bash(gofmt *),Bash(golangci-lint *),"
    "Bash(python3 *),Bash(./scripts/*),"
    f"Bash({GATES})"
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

FIX_PROMPT = """\
Fix exactly one code-review finding in this repository, and leave the
repository's gates green.

FINDING
  file     : {file}:{line}
  severity : {severity} (triage scored it {score}/10)
  claim    : {title}

{body}

WHAT DONE MEANS

1. The finding is actually addressed in the code — a file really changes.
   Read the file first. The reviewer can be wrong about this codebase, and
   if it IS wrong, change nothing and say exactly why in summary. Be aware
   that this ENDS THE RUN rather than passing quietly: triage scored this
   {score}/10, and a scorer and a fixer that disagree is something a person
   has to settle. Do not make a token edit to avoid that.
2. `{gates}` exits 0. Run it. If it fails, fix whatever it reports — the
   production code or the test, whichever is actually wrong — and run it
   AGAIN. Keep going until it exits 0. Do not return while it is red, and
   do not return gates_green=true without having watched it exit 0 on your
   last run. A fix that leaves gates red is not a fix.
3. You touched nothing unrelated to this finding. Do not reformat, tidy,
   rename or "improve" anything else.

Do not commit, do not push, do not amend. The caller commits.

Return files_changed (paths you edited), gates_green (did {gates} exit 0 on
your last run of it), and summary (one sentence on what you changed, or why
you changed nothing).
"""


def fix(to_fix):
    """Fix each finding in turn. A failure stops the whole run, and leaves
    the worktree exactly as the failed fix left it.

    Nothing is reverted. A fix that could not converge is the one case where
    a person has to look, and what they need to see is what the agent
    actually did — half a fix that is nearly right is worth more than a
    clean tree and no evidence. Reverting would also throw away the work of
    every fix that already succeeded this round.

    Sequential on purpose: two fix agents editing one worktree collide, and
    the collision surfaces as a third finding nobody wrote.

    Bracketed by two assertions rather than watched fix by fix: the tree is
    clean going in, and every path the agents said they edited is dirty
    coming out. Between those two, whatever moved was moved by a fix.
    """
    if not to_fix:
        return []
    dirty = worktree_changes()
    if dirty:
        die("the worktree is dirty before any fix has run:\n"
            + "\n".join(f"    {line}" for line in dirty.splitlines())
            + "\n  Every path dirty after the fixes is supposed to be a fix. Deal with "
              "this\n  first, or the two cannot be told apart.")

    fixed = []
    for index, finding in enumerate(to_fix, 1):
        log(f"fix {index}/{len(to_fix)}: {finding['file']}:{finding['line']} — {finding['title'][:70]}")

        payload, error = claude(
            FIX_PROMPT.format(file=finding["file"], line=finding["line"],
                              severity=finding["severity"], score=finding["score"],
                              title=finding["title"], body=finding["body"][:6000],
                              gates=GATES),
            allowed_tools=FIX_TOOLS, permission_mode="acceptEdits",
            schema=FIX_SCHEMA, timeout=FIX_TIMEOUT, role="merge-fix")

        if error or not isinstance(payload, dict):
            die(f"the fix for {finding['file']}:{finding['line']} failed: "
                f"{error or 'no structured output'}\n"
                f"  finding: {finding['title']}\n"
                f"{worktree_note()}")
        if not payload.get("gates_green"):
            die(f"the fix for {finding['file']}:{finding['line']} left the gates red.\n"
                f"  finding: {finding['title']}\n"
                f"  agent  : {payload.get('summary', '')[:300]}\n"
                f"  A finding triage scored {finding['score']}/10 needs a person.\n"
                f"{worktree_note()}")
        log(f"  -> {payload.get('summary', '')[:150]}")
        finding["fix_summary"] = payload.get("summary", "")
        finding["files_changed"] = [os.path.normpath(p)
                                    for p in (payload.get("files_changed") or [])]
        fixed.append(finding)

    # Every file the agents claimed is a file git can see changed. A fix
    # that named none changed nothing, and one that named a file git does
    # not report never touched it — both matter because `resolve_conversations`
    # is about to answer these threads "Fixed on this branch", which would
    # be untrue, and the merge would proceed on it. Triage called each of
    # these wrong behaviour reachable by a real invocation; a scorer and a
    # fixer that disagree is something a person settles.
    silent = [f for f in fixed if not f["files_changed"]]
    if silent:
        die("these fixes changed no file:\n"
            + "\n".join(f"    {f['file']}:{f['line']} — {f.get('fix_summary', '')[:120]}"
                        for f in silent)
            + "\n  Triage scored them 9-10. Re-score them or fix them by hand, then re-run.")

    on_disk = changed_paths()
    phantom = sorted({p for f in fixed for p in f["files_changed"]} - on_disk)
    if phantom:
        die("these paths were reported as edited, and git does not see them changed:\n"
            + "\n".join(f"    {p}" for p in phantom)
            + "\n  The fix agent's report and the worktree disagree; nothing was "
              "committed.")
    return fixed


def worktree_note():
    """What the run is leaving behind, so the reader knows where to look."""
    dirty = worktree_changes()
    if not dirty:
        return "  The worktree is unchanged — the agent wrote nothing."
    lines = "\n".join(f"    {line}" for line in dirty.splitlines())
    return ("  Nothing was reverted and nothing was committed. The worktree "
            "holds this round's\n  work so far, including the failed fix:\n" + lines)


# -------------------------------------------------------------------- tickets


def create(to_create, round_number):
    """File one ClickUp ticket per finding, tagged fix-now.

    lib/clickup.py stays the single ClickUp interface because fix-queue
    invokes it by path, and its four-heading ticket shape is that skill's
    contract.
    """
    if not to_create:
        return []
    queue = f"{STATE_DIR}/round-{round_number}/ticket-queue.json"
    save(queue, to_create)
    result = sh(["python3", CLICKUP, "file", "--queue", queue,
                 "--tag", "fix-now", "--pr", str(PR)],
                timeout=CLICKUP_TIMEOUT, capture=False)
    if result.returncode != 0:
        die(f"filing {len(to_create)} ticket(s) failed — see the output above. "
            f"Threads cannot be answered with a destination that does not exist.")
    return to_create


def ignore(to_ignore, round_number):
    """Recorded, then dropped. Written to disk so a score can be argued with."""
    if to_ignore:
        save(f"{STATE_DIR}/round-{round_number}/ignored.json", to_ignore)
    return to_ignore


def wait_for_all(thunks):
    """Run the three dispositions concurrently and return their results in
    order. Only fix() touches the worktree, so the other two are free to run
    beside it."""
    with concurrent.futures.ThreadPoolExecutor(max_workers=len(thunks)) as pool:
        return [future.result() for future in [pool.submit(t) for t in thunks]]


# ------------------------------------------------------------------ resolving


def ticket_url(finding):
    """The ClickUp task this finding became, from clickup.py's record."""
    if not os.path.exists(FILED):
        return None
    try:
        with open(FILED) as handle:
            rows = json.load(handle)
    except (OSError, ValueError):
        return None
    for row in rows:
        if row.get("id") and (row.get("title") or "")[:60] == finding["title"][:60]:
            return row.get("url") or f"task {row['id']}"
    return None


def reply_and_resolve(comment_id, thread_id, text):
    sh(["gh", "api", "-X", "POST",
        f"repos/{REPO}/pulls/{PR}/comments/{comment_id}/replies",
        "-f", f"body={text}", "--jq", ".id"], check=True)
    sh(["gh", "api", "graphql", "-f",
        "query=mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}",
        "-F", f"id={thread_id}", "--jq", ".data.resolveReviewThread.thread.isResolved"],
       check=True)


def resolve_conversations(fixed, created, ignored):
    """Answer and resolve every thread. `main` requires conversation
    resolution, so a round that leaves one open cannot be merged.

    Body-only findings are skipped here because GitHub models them as
    nothing at all — no id, no reply target, no resolvable state.
    """
    answered = 0
    for finding, prefix, detail in (
        [(f, "**Fixed on this branch.**", f.get("fix_summary") or f["reason"]) for f in fixed]
        + [(f, "**Valid, deferred to a ticket.**",
            ticket_url(f) or "see the ClickUp `fix-now` queue") for f in created]
        + [(f, "**Not actioned.**", f["reason"]) for f in ignored]
    ):
        if not finding.get("thread_id"):
            continue
        reply_and_resolve(finding["comment_id"], finding["thread_id"], f"{prefix} {detail}")
        answered += 1
    log(f"answered and resolved {answered} thread(s)")

    # Anything still open — a thread opened by a human, or one whose finding
    # deduped away — is resolved with a reason. A single open thread blocks
    # the merge, and leaving it to the end is how PR #76 got there.
    stragglers = [t for t in fetch_threads(REPO, PR) if not t["isResolved"]]
    for thread in stragglers:
        head = thread["comments"]["nodes"][0]
        reply_and_resolve(head["databaseId"], thread["id"],
                          "**Not actioned.** Reviewed in this round and not selected for a fix "
                          "or a ticket; resolving so the merge is not blocked.")
    if stragglers:
        log(f"swept {len(stragglers)} remaining thread(s)")


# -------------------------------------------------------------------- commit


COMMIT_PROMPT = (
    "Generate a git commit message for the staged changes shown below. "
    "Format: title on line 1 (max 120 chars; use a conventional prefix like "
    "feat:/fix:/chore:/docs:/refactor: when the recent commits use them, otherwise "
    "plain imperative); blank line on line 2; then a bullet list of the changes "
    '(lines starting with "- ", no blank lines between bullets). No markdown code '
    "fences, no Co-authored-by, no Generated-with trailers, no surrounding quotes, "
    "no explanation. Output the message only."
)

DIFF_EXCLUDES = [":(exclude)tests/bdd-cli/fixtures/*/cassettes/*",
                 ":(exclude)docs/doc-universe.html"]


def staged_context():
    """The commit-message context, bounded. An unbounded diff is what failed
    with 'Prompt is too long' and no diagnostic on PR #70."""
    stat = git("diff", "--cached", "--stat", check=True).stdout
    body = git("diff", "--cached", check=True).stdout
    shape = "the complete diff"
    if len(body) > DIFF_BUDGET_BYTES:
        body = git("diff", "--cached", "--", ".", *DIFF_EXCLUDES, check=True).stdout
        shape = "the diff with recorded cassettes and doc-universe.html filtered out"
    if len(body) > DIFF_BUDGET_BYTES:
        body = body[:DIFF_BUDGET_BYTES]
        shape = ("a PREFIX of the diff, truncated — lean on the complete --stat above "
                 "rather than describing this prefix as the whole change")
    recent = git("log", "-5", "--pretty=format:%s%n%n%b%n---", check=True).stdout
    return (f"=== Recent commits (style reference) ===\n{recent}\n\n"
            f"=== Staged files (complete) ===\n{stat}\n\n"
            f"=== Diff — this is {shape} ===\n{body}\n")


def commit():
    """Commit and push what this round's fixes changed.

    No decisions of its own: `main()` has already established that this
    round planned fixes and that they changed the tree, so there is no
    "nothing to do" path in here to hide the reason a run stopped early.
    It follows that the last round never reaches this function —
    `split_comments` empties its fix band, and the loop leaves first.

    Deliberately not the pr-commit skill: that path also runs
    scan-recordings, a local review round, the ledger, doc-universe and
    memory sync. Those belong to a human's commit. A merge round's commit
    is a fix commit.
    """
    git("add", "-A", check=True)
    log("running the gates before committing")
    if sh([GATES], timeout=GATES_TIMEOUT, capture=False).returncode != 0:
        die("the gates are red after the fixes, though every fix reported them green.\n"
            "  Nothing was committed. Run the gates yourself and read the failure.")

    text, error = claude(COMMIT_PROMPT + "\n\n" + staged_context(),
                         timeout=600, role="commit-msg")
    if error:
        die(f"could not generate a commit message: {error}")
    message = re.sub(r"^\s*```[a-zA-Z]*\s*$", "", text or "", flags=re.M).strip()
    if not message:
        die("the commit message came back empty")

    path = f"{STATE_DIR}/commit-msg.txt"
    os.makedirs(STATE_DIR, exist_ok=True)
    with open(path, "w") as handle:
        handle.write(message + "\n")

    git("commit", "-F", path, check=True)
    git("push", "origin", "HEAD", check=True)
    log(f"committed and pushed: {message.splitlines()[0]}")


def head_sha():
    return gh("pr", "view", str(PR), "--json", "headRefOid", "--jq", ".headRefOid").strip()


def reviewed_sha():
    """The commit the last REAL CodeRabbit review was posted against.

    A non-empty body is the test, because `@coderabbitai approve` posts an
    APPROVED review with `body_len=0` while an actual review is kilobytes of
    COMMENTED. Measured on PR #76:

        CHANGES_REQUESTED  9cc2545  body=9138   <- a real review
        COMMENTED          41f85a5  body=3872   <- also a real review
        APPROVED           14e327a  body=0      <- `@coderabbitai approve`

    Deliberately no `--jq`: `gh api --paginate` applies the filter to each
    page and concatenates, so a filter ending in `last` emits one SHA per
    page rather than one overall.
    """
    reviews = json.loads(
        gh("api", "--paginate", f"repos/{REPO}/pulls/{PR}/reviews?per_page=100") or "[]")
    analysed = [r for r in reviews
                if is_bot((r.get("user") or {}).get("login")) and (r.get("body") or "")]
    return analysed[-1].get("commit_id", "") if analysed else ""


def check_pushed():
    """origin must already hold this HEAD. It does not make that true.

    Committing or pushing on the caller's behalf is how a merge run puts
    work on origin that the caller never decided to publish. So this only
    ever answers the question, and a "no" ends the run.

    Two paths say no silently and must not be collapsed: a dirty tree, and
    `git log @{u}..HEAD` on a branch with no upstream, which exits non-zero
    with empty stdout — indistinguishable from "nothing to push".
    """
    dirty = worktree_changes()
    if dirty:
        die("the working tree is dirty — commit it yourself before merging:\n"
            + "\n".join(f"  {line}" for line in dirty.splitlines()))
    if git("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}").returncode != 0:
        die("this branch has no upstream — push it yourself before merging:\n"
            "  git push -u origin HEAD")
    unpushed = git("log", "@{u}..HEAD", "--oneline", check=True).stdout.strip()
    if unpushed:
        die("origin does not have this HEAD — push it yourself before merging:\n"
            + "\n".join(f"  {line}" for line in unpushed.splitlines())
            + "\n  git push origin HEAD")
    # `@{u}` proves the commit reached SOME upstream. Only this proves it
    # reached the branch the PR is actually built from — a review of a
    # different commit is a review of something else.
    local, remote = git("rev-parse", "HEAD", check=True).stdout.strip(), head_sha()
    if local != remote:
        die(f"local HEAD is {local[:7]} but PR #{PR} is at {remote[:7]}.\n"
            f"  Reconcile them yourself — a review of a commit you are not on "
            f"answers a question\n  nobody asked.")


# --------------------------------------------------------------------- merge


def merge():
    """Approve, then land it.

    The loop only exits after a round that committed nothing, so HEAD is
    the commit that round's review was posted against — whichever round that
    was. That is what makes `@coderabbitai approve` honest here: it analyses
    nothing, it resolves threads and stamps whatever HEAD is. On PR #76 it
    stamped a commit no review was ever pinned to, which is why the check
    below reads the review record rather than trusting the sequence.
    """
    banner("merge")
    # Not a re-check of an argument — there isn't one. This catches the
    # checkout having MOVED during the run: a fix agent has Bash(git *), and
    # squashing the wrong branch then deleting it is not reversible.
    current = git("branch", "--show-current", check=True).stdout.strip()
    pr_branch = gh("pr", "view", str(PR), "--json", "headRefName", "--jq", ".headRefName").strip()
    if pr_branch != current:
        die(f"PR #{PR} is on branch '{pr_branch}', but '{current}' is now checked out. "
            f"The branch moved during the run; nothing was merged.")

    # A merge precondition, not a property of any round: this function ends
    # in `git checkout main`, which REFUSES on a dirty tree — by which point
    # the PR is squashed and the remote branch deleted, stranding the
    # checkout on a branch that no longer exists while holding uncommitted
    # work. That is PR #76's merge-err.txt verbatim.
    dirty = worktree_changes()
    if dirty:
        die("the worktree is dirty — refusing to merge:\n"
            + "\n".join(f"    {line}" for line in dirty.splitlines())
            + "\n  The merge ends in `git checkout main`, which refuses on a dirty tree "
              "— after\n  the squash and the branch deletion. Nothing was merged.")

    # The thing the whole round structure exists to guarantee, checked here
    # rather than inferred from the loop having ended in the right place.
    # `@coderabbitai approve` analyses nothing — it resolves threads and
    # stamps whatever HEAD happens to be. On PR #76 that was `14e327a`, a
    # commit no review had ever been posted against.
    head, reviewed = head_sha(), reviewed_sha()
    if not reviewed:
        die(f"no CodeRabbit review with a body was ever posted on #{PR}. "
            f"There is nothing to approve.")
    if reviewed != head:
        die(f"the last review was posted against {reviewed[:7]}, but #{PR} is now at "
            f"{head[:7]}.\n  Approving would stamp a commit nothing has reviewed — the "
            f"exact\n  PR #76 failure. Nothing was merged; re-run to review {head[:7]}.")
    log(f"HEAD {head[:7]} is the commit the last review was posted against")

    log("requesting approval")
    sh(["gh", "pr", "comment", str(PR), "--body", "@coderabbitai approve"], check=True)

    waited = 0
    while waited < APPROVE_BUDGET:
        time.sleep(POLL)
        waited += POLL
        decision = gh("pr", "view", str(PR), "--json", "reviewDecision",
                      "--jq", ".reviewDecision // \"\"").strip()
        if decision == "APPROVED":
            log(f"approved after {waited}s")
            break
    else:
        log(f"! no approval within {APPROVE_BUDGET}s — trying the merge anyway")

    os.makedirs(STATE_DIR, exist_ok=True)
    result = sh(["gh", "pr", "merge", str(PR), "--squash", "--delete-branch"])
    if result.returncode != 0:
        log("ordinary merge refused:")
        for line in (result.stderr or "").splitlines():
            log(f"  {line}")
        log("escalating to --admin (the ruleset grants the admin role a "
            "pull_request-scoped bypass)")
        sh(["gh", "pr", "merge", str(PR), "--squash", "--delete-branch", "--admin"],
           check=True, capture=False)

    main = "master" if git("show-ref", "--verify", "--quiet",
                           "refs/heads/master").returncode == 0 else "main"
    git("checkout", main, check=True)
    git("pull", "origin", main, check=True)
    git("branch", "-D", current)
    log(f"merged and back on {main}")


# ----------------------------------------------------------------- postmortem


HISTORY_ROLES = ("merge", "merge-triage", "merge-fix", "commit-msg", "clickup")

POSTMORTEM_PROMPT = """\
Below is the transcript of a pull-request merge run, extracted from this
session's history: every headless turn the merge script made, in order.

Diagnose the RUN, not the code it was merging. What to look for:
  - A round whose findings existed only because an earlier round's fix
    created them.
  - Review quota spent on anything that bought nothing.
  - A wait that was longer than it needed to be, or a stop that was not
    diagnosed well enough to act on.
  - A prompt that got an answer the script then had to work around.

Propose improvements to EXACTLY these files, and no others:
  .claude/skills/pr-merge/merge.py
  .claude/skills/pr-merge/SKILL.md
  .claude/skills/lib/clickup.py

Read them before proposing anything — a proposal that misreads the current
code is worse than none. You have read-only tools; do not attempt to edit.

Return ONLY a JSON array, no prose and no code fence:
[{"title": "<imperative, <=100 chars>",
  "score": <1-10 by how much time or noise it would save>,
  "reason": "<one sentence>",
  "file": "<the file it changes>",
  "line": "<line or ?>",
  "severity": "postmortem",
  "source": "postmortem",
  "body": "<markdown: what the transcript shows, with numbers; then the concrete change>"}]

--- BEGIN TRANSCRIPT ---
{transcript}
--- END TRANSCRIPT ---
"""


def history_extract():
    """The merge-related part of this session's history file.

    The live file runs to tens of megabytes, so it is never handed whole to
    a model. It does not need to be: every headless turn this script makes
    is labelled by CLAUDE_HISTORY_ROLE, so the merge turns are addressable
    by their own headings.
    """
    state = "docs/history/hook-state"
    if not os.path.exists(state):
        return None
    with open(state) as handle:
        name = handle.read().strip()
    path = os.path.join("docs/history", name)
    if not name or not os.path.exists(path):
        return None

    wanted = {f"## claude to @{role}" for role in HISTORY_ROLES}
    kept, section, keep = [], [], False
    stamp = re.compile(r"^_(\d{4}-\d{2}-\d{2}T[\d:]+Z)")

    with open(path, errors="replace") as handle:
        for line in handle:
            if line.startswith("## "):
                if keep:
                    kept.append("".join(section))
                header = line.strip()
                section, keep = [line], header in wanted
            elif section:
                match = stamp.match(line)
                if match and not keep and match.group(1) >= STARTED_AT:
                    keep = True         # a plain turn from inside the run window
                section.append(line)
    if keep:
        kept.append("".join(section))

    text = "".join(kept)
    if len(text) > HISTORY_BUDGET_BYTES:
        text = "…earlier turns omitted…\n\n" + text[-HISTORY_BUDGET_BYTES:]

    out = f"{STATE_DIR}/history-extract.md"
    os.makedirs(STATE_DIR, exist_ok=True)
    with open(out, "w") as handle:
        handle.write(text)
    log(f"history extract: {len(text)} bytes from {name} -> {out}")
    return text


def postmortem():
    """Read the run back and file what it suggests. Changes nothing."""
    banner("postmortem")
    transcript = history_extract()
    if not transcript:
        log("no session history to read — skipping")
        return

    text, error = claude(
        POSTMORTEM_PROMPT.replace("{transcript}", transcript),
        allowed_tools="Read,Glob,Grep", timeout=1800, role="merge-postmortem")
    if error:
        log(f"! postmortem failed: {error}")
        return

    proposals = parse_json_array(text, "postmortem")
    if not proposals:
        log("the postmortem proposed nothing")
        return

    queue = f"{STATE_DIR}/postmortem.json"
    save(queue, proposals)
    sh(["python3", CLICKUP, "file", "--queue", queue,
        "--tag", "merge-improvements", "--pr", str(PR)],
       timeout=CLICKUP_TIMEOUT, capture=False)

    # The postmortem reads; it never edits. If the worktree moved, something
    # wrote where nothing should have, and that must not be left for the
    # next branch to discover.
    dirty = worktree_changes()
    if dirty:
        die("the postmortem left changes in the worktree, which it must never do:\n"
            + "\n".join(f"  {line}" for line in dirty.splitlines()))
    log("worktree clean")


# ----------------------------------------------------------------------- main


def start():
    """Where are we, and is this a thing that can be merged at all?

    Everything that must be true before the first round, answered once and
    in one place, so the loop below is only ever the algorithm.
    """
    if sys.argv[1:]:
        sys.exit("usage: merge.py — no arguments. The PR comes from the current branch.")

    root = subprocess.run(["git", "rev-parse", "--show-toplevel"],
                          capture_output=True, text=True)
    if root.returncode != 0:
        sys.exit("not inside a git repository")
    os.chdir(root.stdout.strip())

    missing = [tool for tool in ("gh", "git", "claude") if not shutil.which(tool)]
    if missing:
        sys.exit(f"not on PATH: {', '.join(missing)}")

    branch = git("branch", "--show-current", check=True).stdout.strip()
    if not branch:
        sys.exit("detached HEAD — check out the branch whose PR you want merged")
    if branch in ("main", "master"):
        sys.exit(f"on {branch} — there is no PR to merge here")

    repo = gh("repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner").strip()
    view = sh(["gh", "pr", "view", "--json", "number,state"])
    if view.returncode != 0:
        sys.exit(f"no pull request open for '{branch}' — push it and open one first")
    payload = json.loads(view.stdout)
    if payload.get("state") != "OPEN":
        sys.exit(f"PR #{payload.get('number')} for '{branch}' is {payload.get('state')}, not OPEN")

    global REPO, PR, STARTED_AT
    REPO = repo
    PR = int(payload["number"])
    STARTED_AT = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    banner(f"merging {REPO} #{PR}")


def main():
    start()

    for round_number in range(1, LAST_ROUND + 1):
        request_review(round_number)
        comments = read_comments()
        issues = triage_comments(comments, round_number)
        to_fix, to_create, to_ignore = split_comments(issues, round_number)
        fixed, created, ignored = wait_for_all([
            lambda: fix(to_fix),
            lambda: create(to_create, round_number),
            lambda: ignore(to_ignore, round_number),
        ])
        resolve_conversations(fixed, created, ignored)
        # This round fixed nothing, so HEAD will not move and the next round
        # would buy a review of a byte-identical tree at a quarter of the
        # hourly quota. On PR #77 all three rounds reviewed the same commit
        # and found nothing to fix in any of them.
        # The converse needs no test here: `fix()` refuses to return having
        # changed no file, so a non-empty `to_fix` always has a commit.
        if not to_fix:
            break
        commit()

    merge()
    postmortem()


if __name__ == "__main__":
    main()
