package clickup

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// dupeFloor is where two filed tickets are one proposal rather than related
// ones. Above the gate's ceiling on purpose: blocking a candidate at 4 costs a
// re-file, retiring a ticket at 4 loses work nobody else is carrying.
const dupeFloor = 8

// cluster is a set of filed tickets the turn judged to be one proposal.
type cluster struct {
	Keeper  CorpusRow
	Losers  []CorpusRow
	Reasons map[string]string
}

// edge is one judged relation between two filed tickets.
type edge struct {
	From   string
	To     string
	Score  int
	Reason string
}

// Dupes ranks filed tickets against the rest of the tracker and writes the
// report. Report-only: it holds no mutating tool, and retiring a duplicate
// stays a decision a person makes from the file this leaves behind.
func Dupes(ids []string) error {
	rows, err := dumpCorpus()
	if err != nil {
		return err
	}

	subjects := subjectsOf(rows, ids)
	if len(subjects) == 0 {
		return fmt.Errorf("%w: nothing to audit in %s", errNoSuchTicket, listID())
	}

	slog.Info("Auditing for duplicates", "subjects", len(subjects), "corpus", len(rows))

	edges := audit(subjects, judge{byID: index(rows)}.rank)
	clusters := clustersOf(edges, index(rows))

	slog.Info("Audit complete", "clusters", len(clusters), "pairs", len(edges))

	return writeReport(clusters, len(subjects))
}

// subjectsOf is what gets audited: the named tickets, or every ticket a person
// could still be asked to work.
func subjectsOf(rows []CorpusRow, ids []string) []CorpusRow {
	subjects := make([]CorpusRow, 0, len(rows))

	for _, row := range rows {
		wanted := row.Status == backlogStatus || row.Status == queuedStatus
		if len(ids) > 0 {
			wanted = slices.Contains(ids, row.ID)
		}

		if wanted {
			subjects = append(subjects, row)
		}
	}

	return subjects
}

// audit ranks each subject against the corpus and keeps the pairs at or above
// dupeFloor. A subject that cannot be ranked is named and the audit goes on:
// the rest are independent, and a report short one row still retires the rest.
func audit(subjects []CorpusRow, rank ranker) []edge {
	var edges []edge

	for _, subject := range subjects {
		matches, err := rank(findingOf(subject), subject.ID)
		if err != nil {
			slog.Error("A ticket could not be ranked and was left out of the report",
				"ticket", subject.ID, "title", subject.Name, "error", err)

			continue
		}

		for _, match := range matches {
			if match.Score < dupeFloor {
				continue
			}

			edges = append(edges, edge{
				From: subject.ID, To: match.ID,
				Score: match.Score, Reason: match.Reason,
			})
		}
	}

	return edges
}

// findingOf is a filed ticket, shaped as the candidate the turn judges.
func findingOf(row CorpusRow) Finding {
	return Finding{ID: row.ID, Title: row.Name, Body: row.Description}
}

// clustersOf folds the judged pairs into connected sets and picks each one's
// keeper.
func clustersOf(edges []edge, byID map[string]CorpusRow) []cluster {
	reasons := make(map[string]string, len(edges))
	for _, judged := range edges {
		reasons[judged.To] = judged.Reason
		reasons[judged.From] = judged.Reason
	}

	clusters := make([]cluster, 0, len(edges))

	for _, ids := range components(edges) {
		members := make([]CorpusRow, 0, len(ids))

		for _, id := range ids {
			row, held := byID[id]
			if held {
				members = append(members, row)
			}
		}

		clusters = append(clusters, clusterOf(members, reasons))
	}

	return clusters
}

// clusterOf splits one connected set into its keeper and its losers.
func clusterOf(members []CorpusRow, reasons map[string]string) cluster {
	keeper := keeperOf(members)

	losers := make([]CorpusRow, 0, len(members)-1)

	for _, row := range members {
		if row.ID != keeper.ID {
			losers = append(losers, row)
		}
	}

	return cluster{Keeper: keeper, Losers: losers, Reasons: reasons}
}

// components walks the pair graph and returns each connected set of ids,
// sorted so two runs over the same answer write the same report.
func components(edges []edge) [][]string {
	next := make(map[string][]string, len(edges))

	for _, judged := range edges {
		next[judged.From] = append(next[judged.From], judged.To)
		next[judged.To] = append(next[judged.To], judged.From)
	}

	ids := make([]string, 0, len(next))
	for id := range next {
		ids = append(ids, id)
	}

	slices.Sort(ids)

	seen := make(map[string]struct{}, len(ids))

	var found [][]string

	for _, ticket := range ids {
		_, done := seen[ticket]
		if done {
			continue
		}

		found = append(found, reach(ticket, next, seen))
	}

	return found
}

// reach collects everything connected to start, breadth first.
func reach(start string, next map[string][]string, seen map[string]struct{}) []string {
	queue := []string{start}
	seen[start] = struct{}{}

	var members []string

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		members = append(members, id)

		for _, peer := range next[id] {
			_, done := seen[peer]
			if done {
				continue
			}

			seen[peer] = struct{}{}
			queue = append(queue, peer)
		}
	}

	slices.Sort(members)

	return members
}

// keeperRanks orders the statuses a keeper may hold, weakest first. `done`
// wins outright — the work shipped, so every open copy asks for it again —
// then `to do`, which is a promotion a person made and the sweep never undoes.
func keeperRanks() []string {
	return []string{notRelevantStatus, failedStatus, backlogStatus, queuedStatus, doneStatus}
}

// keeperOf picks the copy that survives a cluster.
func keeperOf(members []CorpusRow) CorpusRow {
	best := members[0]

	for _, row := range members[1:] {
		if outranks(row, best) {
			best = row
		}
	}

	return best
}

// outranks is the keeper rule in order: what shipped, what a person promoted,
// the higher score, the fresher judgement, then the older ticket — whose url
// is the one anything outside ClickUp would already cite.
func outranks(row, best CorpusRow) bool {
	if slices.Index(keeperRanks(), row.Status) != slices.Index(keeperRanks(), best.Status) {
		return slices.Index(keeperRanks(), row.Status) > slices.Index(keeperRanks(), best.Status)
	}

	if scoreOf(row) != scoreOf(best) {
		return scoreOf(row) > scoreOf(best)
	}

	if row.TriageDate != best.TriageDate {
		return row.TriageDate > best.TriageDate
	}

	return row.Created < best.Created
}

// scoreOf reads the transcribed Triage Score back. An unset or unreadable one
// is 0, which loses to every real score rather than beating a low one.
func scoreOf(row CorpusRow) int {
	score, err := strconv.Atoi(strings.TrimSpace(row.TriageScore))
	if err != nil {
		return 0
	}

	return score
}

// writeReport lands the artifact a person reads before anything is retired —
// the render-before-upload rule the filing path already follows.
func writeReport(clusters []cluster, audited int) error {
	var built strings.Builder

	fmt.Fprintf(&built, `# Duplicate report

%d ticket(s) audited. %d cluster(s) at identity %d or above.

The keeper of each cluster is the copy that survives: what shipped, then what
a person promoted, then the higher Triage Score, the fresher Triage Date, and
failing all of those the older ticket, whose url anything outside ClickUp
would already cite.
`, audited, len(clusters), dupeFloor)

	for number, found := range clusters {
		built.WriteString(found.render(number + 1))
	}

	if len(clusters) == 0 {
		built.WriteString("\nNothing was judged a duplicate.\n")
	}

	err := disk.Write(DupesReport, []byte(built.String()), disk.Shared)
	if err != nil {
		return fmt.Errorf("writing %s: %w", DupesReport, err)
	}

	slog.Info("Report written", "clusters", len(clusters), "path", DupesReport)

	return nil
}

// render is one cluster: the keeper, then every copy to retire and the comment
// that retires it.
func (c cluster) render(number int) string {
	var built strings.Builder

	fmt.Fprintf(&built, "\n## %d. %s\n\n**Keep** %s — %s\n\nRetire:\n\n",
		number, c.Keeper.Name, c.Keeper.line(), c.Keeper.URL)

	for _, loser := range c.Losers {
		fmt.Fprintf(&built, "- %s — %s\n  - %s\n  - why: %s\n  - comment: %s\n",
			loser.Name, loser.line(), loser.URL,
			orUnknown(c.Reasons[loser.ID]), retirement(c.Keeper))
	}

	return built.String()
}

// line is the one-line identity of a ticket in the report.
func (r CorpusRow) line() string {
	return fmt.Sprintf("`%s`, %s, score %s, triaged %s",
		r.ID, r.Status, orUnknown(r.TriageScore), orUnknown(r.TriageDate))
}

// retirement is the comment a loser is retired with, ready to paste. Prose
// rather than a field: the MCP layer cannot write a tasks-type relationship
// (400 FIELD_342, probed 2026-09-02), so the marker is a comment.
func retirement(keeper CorpusRow) string {
	return "Duplicate of " + keeper.URL + " (" + keeper.ID +
		"). Retired by the duplicate sweep; the keeper carries this proposal."
}
