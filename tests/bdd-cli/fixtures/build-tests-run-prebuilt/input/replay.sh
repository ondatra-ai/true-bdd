#!/bin/sh
set -eu
engine_bin=
for arg in "$@"; do
  case "$arg" in
    -engine-bin=*) engine_bin=${arg#-engine-bin=} ;;
  esac
done
if [ -z "$engine_bin" ]; then
  mkdir -p tmp
  touch tmp/service-build-attempted
  go build -o ./tmp/service ./services/calc
fi
[ ! -d services/calc ]
exec go test -tags bdd -count=1 ./tests/calc/ "$@"
