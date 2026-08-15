#!/usr/bin/env bash
# Usage: scan-recordings.sh
#
# Deterministic pre-publication scan of the RECORDED fixture data —
# cassettes and goldens — for anything that identifies the machine or
# account that recorded them.
#
# This exists because the LLM reviewer cannot do it: a re-record changes
# ~475 files, four times CodeRabbit's per-review limit, and reading
# recordings line by line is not what it is good at anyway. What is
# needed here is not judgement but exhaustiveness, and grep is exhaustive.
#
# The failure it prevents already happened once: a recording carrying a
# developer's home directory and their 23 connected MCP servers reached a
# public repository, and the first thing to notice was a bot reviewing
# the push.
set -uo pipefail

ROOT=$(git rev-parse --show-toplevel) || exit 1
cd "$ROOT"

RECORDINGS='tests/bdd-cli/fixtures/*/cassettes'
FOUND=0

report() {
  FOUND=1
  echo "RECORDING LEAK — $1" >&2
  shift
  printf '  %s\n' "$@" >&2
}

# 1. Home directories. Not $HOME literally: a cassette recorded by
#    someone else carries THEIR path, and this must fail for them too.
HITS=$(grep -rlE '/(Users|home)/[a-z_][a-z0-9_-]*' $RECORDINGS 2>/dev/null | head -5)
[ -n "$HITS" ] && report "a home directory path survived normalization" $HITS

# 2. The agent CLI's session-init inventory: which integrations, skills
#    and plugins the recording machine had installed.
HITS=$(grep -rlE '"(mcp_servers|slash_commands|memory_paths|apiKeySource)"' $RECORDINGS 2>/dev/null | head -5)
[ -n "$HITS" ] && report "session inventory survived sanitizing" $HITS

# 3. Credentials. Never yet seen in a recording, which is exactly when a
#    check is worth having.
HITS=$(grep -rlE '(sk-[A-Za-z0-9_-]{16,}|ghp_[A-Za-z0-9]{20,}|xox[baprs]-[A-Za-z0-9-]{10,}|BEGIN [A-Z ]*PRIVATE KEY)' \
  $RECORDINGS 2>/dev/null | head -5)
[ -n "$HITS" ] && report "credential-shaped string in a recording" $HITS

# 4. E-mail addresses, minus the ones this project legitimately ships.
HITS=$(grep -rlE '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-z]{2,}' $RECORDINGS 2>/dev/null \
  | xargs grep -lvE 'noreply@|example\.(com|org)' 2>/dev/null | head -5)
[ -n "$HITS" ] && report "e-mail address in a recording" $HITS

if [ $FOUND -eq 0 ]; then
  echo "recordings: clean" >&2
  exit 0
fi

echo >&2
echo "Re-record after fixing the shim's normalization — do not edit cassettes by hand." >&2
exit 1
