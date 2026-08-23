#!/usr/bin/env bash
# Usage: lint-claude.md.sh
# Lint CLAUDE.md. Three checks, cheapest first.
#
# 1. SIZE — the file stays under MAX_LINES. CLAUDE.md is a cache of the
#    repository loaded into every session, so its cost is paid constantly
#    and its growth is invisible one commit at a time. The `update-memory`
#    skill states the same limit in prose; this is what makes it true.
#
# 2. WIDTH — no line over MAX_COLS, OUTSIDE the mirrored block. The
#    exemption is not a nicety: the upstream file carries lines of 86, 95,
#    111, 114 and 192 characters, and check 3 fails if we reflow them. We
#    do not own those bytes, so they cannot be held to our column rule.
#
# 3. MIRROR — the fenced block at the top matches upstream byte for byte:
#      https://github.com/multica-ai/andrej-karpathy-skills/blob/main/CLAUDE.md
#    Fetched live on every run. Two consequences, both deliberate:
#      * No network, no verdict. A failed fetch is a hard failure, never a
#        pass — "could not check" must not read as "checked and fine".
#      * An upstream edit turns this red on branches that never touched
#        CLAUDE.md. That is the point: a mirror that silently stops
#        matching is not one. Re-sync by pasting the new upstream bytes
#        between the markers.

set -euo pipefail

cd "$(dirname "$0")/.."

CLAUDE_MD=CLAUDE.md
MAX_LINES=215
MAX_COLS=80
UPSTREAM=https://raw.githubusercontent.com/multica-ai/andrej-karpathy-skills/main/CLAUDE.md
BEGIN='<!-- KARPATHY:BEGIN'
END='<!-- KARPATHY:END'

fail() { echo "lint-claude.md: $*" >&2; exit 1; }

# ---- 1. size ----------------------------------------------------------
lines=$(wc -l < "$CLAUDE_MD" | tr -d ' ')
[ "$lines" -lt "$MAX_LINES" ] ||
	fail "$CLAUDE_MD is $lines lines, limit is $MAX_LINES.
Free lines before adding: prefer the point of use — a package doc comment, a
script header, README.md, a config file's comments, docs/for_further/."

# ---- markers ----------------------------------------------------------
begin_n=$(grep -c "^$BEGIN" "$CLAUDE_MD" || true)
end_n=$(grep -c "^$END" "$CLAUDE_MD" || true)
[ "$begin_n" = 1 ] || fail "expected exactly one '$BEGIN' line in $CLAUDE_MD, found $begin_n"
[ "$end_n" = 1 ] || fail "expected exactly one '$END' line in $CLAUDE_MD, found $end_n"

begin_at=$(grep -n "^$BEGIN" "$CLAUDE_MD" | cut -d: -f1)
end_at=$(grep -n "^$END" "$CLAUDE_MD" | cut -d: -f1)
[ "$begin_at" -lt "$end_at" ] || fail "END marker (line $end_at) precedes BEGIN (line $begin_at)"

# ---- 2. width ---------------------------------------------------------
# Skipping strictly BETWEEN the markers, so the marker lines themselves are
# held to the rule — they are ours to write.
wide=$(awk -v b="$begin_at" -v e="$end_at" -v m="$MAX_COLS" \
	'NR>b && NR<e {next} length>m {printf "    line %d: %d chars\n", NR, length}' "$CLAUDE_MD")
[ -z "$wide" ] || fail "lines over $MAX_COLS columns:
$wide
Reflow them. Prose wraps; a long URL or table row that cannot wrap belongs
behind a shorter reference."

# ---- 3. upstream mirror -----------------------------------------------

# The block must come first: nothing but blank lines may precede it. CLAUDE.md is
# read top-down and a preamble above the guidelines would outrank them.
if [ "$begin_at" -gt 1 ]; then
	head -n "$((begin_at - 1))" "$CLAUDE_MD" | grep -q '[^[:space:]]' &&
		fail "non-blank content above the BEGIN marker (line $begin_at); the block must open the file"
fi

upstream_copy=$(mktemp)
trap 'rm -f "$upstream_copy"' EXIT

# --fail so an HTML 404 page is an error rather than content to diff against.
curl -sSL --fail --max-time 30 "$UPSTREAM" -o "$upstream_copy" ||
	fail "could not fetch $UPSTREAM — this gate needs the network, and an unreachable upstream is a failure, not a pass"
[ -s "$upstream_copy" ] || fail "$UPSTREAM returned an empty body"

# Everything strictly between the markers, compared to the live upstream bytes.
if ! sed -n "$((begin_at + 1)),$((end_at - 1))p" "$CLAUDE_MD" | diff -u "$upstream_copy" - ; then
	fail "the KARPATHY block in $CLAUDE_MD has drifted from upstream (diff above; '-' is upstream, '+' is CLAUDE.md).
Paste the upstream bytes back between the markers — see the header of this script."
fi

echo "lint-claude.md: OK ($lines/$MAX_LINES lines, all ≤$MAX_COLS cols; mirror at $begin_at-$end_at matches upstream)"
