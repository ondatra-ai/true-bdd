package engine

// fixLimit is a walk's bound on one query's fixes, defaultMaxApplyAttempts when it configures none.
func fixLimit(configured int) int {
	if configured <= 0 {
		return defaultMaxApplyAttempts
	}

	return configured
}
