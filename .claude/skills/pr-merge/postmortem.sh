#!/usr/bin/env bash
# Post-mortem for a merge that went badly — over ten minutes, or with an
# anomaly the loop recorded (no review arrived, preflight red, quota hit).
#
# It gathers the facts first, deterministically, because the facts are what
# a post-mortem usually lacks: how long each round took, how many findings
# each raised and where they landed, how much review quota was spent. Then
# one `claude -p` turn turns that into a diagnosis and concrete changes,
# researching anything it does not already know. The result is filed to
# ClickUp under `merge-improvements`.
#
#   postmortem.sh [pr]
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB="$HERE/../lib"
STATE_DIR=tmp/merge
FACTS="$STATE_DIR/postmortem-facts.md"
OUT="$STATE_DIR/postmortem.json"

mkdir -p "$STATE_DIR"
PR="${1:-$(python3 -c "import json;print(json.load(open('$STATE_DIR/state.json')).get('pr',''))" 2>/dev/null || true)}"
[ -n "$PR" ] || { echo "usage: postmortem.sh <pr>" >&2; exit 2; }
REPO=$(gh repo view --json nameWithOwner --jq .nameWithOwner)

{
  echo "# Merge post-mortem — PR #$PR"
  echo
  echo "## Loop state"
  echo '```'
  cat "$STATE_DIR/state.json" 2>/dev/null || echo "(no state file)"
  echo '```'
  echo
  echo "## Timeline"
  # Guarded: a post-mortem runs precisely when something went wrong, so any
  # of these facts may be unavailable. Under `set -euo pipefail` one missing
  # fact would abort the whole report — losing the facts that ARE available.
  gh pr view "$PR" --json createdAt,mergedAt,commits \
    --jq '"created: \(.createdAt)\nmerged : \(.mergedAt // "not merged")\ncommits: \(.commits | length)"' \
    2>/dev/null || echo "(PR metadata unavailable)"
  echo
  echo "## Review rounds, from the ledger"
  echo '```'
  # PR passed as argv, not interpolated into the heredoc: a PR number is
  # trusted here, but building code out of a variable is a habit worth not
  # having in a script that runs unattended.
  python3 - "$PR" <<'PY' || echo "(ledger unavailable)"
import json, sys
try:
    rounds = json.load(open("docs/ci/review-rounds.json"))["rounds"]
except (OSError, ValueError, KeyError) as error:
    print("could not read the ledger: %s" % error)
    sys.exit(0)
for r in rounds:
    if str(r.get("pr")) == sys.argv[1]:
        print(r["at"], r["source"], "found=%s" % r["found"], r.get("verdicts"))
PY
  echo '```'
  echo
  echo "## Every review object the bot posted"
  gh api --paginate "repos/$REPO/pulls/$PR/reviews?per_page=100" \
    --jq '.[] | "\(.submitted_at)  \(.state)  by \(.user.login)"' 2>/dev/null || true
  echo
  echo "## Findings by file (which files kept re-raising)"
  echo '```'
  python3 "$HERE/render_review.py" list --pr "$PR" --json 2>/dev/null \
    | python3 -c "
import json,sys,collections
d=json.load(sys.stdin)
c=collections.Counter([t['path'] for t in d['threads']] + [b['path'] for b in d['body_only']])
for path, n in c.most_common(): print('%3d  %s' % (n, path))
print('total', d['counts']['total'])
" || echo "(unavailable)"
  echo '```'
  echo
  echo "## CodeRabbit quota"
  coderabbit usage 2>/dev/null | sed 's/^/    /' || echo "    (unavailable)"
} > "$FACTS"

echo "facts -> $FACTS"

# Written to a file rather than captured with PROMPT=$(cat <<'EOF' …): bash
# 3.2 — still /bin/bash on macOS — mis-scans an apostrophe inside a heredoc
# nested in a command substitution, and this prompt is full of them.
cat > "$STATE_DIR/postmortem-prompt.txt" <<'EOF'
You are writing a post-mortem for a pull-request merge that was slower or
rougher than it should have been. The measured facts follow.

Diagnose the SLOWNESS, not the code. The questions that matter:
  - How many review rounds were there, and what did each round actually find?
  - Did any round's findings exist because an EARLIER round's fix created
    them? That pattern is the main thing to look for; on the merge that
    prompted this tooling, 13 of 16 findings were prose inconsistencies in
    one document, each authored by the previous round's fix.
  - Was review quota spent on anything that bought nothing?
  - Which single change would have removed the most elapsed time?

Where you are unsure how a tool behaves — CodeRabbit commands, rate limits,
GitHub rulesets, the gh CLI — say so explicitly in `unknowns` rather than
guessing; the caller will research those.

Return ONLY a JSON array of proposed improvements, no prose, no code fence.
Each entry:
[{"title": "<imperative, <=100 chars>",
  "score": <1-10 by how much time it would save>,
  "body": "<markdown: ## Why (what the facts show, with numbers) / ## What to change (concrete, file:line where known) / ## Verification / ## Context>",
  "unknowns": "<what you would need to check, or empty>"}]
EOF

env CLAUDE_HISTORY_ROLE=merge-postmortem \
  claude -p "$(cat "$STATE_DIR/postmortem-prompt.txt")" < "$FACTS" > "$STATE_DIR/postmortem.raw" || {
  echo "claude -p (post-mortem) failed:" >&2; head -20 "$STATE_DIR/postmortem.raw" >&2; exit 1
}

python3 - "$STATE_DIR/postmortem.raw" "$OUT" <<'PY'
import json, re, sys
raw = open(sys.argv[1]).read()
raw = re.sub(r"^\s*```(?:json)?\s*|\s*```\s*$", "", raw.strip(), flags=re.M)
start, end = raw.find("["), raw.rfind("]")
if start == -1:
    sys.exit("post-mortem produced no JSON array:\n" + raw[:400])
items = json.loads(raw[start:end + 1])
for item in items:
    item.setdefault("file", "-")
    item.setdefault("line", "-")
    item.setdefault("reason", item.get("unknowns", ""))
json.dump(items, open(sys.argv[2], "w"), indent=2)
print("%d improvement(s) -> %s" % (len(items), sys.argv[2]))
PY

echo
echo "Unknowns to research before filing (read the internet / context7 on each):"
python3 -c "
import json
for i in json.load(open('$OUT')):
    if i.get('unknowns'): print(' -', i['unknowns'])
" || true

echo
python3 "$LIB/clickup.py" file --queue "$OUT" --tag merge-improvements --pr "$PR"
