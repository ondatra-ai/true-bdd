package clickup

import (
	"cmp"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// The statuses a sweep walks. `to do` is the queue task-loop works unattended,
// which makes a stale ticket there the expensive kind; the closed statuses and
// `not relevant` are already settled.
const (
	backlogStatus     = "backlog"
	queuedStatus      = "to do"
	notRelevantStatus = "not relevant"
	doneStatus        = "done"
	failedStatus      = "failed"
)

// Triage re-judges the count least-recently-triaged tickets against HEAD.
func Triage(count int) error {
	if count < 1 {
		return fmt.Errorf("%w: %d", errNothingToTriage, count)
	}

	stale, err := staleTickets(count)
	if err != nil {
		return err
	}

	if len(stale) == 0 {
		slog.Info("Nothing to triage", "statuses", backlogStatus+", "+queuedStatus)

		return nil
	}

	bodies, err := fetchBodies(stale)
	if err != nil {
		return err
	}

	return sweep(stale, bodies)
}

// staleTickets lists every walkable ticket and picks the oldest-triaged. The
// sort is Go's: ClickUp cannot order by a custom field, so the turn transcribes
// and this decides.
func staleTickets(count int) ([]Task, error) {
	listed, err := walkableTasks()
	if err != nil {
		return nil, err
	}

	slog.Info("Tickets walked", "listed", len(listed), "taking", min(count, len(listed)))

	return selectStale(listed, count), nil
}

// selectStale orders by Triage Date, oldest first, and takes count of them. A
// ticket that was never triaged sorts before every one that was — the point of
// the stamp is that the sweep advances rather than re-walking the same rows.
func selectStale(listed []Task, count int) []Task {
	ordered := slices.Clone(listed)

	// Raw milliseconds, transcribed: 13 fixed-width digits sort lexically the
	// way they sort numerically, and "" sorts before all of them. ClickUp
	// stores the DAY (11:18Z came back as the day start), so same-day ties.
	slices.SortStableFunc(ordered, func(left, right Task) int {
		if left.TriageDate != right.TriageDate {
			return cmp.Compare(left.TriageDate, right.TriageDate)
		}

		return cmp.Compare(left.Created, right.Created)
	})

	return ordered[:min(count, len(ordered))]
}

// sweep scores each ticket and applies the verdict, one at a time. A ticket
// that fails is named and the sweep goes on: the rest are independent, and
// stopping would leave the run half-stamped with nothing saying where.
func sweep(stale []Task, bodies map[string]prior) error {
	applied := 0

	for _, ticket := range stale {
		// A body that did not come back would be judged on its title alone,
		// and a ticket retired on a title is a ticket lost to a failed read.
		// A missing Score is not that: it costs an arrow in the note, no more.
		was := bodies[ticket.ID]
		if strings.TrimSpace(was.Description) == "" {
			slog.Error("A ticket's description could not be read and it was left alone",
				"ticket", ticket.ID, "title", ticket.Name)

			continue
		}

		verdict, err := triage.Score(subjectOfTicket(ticket, was.Description))
		if err != nil {
			slog.Error("A ticket could not be scored and was left alone",
				"ticket", ticket.ID, "title", ticket.Name, "error", err)

			continue
		}

		err = apply(ticket, was, verdict, now())
		if err != nil {
			slog.Error("A ticket was scored but not updated",
				"ticket", ticket.ID, "error", err)

			continue
		}

		applied++
	}

	slog.Info("Sweep complete", "triaged", applied, "walked", len(stale))

	if applied < len(stale) {
		return fmt.Errorf("%w: %d of %d", errNotTriaged, len(stale)-applied, len(stale))
	}

	return nil
}

// subjectOfTicket is where this caller's subject comes from. Filed is set here
// and nowhere else: this is the ONE updater, and the flag is what forbids it
// growing a ticket somebody already filed into a newer shape.
func subjectOfTicket(ticket Task, body string) triage.Subject {
	return triage.Subject{
		ID:     ticket.ID,
		Title:  ticket.Name,
		Body:   body,
		Origin: "ClickUp " + ticket.ID + ", last triaged " + orNever(ticket.TriageDate),
		Filed:  true,
	}
}

// orNever reads the stamp back for the prompt. Empty is not "the epoch": it is
// a ticket filed before anything wrote the field at all.
func orNever(triageDate string) string {
	if triageDate == "" {
		return "never"
	}

	return triageDate
}

// dispositionOf is what the verdict means for the ticket: retire it, or leave
// its status exactly where a person put it.
func dispositionOf(verdict triage.Verdict, was string) string {
	if verdict.Score < triage.Floor {
		return notRelevantStatus
	}

	return was
}
