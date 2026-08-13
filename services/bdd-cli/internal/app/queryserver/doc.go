// Package queryserver is the v2 CLI-side of the register / poll / reply
// protocol (plan §2): the reconnecting client (typed HTTP errors, re-register
// with the same client-owned session id on poll 404/stale-epoch, capped
// full-jitter backoff) and the bounded worker pool (mutations serialized,
// ≤4 concurrent DB reads, 1 inventory scan, mutation priority answer >
// dispatch > reads).
//
// The implementation lands here in the query-server phase. The tests-first
// suite pins the target contract via the in-test seam in seam_test.go (nil
// hooks ⇒ RED until wired), so `go build ./...` / `go vet ./...` pass today.
package queryserver
