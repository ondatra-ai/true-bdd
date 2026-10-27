#!/bin/sh
set -eu
printf '%s\n' '{"Action":"run","Package":"fixture/tests/suite","Test":"TestBytes"}'
printf '%s' '{"Action":"pass","Package":"fixture/tests/suite","Test":"TestBytes"}'
printf '  stderr\tbytes  ' >&2
