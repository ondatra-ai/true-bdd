set -eu
repo=$(git rev-parse --show-toplevel)
go mod edit -replace="github.com/ondatra-ai/true-bdd/pkg=$repo/pkg"
cp "$repo/pkg/go.sum" go.sum
chmod +x tests/calc/fixtures/offline/bin/go
