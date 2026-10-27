#!/bin/sh
set -eu
exec go test -tags bdd -count=1 ./tests/... "$@"
