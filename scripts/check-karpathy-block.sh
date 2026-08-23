#!/usr/bin/env bash
# Assert CLAUDE.md opens with the upstream behavioural guidelines, byte for byte.
#
# Upstream IS the source of truth — this fetches it live on every run:
#   https://github.com/multica-ai/andrej-karpathy-skills/blob/main/CLAUDE.md
#
# Two consequences to know, both deliberate:
#   * No network, no verdict. The fetch failing is a hard failure, not a pass —
#     "could not check" must never read as "checked and fine".
#   * An upstream edit turns this red on branches that never touched CLAUDE.md.
#     That is the point: the block is a mirror, and a mirror that silently
#     stops matching is not one. Re-sync with:
#       curl -sL "$UPSTREAM" > /tmp/k.md   # then paste between the markers

set -euo pipefail

cd "$(dirname "$0")/.."

CLAUDE_MD=CLAUDE.md
UPSTREAM=https://raw.githubusercontent.com/multica-ai/andrej-karpathy-skills/main/CLAUDE.md
BEGIN='<!-- KARPATHY:BEGIN'
END='<!-- KARPATHY:END'

fail() { echo "check-karpathy-block: $*" >&2; exit 1; }

begin_n=$(grep -c "^$BEGIN" "$CLAUDE_MD" || true)
end_n=$(grep -c "^$END" "$CLAUDE_MD" || true)
[ "$begin_n" = 1 ] || fail "expected exactly one '$BEGIN' line in $CLAUDE_MD, found $begin_n"
[ "$end_n" = 1 ] || fail "expected exactly one '$END' line in $CLAUDE_MD, found $end_n"

begin_at=$(grep -n "^$BEGIN" "$CLAUDE_MD" | cut -d: -f1)
end_at=$(grep -n "^$END" "$CLAUDE_MD" | cut -d: -f1)
[ "$begin_at" -lt "$end_at" ] || fail "END marker (line $end_at) precedes BEGIN (line $begin_at)"

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

echo "check-karpathy-block: OK (lines $begin_at-$end_at match $UPSTREAM)"
