#!/usr/bin/env bash
# Usage: lint-claude.md.sh
# Lint CLAUDE.md. Two checks, cheapest first. Its line budget and its
# opening block are markdownlint custom rules now — the `CLAUDE.md` override
# in .markdownlint-cli2.yaml — and `lints.sh` runs both gates for this file.
#
# 1. WIDTH — no line over MAX_COLS, OUTSIDE the mirrored block. The
#    exemption is not a nicety: the upstream file carries lines of 86, 95,
#    111, 114 and 192 characters, and check 2 fails if we reflow them. We
#    do not own those bytes, so they cannot be held to our column rule.
#
# 2. MIRROR — the fenced block at the top matches upstream byte for byte:
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
MAX_COLS=80
UPSTREAM=https://raw.githubusercontent.com/multica-ai/andrej-karpathy-skills/main/CLAUDE.md
BEGIN='<!-- KARPATHY:BEGIN'
END='<!-- KARPATHY:END'

fail() { echo "lint-claude.md: $*" >&2; exit 1; }

# ---- markers ----------------------------------------------------------
begin_n=$(grep -c "^$BEGIN" "$CLAUDE_MD" || true)
end_n=$(grep -c "^$END" "$CLAUDE_MD" || true)
[ "$begin_n" = 1 ] || fail "expected exactly one '$BEGIN' line in $CLAUDE_MD, found $begin_n"
[ "$end_n" = 1 ] || fail "expected exactly one '$END' line in $CLAUDE_MD, found $end_n"

begin_at=$(grep -n "^$BEGIN" "$CLAUDE_MD" | cut -d: -f1)
end_at=$(grep -n "^$END" "$CLAUDE_MD" | cut -d: -f1)
[ "$begin_at" -lt "$end_at" ] || fail "END marker (line $end_at) precedes BEGIN (line $begin_at)"

# ---- 1. width ---------------------------------------------------------
# Skipping strictly BETWEEN the markers, so the marker lines themselves are
# held to the rule — they are ours to write.
wide=$(awk -v b="$begin_at" -v e="$end_at" -v m="$MAX_COLS" \
	'NR>b && NR<e {next} length>m {printf "    line %d: %d chars\n", NR, length}' "$CLAUDE_MD")
[ -z "$wide" ] || fail "lines over $MAX_COLS columns:
$wide
Reflow them. Prose wraps; a long URL or table row that cannot wrap belongs
behind a shorter reference."

# ---- 2. upstream mirror -----------------------------------------------

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

echo "lint-claude.md: OK (all lines ≤$MAX_COLS cols; mirror at $begin_at-$end_at matches upstream)"
