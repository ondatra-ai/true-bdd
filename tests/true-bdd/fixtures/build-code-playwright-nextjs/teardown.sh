#!/usr/bin/env bash
# Playwright's webServer brings up a docker compose stack that outlives the
# CLI process; without this its container keeps its port and its image stays.
#
# The compose project is this copy's own: tests/playwright.config.ts names it
# from the sha256 of the tests/ directory's physical path, and so does this,
# so a parallel run's stack is never the one taken down.
#
# Best-effort by contract: runs after the post-run snapshot whatever the
# verdict, and its failure never masks the primary one.
project="bdd-$(printf '%s' "$(pwd -P)/tests" | shasum -a 256 | cut -c1-12)"
COMPOSE_PROJECT_NAME="$project" docker compose down -v --remove-orphans --rmi local
