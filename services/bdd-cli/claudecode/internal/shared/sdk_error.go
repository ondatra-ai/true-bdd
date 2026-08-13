// Package shared provides shared types and interfaces used across internal packages.
package shared

// SDKError is the base interface for all Claude Code SDK errors.
type SDKError interface {
	error
	Type() string
}
