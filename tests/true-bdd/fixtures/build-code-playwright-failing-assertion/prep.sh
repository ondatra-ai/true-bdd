#!/usr/bin/env bash
# Installs what the CLI cannot: the Playwright runner this fixture's suite
# declares, and the browser it drives. Run in the tmpdir after the input
# overlay and before the pre-run snapshot, so neither the node_modules tree
# nor the browser download reaches the diff the run is graded on.
set -euo pipefail

npm install --prefix tests --no-audit --no-fund --silent
./tests/node_modules/.bin/playwright install chromium
