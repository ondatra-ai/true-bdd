// Package disk owns the engine's filesystem byte access. Reads take a shared
// parent-directory flock; mutations take an exclusive one, released before
// returning. Whole-file writes stage and atomically rename while preserving
// existing permissions. Errors retain their identity and never terminate or
// log. The independent tests/tooling implementation remains outside the engine.
package disk
