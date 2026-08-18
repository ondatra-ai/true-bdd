#!/usr/bin/env bash
# Shared diff-context builder for the scripts that pipe a diff into `claude -p`.
#
# Source it, then call:
#
#   emit_diff_context --cached
#   emit_diff_context origin/main...HEAD
#
# Why it exists: PR #70 was 276 files and 3.45 MB of diff. Both
# pr-update.sh and pr-commit's commit.sh piped that whole thing to
# `claude -p`, which answered "Prompt is too long" — no PR, no commit
# message, and (because the output was redirected to a file) no visible
# error either. See ClickUp 86cb6g6q8.
#
# The shape of the fix: the --stat is small, structured, and carries the
# full scope of the change, so it always goes in complete. The diff BODY
# is what has to be bounded.
#
# Excluding the bulk-by-nature paths is not enough on its own — on #70
# they take 3.45 MB down to 1.03 MB, still far past the limit — so the
# byte cap is the load-bearing part and the exclusions just spend the
# budget on files that carry signal.

# Roughly 50k tokens of diff, which leaves generous headroom.
DIFF_BUDGET_BYTES="${DIFF_BUDGET_BYTES:-200000}"

# Recorded cassettes are machine-written JSON on very long single lines,
# and doc-universe.html embeds base64 fonts. Neither tells a model
# anything about what the change means. They stay in the --stat.
DIFF_BULK_EXCLUDES=(
  ':(exclude)tests/bdd-cli/fixtures/*/cassettes/*'
  ':(exclude)docs/doc-universe.html'
)

_human_bytes() {
  awk -v b="$1" 'BEGIN{
    if (b >= 1048576) printf "%.1f MB", b/1048576;
    else if (b >= 1024) printf "%.0f KB", b/1024;
    else printf "%d B", b;
  }'
}

# emit_diff_context <git-diff-revision-argv...>
emit_diff_context() {
  local scope="$*"
  local workdir=./tmp
  mkdir -p "$workdir"

  local full capped full_bytes capped_bytes
  full=$(mktemp "$workdir/diff-full.XXXXXX")
  capped=$(mktemp "$workdir/diff-capped.XXXXXX")

  git diff "$@" >"$full"
  full_bytes=$(wc -c <"$full" | tr -d ' ')

  # head -c on a FILE, never on a pipe: `git diff | head -c N` makes git
  # die of SIGPIPE, and `set -o pipefail` in the callers turns that into
  # a failed script.
  git diff "$@" -- . "${DIFF_BULK_EXCLUDES[@]}" >"$capped"
  capped_bytes=$(wc -c <"$capped" | tr -d ' ')

  echo "=== Diff stat ($scope) — COMPLETE, every file in the change ==="
  git diff "$@" --stat
  echo ""

  if [ "$full_bytes" -le "$DIFF_BUDGET_BYTES" ]; then
    echo "=== Diff ($scope) ==="
    cat "$full"
  elif [ "$capped_bytes" -le "$DIFF_BUDGET_BYTES" ]; then
    # The common case after a re-record: enormous in cassettes, small in
    # the files a person wrote. Nothing is truncated, so say that — the
    # truncation branch below would print a NEGATIVE "not shown" figure
    # and claim a prefix that is actually the whole thing.
    echo "=== Diff ($scope) — COMPLETE for every file below ==="
    echo "=== Recorded cassettes and docs/doc-universe.html are omitted here; they ARE in the stat above. ==="
    echo ""
    cat "$capped"
  else
    # Say so in the text the model reads. A model told it is looking at a
    # prefix leans on the stat above; one that is not told will describe
    # the prefix as if it were the whole change.
    echo "=== Diff ($scope) — TRUNCATED to $(_human_bytes "$DIFF_BUDGET_BYTES") of $(_human_bytes "$full_bytes") ==="
    echo "=== Recorded cassettes and docs/doc-universe.html are omitted here; they ARE in the stat above. ==="
    echo "=== You are reading a PREFIX of the diff. Use the stat above for the full scope of the change. ==="
    echo ""
    head -c "$DIFF_BUDGET_BYTES" "$capped"
    echo ""
    echo "=== [diff truncated here — $(_human_bytes $((capped_bytes - DIFF_BUDGET_BYTES))) not shown] ==="
  fi

  rm -f "$full" "$capped"
}

# run_claude_or_explain <role> <prompt> <outfile>  — stdin is the context.
#
# The old failure was silent from the caller's side: `claude -p` wrote
# "Prompt is too long" into the redirect target and exited non-zero,
# `set -euo pipefail` killed the script, and nothing reached the terminal.
# This names the reason on stderr.
run_claude_or_explain() {
  local role="$1" prompt="$2" outfile="$3" rc=0

  set +e
  CLAUDE_HISTORY_ROLE="$role" claude -p "$prompt" >"$outfile" 2>"$outfile.err"
  rc=$?
  set -e

  # The EXIT STATUS decides whether the call failed. "Prompt is too long"
  # only ever explains a failure — it must never declare one, because the
  # model's own output can legitimately contain the phrase. It did on the
  # very first run of this helper: the PR body describing this bug quoted
  # the error, and a successful call was reported as a failure.
  # Both streams are searched, since nothing pins the message to stdout.
  local too_long=1
  if [ "$rc" -ne 0 ]; then
    grep -qi 'prompt is too long' "$outfile" "$outfile.err" 2>/dev/null && too_long=0
  fi

  if [ "$rc" -ne 0 ] || [ ! -s "$outfile" ]; then
    {
      echo "claude -p ($role) failed — exit $rc."
      if [ "$too_long" -eq 0 ]; then
        echo "Reason: the prompt exceeded the model's context window."
        echo "The diff is capped at DIFF_BUDGET_BYTES=$DIFF_BUDGET_BYTES; lower it and retry."
      elif [ ! -s "$outfile" ]; then
        echo "Reason: claude returned nothing."
      fi
      # head FIRST, then sed. The other order makes sed die of SIGPIPE
      # when head closes early, and `set -o pipefail` in the callers turns
      # that into an abort — a failure report that swallows itself.
      echo "--- stdout ---"
      head -20 "$outfile" 2>/dev/null | sed 's/^/  /'
      echo "--- stderr ---"
      head -20 "$outfile.err" 2>/dev/null | sed 's/^/  /'
    } >&2
    rm -f "$outfile.err"
    return 1
  fi

  rm -f "$outfile.err"
  return 0
}
