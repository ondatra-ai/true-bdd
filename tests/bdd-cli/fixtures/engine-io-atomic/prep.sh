#!/bin/sh
set -eu
mkdir -p tmp
chmod 600 tests/mcp/ticket_test.go
printf 'sentinel\n' > tmp/staging-target.txt
