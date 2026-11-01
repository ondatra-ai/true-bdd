package engine

// fixLimit is a walk's bound on one query's fixes, defaultMaxApplyAttempts when it configures none.
func fixLimit(configured int) int {
	if configured <= 0 {
		return defaultMaxApplyAttempts
	}

	return configured
}

// isGate reports whether a failure of the query skips the item's others.
func isGate[Q any](gate func(Q) bool, query Q) bool {
	return gate != nil && gate(query)
}

// skippedByGate reports a query an earlier failed gate skips, telling onSkip of it.
func skippedByGate[I, Q any](gateFailed bool, gate func(Q) bool, onSkip func(I, Q), item I, query Q) bool {
	if !gateFailed || isGate(gate, query) {
		return false
	}

	if onSkip != nil {
		onSkip(item, query)
	}

	return true
}
