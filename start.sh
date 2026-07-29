#!/usr/bin/env bash
#
# start.sh — launch Claude Code against a chosen model backend.
#
# Usage:
#   ./start.sh              # standard Anthropic (your normal `claude` login)
#   ./start.sh zai          # Z.AI  (GLM models)
#   ./start.sh kimi         # Kimi  (Moonshot K2 models)
#   ./start.sh <mode> ...   # any extra args are passed through to `claude`
#   ./start.sh help
#
# Secrets are read from a gitignored .env next to this script (see
# .env.example). All model ids are overridable from .env.
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load .env (KEY=VALUE lines; comments/blanks ignored) into the environment.
if [ -f "$ROOT/.env" ]; then
  set -a
  # shellcheck disable=SC1091
  . "$ROOT/.env"
  set +a
fi

MODE="${1:-standard}"
[ "$#" -gt 0 ] && shift   # remaining args pass through to `claude`

case "$MODE" in
  standard | anthropic)
    # Normal Anthropic login — strip any override that may be set so it
    # never silently leaks a third-party backend into a "standard" run.
    unset ANTHROPIC_BASE_URL ANTHROPIC_AUTH_TOKEN ANTHROPIC_MODEL \
      ANTHROPIC_DEFAULT_OPUS_MODEL ANTHROPIC_DEFAULT_SONNET_MODEL \
      ANTHROPIC_DEFAULT_HAIKU_MODEL || true
    echo "→ Claude Code: standard Anthropic" >&2
    ;;

  zai | z.ai | glm)
    : "${ZAI_API_KEY:?set ZAI_API_KEY in .env (get one at https://z.ai/manage-apikey/apikey-list)}"
    export ANTHROPIC_BASE_URL="${ZAI_BASE_URL:-https://api.z.ai/api/anthropic}"
    export ANTHROPIC_AUTH_TOKEN="$ZAI_API_KEY"
    export API_TIMEOUT_MS="${API_TIMEOUT_MS:-3000000}"
    # Z.AI's endpoint auto-maps Claude model ids to current GLM models, so we
    # only pin models when you explicitly set them in .env (keeps you on the
    # newest GLM by default).
    [ -n "${ZAI_OPUS_MODEL:-}" ]   && export ANTHROPIC_DEFAULT_OPUS_MODEL="$ZAI_OPUS_MODEL"
    [ -n "${ZAI_SONNET_MODEL:-}" ] && export ANTHROPIC_DEFAULT_SONNET_MODEL="$ZAI_SONNET_MODEL"
    [ -n "${ZAI_HAIKU_MODEL:-}" ]  && export ANTHROPIC_DEFAULT_HAIKU_MODEL="$ZAI_HAIKU_MODEL"
    echo "→ Claude Code: Z.AI (GLM) via $ANTHROPIC_BASE_URL" >&2
    ;;

  kimi | moonshot | k2)
    : "${KIMI_API_KEY:?set KIMI_API_KEY in .env (get one at https://platform.moonshot.ai/)}"
    export ANTHROPIC_BASE_URL="${KIMI_BASE_URL:-https://api.moonshot.ai/anthropic}"
    export ANTHROPIC_AUTH_TOKEN="$KIMI_API_KEY"
    export API_TIMEOUT_MS="${API_TIMEOUT_MS:-3000000}"
    # Moonshot does NOT auto-map Claude model ids, so every tier must point at
    # a real Kimi model or requests 404. Defaults are overridable in .env.
    export ANTHROPIC_DEFAULT_OPUS_MODEL="${KIMI_OPUS_MODEL:-kimi-k2-turbo-preview}"
    export ANTHROPIC_DEFAULT_SONNET_MODEL="${KIMI_SONNET_MODEL:-kimi-k2-turbo-preview}"
    export ANTHROPIC_DEFAULT_HAIKU_MODEL="${KIMI_HAIKU_MODEL:-kimi-k2-turbo-preview}"
    echo "→ Claude Code: Kimi (Moonshot K2) via $ANTHROPIC_BASE_URL" >&2
    ;;

  help | -h | --help)
    # Print the leading comment header (skip the shebang; stop at first code).
    awk 'NR==1 {next} /^#/ {sub(/^# ?/, ""); print; next} {exit}' "$ROOT/start.sh"
    exit 0
    ;;

  *)
    echo "start.sh: unknown mode '$MODE' (use: standard | zai | kimi | help)" >&2
    exit 1
    ;;
esac

exec claude "$@"
