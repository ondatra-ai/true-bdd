#!/usr/bin/env bash
# Installs what the CLI cannot: the Jest runner this fixture's suite
# declares. Run in the tmpdir after the input overlay and before the
# pre-run snapshot, so the node_modules tree never reaches the diff the
# run is graded on.
set -euo pipefail

npm install --prefix tests --no-audit --no-fund --silent
