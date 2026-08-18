#!/usr/bin/env python3
"""Render a pull request's WHOLE review surface, and reconcile the count.

The bug this exists to fix: CodeRabbit posts findings in two places, and
only one of them is a review thread.

  * "Actionable comments posted: N" become real review threads. They have
    a comment id, can be replied to, and can be resolved.
  * "Nitpick comments (N)", "Outside diff range comments (N)" and
    "Duplicate comments (N)" live ONLY inside the review body, folded into
    a <details> block. They have no comment id, no reply target, and no
    resolve verb — and a helper that queries reviewThreads cannot see them
    at all.

On PR #70 that was 16 threads and 12 body-only findings; triage saw the 16
and merged past the 12 without ever knowing they existed.

Second bug, same file: the old renderer truncated each comment at its
first "<details>". CodeRabbit routinely opens a comment with its
verification transcript ("Analysis chain") and puts the finding AFTER it,
so six of those sixteen threads rendered as nothing but a severity line.
Here the noise blocks are removed by name and everything else is kept.

Usage:
  render_review.py list   [--pr N] [--all]
  render_review.py status [--pr N]

`list` exits 2 when the findings it extracted do not reconcile against the
counts the review bodies state about themselves. Silent under-reporting is
the failure being fixed, so the renderer has to be able to accuse itself.
"""

import argparse
import json
import re
import subprocess
import sys

RULE = "═" * 72

# Blocks CodeRabbit wraps around machinery rather than around a finding.
# Matched on the <summary> text, emoji-insensitively. Everything NOT named
# here is kept — a denylist, because the block that carries the proposed
# fix has at least four different summaries ("Proposed change", "Proposed
# guard", "Committable suggestion", "Refactor suggestion") and an allowlist
# would silently drop the one that matters.
NOISE_SUMMARIES = (
    "Analysis chain",
    "Prompt for AI Agents",
    "Skipped due to learnings",
    "Learnings used",
    "Review details",
    "Review info",
    "Run configuration",
    "Files selected for processing",
    "Files with no reviewable changes",
    "Files skipped from review",
    "Commits",
    "Autofix",
    "Tip",
)

# The three sections whose findings never become threads.
BODY_ONLY_SECTIONS = (
    "Nitpick comments",
    "Outside diff range comments",
    "Duplicate comments",
)

SECTION_RE = re.compile(
    r"(%s)\s*\((\d+)\)" % "|".join(re.escape(s) for s in BODY_ONLY_SECTIONS)
)
FILE_HEADING_RE = re.compile(r"^(?P<path>\S.*?)\s*\((?P<count>\d+)\)$")
# The range heading a finding: "`130-142`: _severity_ | …". Findings after
# the first in a file block are preceded by a markdown rule, so skip any
# leading "---" lines before matching.
LINE_RANGE_RE = re.compile(r"^(?:\s*-{3,}\s*)*\s*`(?P<lines>[\d\-]+)`\s*:")
ACTIONABLE_RE = re.compile(r"Actionable comments posted:\s*(\d+)")
MARKER_RE = re.compile(r"<!--\s*cr-comment:v\d+:[0-9a-f]+\s*-->")
SUMMARY_RE = re.compile(r"<summary>(.*?)</summary>", re.DOTALL)
TAG_RE = re.compile(r"<[^>]+>")
BLOCKQUOTE_RE = re.compile(r"</?blockquote>")
HTML_COMMENT_RE = re.compile(r"<!--.*?-->", re.DOTALL)

OPEN, CLOSE = "<details>", "</details>"


# ---------------------------------------------------------------- gh access


def gh(*args):
    result = subprocess.run(["gh", *args], capture_output=True, text=True)
    if result.returncode != 0:
        sys.exit("gh %s failed: %s" % (" ".join(args), result.stderr.strip()))
    return result.stdout


def gh_json(*args):
    return json.loads(gh(*args) or "null")


def repo_slug():
    return gh("repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner").strip()


def pr_number(explicit):
    if explicit:
        return int(explicit)
    return int(gh("pr", "view", "--json", "number", "--jq", ".number").strip())


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


def fetch_threads(repo, pr):
    owner, name = repo.split("/", 1)
    data = gh_json(
        "api", "graphql", "-f", "query=%s" % THREAD_QUERY,
        "-F", "owner=%s" % owner, "-F", "repo=%s" % name, "-F", "pr=%d" % pr,
    )
    threads = data["data"]["repository"]["pullRequest"]["reviewThreads"]
    # A truncated page would make every count below a floor rather than a
    # total, and the reconciliation would still print "✓ reconciled" —
    # the exact silent under-report this module exists to prevent. Refuse
    # instead: "could not answer" is not "nothing to report".
    if threads["pageInfo"]["hasNextPage"]:
        sys.exit("reviewThreads is truncated at 100 — paginate the query before trusting any count")
    for thread in threads["nodes"]:
        if thread["comments"]["pageInfo"]["hasNextPage"]:
            sys.exit(
                "thread %s has more than 20 comments — paginate before trusting the reply count "
                "or the resolved-by-comment map" % thread["id"]
            )
    return threads["nodes"]


def fetch_reviews(repo, pr):
    return gh_json("api", "--paginate", "repos/%s/pulls/%d/reviews?per_page=100" % (repo, pr))


def is_bot(login):
    return "coderabbit" in (login or "").lower()


def author_of(comment):
    """GitHub returns author: null for a deleted account. Reaching through
    it would raise and abort the whole listing — in the one tool whose job
    is to never let a finding go unseen."""
    return ((comment or {}).get("author") or {}).get("login") or "(deleted account)"


# ------------------------------------------------------------- body surgery


def find_block(text, start):
    """Content span of the <details> block opening at `start`.

    Returns (content_start, content_end, block_end), or None when the tag
    is never closed. Nesting-aware: CodeRabbit puts <details> inside
    <details> three levels deep.
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
    if not match:
        return ""
    return TAG_RE.sub("", match.group(1)).strip()


def is_noise(summary):
    return any(noise in summary for noise in NOISE_SUMMARIES)


def clean(body, depth=0):
    """Strip the noise blocks; keep everything else, headed by its summary."""
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
            pad = "  " * (depth + 1)
            if summary:
                out.append("\n%s▸ %s\n" % (pad, summary))
            out.append(clean(stripped, depth + 1))
        i = block_end
    text = "".join(out)
    # Every HTML comment here is CodeRabbit bookkeeping — the cr-comment
    # markers (already consumed for splitting), fingerprints, indicator
    # types, agent hints. None of it is the finding.
    text = HTML_COMMENT_RE.sub("", text)
    text = BLOCKQUOTE_RE.sub("", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text.strip()


def claimed_counts(reviews):
    """What the review bodies say they contain, summed across reviews."""
    counts = {"actionable": 0}
    for section in BODY_ONLY_SECTIONS:
        counts[section] = 0
    for review in reviews:
        if not is_bot((review.get("user") or {}).get("login")):
            continue
        body = review.get("body") or ""
        for match in ACTIONABLE_RE.finditer(body):
            counts["actionable"] += int(match.group(1))
        for match in SECTION_RE.finditer(body):
            # Only count a heading that really opens a <details> section,
            # not a mention of one in prose.
            counts[match.group(1)] += int(match.group(2))
    return counts


def extract_body_findings(review):
    """Findings that live only inside a review body — never a thread."""
    body = review.get("body") or ""
    findings = []
    for match in SECTION_RE.finditer(body):
        section = match.group(1)
        block_start = body.rfind(OPEN, 0, match.start())
        if block_start == -1:
            continue
        block = find_block(body, block_start)
        if block is None:
            continue
        content_start, content_end, _ = block
        findings.extend(
            _findings_in_section(body[content_start:content_end], section, review["id"])
        )
    return findings


def _findings_in_section(section_body, section, review_id):
    """Walk the per-file sub-blocks of one section."""
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
            path = heading.group("path")
            stripped = SUMMARY_RE.sub("", inner, count=1)
            # A finding ENDS at its cr-comment marker, so only the chunks
            # that precede one are findings. Whatever trails the last
            # marker is the block's own footer ("_Source: Learnings_"),
            # and counting it inflates the total by one per file.
            chunks = MARKER_RE.split(stripped)
            chunks = chunks[:-1] if len(chunks) > 1 else chunks
            for chunk in chunks:
                text = clean(chunk)
                if not text:
                    continue
                lines = LINE_RANGE_RE.match(BLOCKQUOTE_RE.sub("", chunk).strip())
                findings.append(
                    {
                        "section": section,
                        "review": review_id,
                        "path": path,
                        "lines": lines.group("lines") if lines else "?",
                        "text": text,
                    }
                )
        i = block_end
    return findings


# ----------------------------------------------------------------- commands


def cmd_list(args):
    repo = repo_slug()
    pr = pr_number(args.pr)
    threads = fetch_threads(repo, pr)
    reviews = fetch_reviews(repo, pr)

    body_findings = []
    for review in reviews:
        if is_bot((review.get("user") or {}).get("login")):
            body_findings.extend(extract_body_findings(review))

    unresolved = [t for t in threads if not t["isResolved"]]
    shown = threads if args.all else unresolved

    for thread in shown:
        head = thread["comments"]["nodes"][0]
        replies = len(thread["comments"]["nodes"]) - 1
        print(RULE)
        print("THREAD : %s" % thread["id"])
        print(
            "FILE   : %s:%s%s"
            % (
                thread["path"],
                thread["line"] if thread["line"] is not None else "?",
                "   [OUTDATED — line has since moved]" if thread["isOutdated"] else "",
            )
        )
        print("STATE  : %s" % ("resolved" if thread["isResolved"] else "unresolved"))
        print("%s (comment %s):" % (author_of(head), head["databaseId"]))
        print(clean(head["body"] or ""))
        if replies > 0:
            print("--- %d reply(ies) already on this thread ---" % replies)

    for finding in body_findings:
        print(RULE)
        print("BODY-ONLY FINDING  [NO THREAD — cannot be replied to or resolved individually]")
        print("SECTION: %s   (review %s)" % (finding["section"], finding["review"]))
        print("FILE   : %s:%s" % (finding["path"], finding["lines"]))
        print(finding["text"])

    print(RULE)
    bot_threads = [t for t in threads if is_bot(author_of(t["comments"]["nodes"][0]))]
    human_threads = len(threads) - len(bot_threads)
    claimed = claimed_counts(reviews)
    claimed_body = sum(claimed[s] for s in BODY_ONLY_SECTIONS)

    # TOTAL counts every thread, human ones included: they all need a row
    # in the triage table. Only the reconciliation below narrows to the
    # bot's threads, because only the bot states a count to check against.
    print(
        "THREADS: %d (%d unresolved, %d resolved)%s   BODY-ONLY: %d   TOTAL: %d"
        % (
            len(threads),
            len(unresolved),
            len(threads) - len(unresolved),
            "   [%d opened by a human]" % human_threads if human_threads else "",
            len(body_findings),
            len(threads) + len(body_findings),
        )
    )
    print(
        "reviews claim: actionable=%d nitpick=%d outside-diff=%d duplicate=%d"
        % (
            claimed["actionable"],
            claimed["Nitpick comments"],
            claimed["Outside diff range comments"],
            claimed["Duplicate comments"],
        )
    )

    gaps = []
    if len(bot_threads) != claimed["actionable"]:
        gaps.append(
            "actionable: %d thread(s) found, %d claimed" % (len(bot_threads), claimed["actionable"])
        )
    if len(body_findings) != claimed_body:
        gaps.append(
            "body-only: %d extracted, %d claimed" % (len(body_findings), claimed_body)
        )
    if gaps:
        print("✗ MISMATCH — %s" % "; ".join(gaps))
        print(
            "  Read the review body directly (gh api repos/%s/pulls/%d/reviews --jq '.[].body')"
            % (repo, pr)
        )
        print("  and fix this parser. A mismatch is never a reason to proceed.")
        return 2
    print("✓ reconciled")
    if not args.all and len(threads) != len(unresolved):
        print("(%d resolved thread(s) hidden — pass --all to print them)" % (len(threads) - len(unresolved)))
    return 0


def cmd_status(args):
    repo = repo_slug()
    pr = pr_number(args.pr)
    view = gh_json(
        "pr", "view", str(pr), "--json",
        "mergeable,mergeStateStatus,reviewDecision,statusCheckRollup,headRefOid",
    )
    threads = fetch_threads(repo, pr)
    reviews = fetch_reviews(repo, pr)

    unresolved = [t for t in threads if not t["isResolved"]]
    print("mergeable      : %s" % view.get("mergeable"))
    print("mergeStateStatus: %s" % view.get("mergeStateStatus"))
    print("reviewDecision : %s" % (view.get("reviewDecision") or "(none)"))
    print("threads        : %d unresolved of %d" % (len(unresolved), len(threads)))
    for check in view.get("statusCheckRollup") or []:
        print(
            "check %-24s %s %s"
            % (
                check.get("name") or check.get("context"),
                check.get("status") or check.get("state"),
                check.get("conclusion") or "",
            )
        )

    resolved_by_comment = {}
    for thread in threads:
        for comment in thread["comments"]["nodes"]:
            resolved_by_comment[comment["databaseId"]] = thread["isResolved"]

    verdicts = [r for r in reviews if r.get("state") in ("CHANGES_REQUESTED", "APPROVED", "DISMISSED")]
    latest = max((r.get("submitted_at") or "" for r in reviews), default="")
    print("\nlatest review object: %s" % (latest or "(none)"))

    # The evidence a stale-verdict decision is made on. Assembled here
    # because assembling it by hand at 04:15 is what PR #70 did.
    for review in verdicts:
        sha = review.get("commit_id") or ""
        ahead = "?"
        if sha and view.get("headRefOid"):
            try:
                ahead = gh_json(
                    "api", "repos/%s/compare/%s...%s" % (repo, sha, view["headRefOid"])
                ).get("ahead_by", "?")
            except SystemExit:
                ahead = "?"
        comments = gh_json(
            "api", "--paginate",
            "repos/%s/pulls/%d/reviews/%s/comments?per_page=100" % (repo, pr, review["id"]),
        ) or []
        ids = [c["id"] for c in comments]
        open_ids = [i for i in ids if resolved_by_comment.get(i) is False]
        print(
            "\nreview %s  %s  by %s  at %s"
            % (review["id"], review.get("state"), (review.get("user") or {}).get("login"), review.get("submitted_at"))
        )
        print("  pinned to     : %s (HEAD is %s commit(s) ahead)" % (sha[:7] or "?", ahead))
        print("  its threads   : %d total, %d still unresolved" % (len(ids), len(open_ids)))
        claimed = claimed_counts([review])
        body_only = sum(claimed[s] for s in BODY_ONLY_SECTIONS)
        if body_only:
            print("  body-only     : %d finding(s) with no thread — see `list`" % body_only)
        if review.get("state") == "CHANGES_REQUESTED" and not open_ids and ahead not in ("?", 0):
            print("  → STALE: every thread answered, and HEAD moved past the commit it judged.")
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)

    lister = sub.add_parser("list")
    lister.add_argument("--pr")
    lister.add_argument("--all", action="store_true", help="print resolved threads too")
    lister.set_defaults(func=cmd_list)

    status = sub.add_parser("status")
    status.add_argument("--pr")
    status.set_defaults(func=cmd_status)

    args = parser.parse_args()
    sys.exit(args.func(args))


if __name__ == "__main__":
    main()
