#!/usr/bin/env python3
"""File deferred findings as ClickUp tasks, through `claude -p` and MCP.

There is no ClickUp REST token in this repo's `.env`, and adding one would
be a second credential to hold for something the session can already do:
`claude -p` inherits the configured MCP servers, so it can create the tasks
itself. Two details make that work headlessly, both verified:

  * the tool allowlist is mandatory — without `--allowedTools` a headless
    run answers "permission not granted" and files nothing;
  * `--allowedTools` must come BEFORE `-p`, or the prompt is read as a flag
    argument and claude asks for input on stdin instead.

The queue is rendered to a markdown file first, and that file is the
artifact: it is what a person reads before anything is uploaded, and it is
what the model is asked to transcribe rather than invent.

Usage:
  clickup.py render --queue tmp/merge/defer-queue.json --tag fix-now --pr 76
  clickup.py file   --queue tmp/merge/defer-queue.json --tag fix-now --pr 76
  clickup.py list   --tag fix-now
"""

import argparse
import json
import os
import re
import subprocess
import sys

LIST_ID = os.environ.get("CLICKUP_LIST_ID", "901523097822")
TICKETS_MD = "tmp/merge/tickets.md"
FILED = "tmp/merge/filed.json"

CREATE_TOOLS = "mcp__claude_ai_ClickUP__createTask,mcp__claude_ai_ClickUP__addTagToTask"
LIST_TOOLS = "mcp__claude_ai_ClickUP__listTasks,mcp__claude_ai_ClickUP__getTask"

CLAUDE_TIMEOUT = int(os.environ.get("CLICKUP_CLAUDE_TIMEOUT", "900"))


def claude(allowed_tools, prompt):
    """`--allowedTools` before `-p`. The other order is not a style choice:
    claude reads the prompt as the flag's argument and then blocks on stdin."""
    env = dict(os.environ, CLAUDE_HISTORY_ROLE="clickup")
    env.pop("CLAUDECODE", None)
    try:
        result = subprocess.run(
            ["claude", "--allowedTools", allowed_tools, "-p", prompt],
            capture_output=True, text=True, env=env, timeout=CLAUDE_TIMEOUT,
        )
    except subprocess.TimeoutExpired:
        # A stalled MCP call must fail loudly. Hanging here would leave the
        # caller believing tickets were filed while nothing was.
        sys.exit("claude -p (ClickUp) timed out after %ds — nothing was filed."
                 % CLAUDE_TIMEOUT)
    if result.returncode != 0:
        sys.exit("claude -p failed (%d):\n%s" % (result.returncode, (result.stderr or result.stdout)[:600]))
    return result.stdout


def parse_json_array(text, what):
    text = re.sub(r"^\s*```(?:json)?\s*|\s*```\s*$", "", text.strip(), flags=re.M)
    start, end = text.find("["), text.rfind("]")
    if start == -1 or end == -1:
        sys.exit("%s returned no JSON array:\n%s" % (what, text[:600]))
    try:
        return json.loads(text[start : end + 1])
    except json.JSONDecodeError as error:
        sys.exit("%s returned invalid JSON (%s):\n%s" % (what, error, text[:600]))


def render(queue, tag, pr):
    """One markdown document, one `## ` heading per ticket.

    Written so a ticket can be picked up cold — the repository's rule for a
    deferral — and so the model's job is transcription, not authorship.
    """
    origin = "PR #%s" % pr if pr else "a local review"
    out = [
        "# ClickUp tickets to create",
        "",
        "List: `%s`   Tag: `%s`   Source: %s" % (LIST_ID, tag, origin),
        "",
        "%d ticket(s). One ClickUp task per `## ` heading below." % len(queue),
        "",
    ]
    for index, finding in enumerate(queue, 1):
        out += [
            "---",
            "",
            "## %d. %s" % (index, (finding.get("title") or "Review finding").strip()),
            "",
            "### Why",
            "",
            "CodeRabbit raised this on %s; triage scored it **%d/10** — real, but not"
            % (origin, finding.get("score", 0)),
            "worth blocking that merge on.",
            "",
            "> %s" % (finding.get("reason", "").strip() or "(no reason recorded)"),
            "",
            "### What to change",
            "",
            "`%s:%s`" % (finding.get("file", "?"), finding.get("line", "?")),
            "",
            (finding.get("body") or "").strip()[:4000],
            "",
            "### Verification",
            "",
            "```bash",
            "./.claude/skills/pr-commit/gates.sh",
            "```",
            "",
            "### Context",
            "",
            "Deferred rather than fixed inline: the merge loop caps fixing at two",
            "review rounds, because CodeRabbit's free tier allows ~4 PR reviews an",
            "hour and PR #76 exhausted that by fixing everything inline over four",
            "rounds. Reviewer severity `%s`, source `%s`."
            % (finding.get("severity", "?"), finding.get("source", "?")),
            "",
        ]
    return "\n".join(out)


def cmd_render(args):
    with open(args.queue) as handle:
        queue = json.load(handle)
    os.makedirs(os.path.dirname(TICKETS_MD), exist_ok=True)
    with open(TICKETS_MD, "w") as handle:
        handle.write(render(queue, args.tag, args.pr))
    print("%d ticket(s) -> %s" % (len(queue), TICKETS_MD))
    return 0


def cmd_file(args):
    with open(args.queue) as handle:
        queue = json.load(handle)
    if not queue:
        print("nothing to file from %s" % args.queue)
        return 0

    cmd_render(args)
    with open(TICKETS_MD) as handle:
        document = handle.read()

    prompt = f"""\
Create ClickUp tasks from the markdown document below, using the ClickUp MCP
tools. Exactly one task per `## ` heading — {len(queue)} in total.

For each:
  - list id: {LIST_ID}
  - name: the heading text, without its leading number
  - markdownContent: everything under that heading, verbatim
  - tag: {args.tag}

Transcribe. Do not summarise, rewrite, merge, split, reorder or improve any
ticket, and do not create a task that is not in the document. If a task
cannot be created, leave its "id" null and put the error in "error" — do not
retry silently and do not omit the row.

Return ONLY a JSON array, no prose and no code fence:
[{{"title": "<heading text>", "id": "<clickup task id or null>", "url": "<url or null>", "error": "<empty if created>"}}]

--- BEGIN DOCUMENT ---
{document}
--- END DOCUMENT ---
"""

    created = parse_json_array(claude(CREATE_TOOLS, prompt), "ticket creation")

    ok = [c for c in created if c.get("id")]
    failed = [c for c in created if not c.get("id")]

    os.makedirs(os.path.dirname(FILED), exist_ok=True)
    with open(FILED, "w") as handle:
        json.dump(created, handle, indent=2)

    for item in ok:
        print("filed %-12s %s" % (item.get("id"), (item.get("title") or "")[:70]))
    for item in failed:
        print("FAILED %-11s %s — %s"
              % ("-", (item.get("title") or "?")[:60], item.get("error", "no id returned")), file=sys.stderr)

    # A count that does not match is a silent drop, which is the whole
    # failure this queue exists to prevent — so it is an error, not a note.
    if len(created) != len(queue):
        print("MISMATCH: %d ticket(s) in the queue, %d row(s) returned. Nothing is trustworthy here."
              % (len(queue), len(created)), file=sys.stderr)
        return 1
    if failed:
        print("%d of %d ticket(s) were NOT created." % (len(failed), len(queue)), file=sys.stderr)
        return 1

    print("%d ticket(s) filed; record in %s" % (len(ok), FILED))
    return 0


def cmd_list(args):
    prompt = f"""\
Use the ClickUp MCP tools to list every OPEN (not closed, not complete) task
in list {LIST_ID} carrying the tag `{args.tag}`.

Return ONLY a JSON array sorted OLDEST FIRST by creation date, no prose and
no code fence:
[{{"id": "<id>", "name": "<name>", "status": "<status>", "url": "<url>"}}]

Oldest first matters: this is a work queue, and ClickUp's default ordering
is newest-first, which leaves the oldest item permanently last.
If there are none, return [].
"""
    tasks = parse_json_array(claude(LIST_TOOLS, prompt), "ticket listing")
    for task in tasks:
        print("%s\t%s\t%s" % (task.get("id"), task.get("status"), task.get("name")))
    if not tasks:
        print("(queue empty)")
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)

    for name, func in (("render", cmd_render), ("file", cmd_file)):
        item = sub.add_parser(name)
        item.add_argument("--queue", required=True)
        item.add_argument("--tag", required=True)
        item.add_argument("--pr")
        item.set_defaults(func=func)

    lister = sub.add_parser("list")
    lister.add_argument("--tag", required=True)
    lister.set_defaults(func=cmd_list)

    args = parser.parse_args()
    sys.exit(args.func(args))


if __name__ == "__main__":
    main()
