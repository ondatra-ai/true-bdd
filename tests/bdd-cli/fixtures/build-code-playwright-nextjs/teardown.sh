#!/usr/bin/env bash
# Playwright's webServer brings up a docker compose stack that outlives the
# CLI process; without this the frontend container keeps port 3000 and the
# next run of this fixture fails for a reason nobody wrote.
#
# Best-effort by contract: runs after the post-run snapshot whatever the
# verdict, and its failure never masks the primary one.
docker compose down -v --remove-orphans
