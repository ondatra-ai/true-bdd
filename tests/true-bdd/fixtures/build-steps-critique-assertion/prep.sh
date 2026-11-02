set -eu
repo=$(git rev-parse --show-toplevel)
go mod edit -replace="github.com/ondatra-ai/true-bdd/pkg=$repo/pkg"
cp "$repo/pkg/go.sum" go.sum
# Keep the root immutable while the writer changes only tests/ and tmp/.
mkdir -p tmp
rm -f .DS_Store
chmod u-w .
