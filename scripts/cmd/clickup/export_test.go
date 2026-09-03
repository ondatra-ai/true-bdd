package main

// SweepCountForTest is the count-or-tickets decision `triage` makes; both
// branches below it reach ClickUp and cannot run in a test.
func SweepCountForTest(args []string) (int, bool) { return sweepCount(args) }
