// Package merge merges a pull request: up to three review rounds, then land it.
//
// One package, one straight line, no resume, no flags:
//
//	for round in 1, 2, 3:
//	    requestReview()        # and wait out CodeRabbit's rate limit
//	    findings = readComments()
//	    scored   = triage(findings)
//	    fix / file a ticket / ignore, by score
//	    resolve every thread
//	    break if this round changed nothing, else commit
//	merge
//	postmortem
//
// Rounds 1 and 2 fix what scores 9-10 and file 6-8 as ClickUp tickets. Round 3
// fixes nothing — everything >= 6 becomes a ticket.
//
// A round that changes no file ends the loop, because the next round would buy
// a review of a byte-identical tree at a quarter of the hourly quota. That is
// also what makes the approval honest, whichever round it happens in: the loop
// only ever exits after a round that left HEAD where its own review found it,
// so the commit that was reviewed is the commit that gets approved.
//
// Nothing here is swallowed. The predecessor wrapped every round in
// `except (Exception, SystemExit)` so the loop always reached a merge; it
// reached one on PR #76 with eight recorded anomalies and a red preflight.
// This one stops and says why. The single exception is CodeRabbit's rate
// limit, which is not a failure — it is a wait.
package merge
