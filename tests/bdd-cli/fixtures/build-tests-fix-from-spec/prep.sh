set -eu
repo=$(git rev-parse --show-toplevel)
go mod edit -replace="github.com/ondatra-ai/true-bdd=$repo"
cp "$repo/go.sum" go.sum
