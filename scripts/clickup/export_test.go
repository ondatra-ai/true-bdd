package clickup

import (
	"encoding/json"

	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// The field plan is unexported and reaches the filing turn as JSON, so the
// test asserts against those bytes through this export_test.go seam, which
// the compiler drops from any non-test build.

// PlanFieldsForTest is the plan the filing prompt carries, and
// PlanStampsForTest the shorter one a deferral carries. The stamp is the
// caller's: now() reads the clock and HEAD, which a test cannot pin.
func PlanFieldsForTest(queue []Finding, millis int64, commit string) []byte {
	return encodePlan(planFields(queue, stamp{Millis: millis, Commit: commit}))
}

func PlanStampsForTest(queue []Finding, millis int64, commit string) []byte {
	return encodePlan(planStamps(queue, stamp{Millis: millis, Commit: commit}))
}

func encodePlan(plans []fieldPlan) []byte {
	encoded, err := json.MarshalIndent(plans, "", "  ")
	if err != nil {
		panic(err)
	}

	return encoded
}

// DropAlreadyOpenForTest is the dedupe filter FileDeduped runs before the
// render; the listing turn it feeds on cannot run in a test.
func DropAlreadyOpenForTest(queue []Finding, open []Task) ([]Finding, []Task) {
	return dropAlreadyOpen(queue, open)
}

// MisplacedForTest is the backlog check report runs; the filing turn that
// produces the statuses it reads cannot run in a test.
func MisplacedForTest(filed []Ticket) []string {
	return misplaced(filed)
}

// SelectStaleForTest is the order the sweep walks in; the listing turn that
// produces the rows cannot run in a test.
func SelectStaleForTest(listed []Task, count int) []Task {
	return selectStale(listed, count)
}

// DispositionForTest is what a verdict means for a ticket's status.
func DispositionForTest(verdict triage.Verdict, was string) string {
	return dispositionOf(verdict, was)
}

// ApplyPromptForTest is the update turn's prompt, as it is sent.
func ApplyPromptForTest(ticket Task, verdict triage.Verdict, millis int64, commit string) string {
	taken := stamp{Millis: millis, Commit: commit}
	status := dispositionOf(verdict, ticket.Status)

	return applyPrompt(ticket, verdict, taken, status,
		noteOf(prior{}, verdict, taken, ticket.Status, status))
}

// NoteForTest is the record a triage leaves, over a prior score in the raw
// shape the bodies turn transcribes it in.
func NoteForTest(was string, verdict triage.Verdict, commit, from string) string {
	taken := stamp{Millis: 1, Commit: commit}

	return noteOf(prior{Score: was}, verdict, taken, from, dispositionOf(verdict, from))
}

// ApplySchemaForTest is the shape the update turn is held to.
func ApplySchemaForTest() string {
	return applySchema
}

// WalkPromptForTest is the listing turn's prompt, as it is sent.
func WalkPromptForTest() string {
	return walkPrompt()
}

// RaiserForTest is who a ticket credits, from the finding's source.
func RaiserForTest(source string) string {
	return raiserOf(source)
}

// CountHeadingsForTest is the count FileDocument tells the filing turn to
// create; the turn itself cannot run in a test.
func CountHeadingsForTest(document string) int {
	return countHeadings(document)
}

// DocumentPromptForTest is the document turn's prompt, as it is sent — built
// from sections a fake scorer has already judged, since the turn cannot run.
func DocumentPromptForTest(document, tag string, score int) string {
	kept := TriageSectionsForTest(document, func(_ triage.Subject) (triage.Verdict, error) {
		return triage.Verdict{Score: score, Reason: "why", Description: "### Why\n\nrefreshed."}, nil
	})

	prompt, err := documentPrompt(kept, tag, stamp{Millis: 1, Commit: "abc"})
	if err != nil {
		panic(err)
	}

	return prompt
}

// TriageSectionsForTest is the keep-or-drop decision, over a fake scorer.
func TriageSectionsForTest(document string, score scorer) []section {
	return triageSections(splitSections(document), score)
}

// SectionTitlesForTest names what survived, in order.
func SectionTitlesForTest(kept []section) []string {
	titles := make([]string, 0, len(kept))
	for _, held := range kept {
		titles = append(titles, held.Title)
	}

	return titles
}

// TicketStatusForTest is the status ticket.yaml declares, and
// StatusRuleForTest the sentence both filing turns carry to state it.
func TicketStatusForTest() string {
	return ticketStatus()
}

func StatusRuleForTest() string {
	return statusRule()
}

// HeadingNamesForTest is the heading order ticket.yaml declares. Now a
// delegation: Headings is exported, so the seam must not be a second copy.
func HeadingNamesForTest() []string { return Headings() }

// FieldIDs is every custom-field UUID this package writes, by field name, so
// the conformance test can hold them against ticket-schema.yaml.
func FieldIDs() map[string]string {
	return map[string]string{
		"Triage Score":     triageScoreField,
		"Expected Changes": expectedChangesField,
		"Triage Date":      triageDateField,
		"Triage Commit":    triageCommitField,
	}
}

// ListPromptForTest is the queue listing turn's prompt, as it is sent.
func ListPromptForTest(tag string) string {
	return listPrompt(tag)
}

// RankPromptForTest is the similarity turn's prompt, as it is sent.
func RankPromptForTest(candidate Finding, exclude string) string {
	return rankPrompt(candidate, exclude)
}

// ResolveMatchesForTest is the check on an answer the schema cannot make; the
// turn that produces the matches cannot run in a test.
func ResolveMatchesForTest(rows []CorpusRow, matches []Match, exclude string) ([]Match, error) {
	return judge{byID: index(rows)}.resolve(matches, exclude)
}

// RunGateForTest is the three dedup passes over a corpus and a ranker the test
// supplies; neither the dump nor the turn can run in a test.
func RunGateForTest(queue []Finding, rows []CorpusRow,
	rank func(Finding, string) ([]Match, error),
) ([]Finding, error) {
	return runGate(queue, rows, rank)
}

// DecideForTest is what a gate that could not answer means, by caller.
func DecideForTest(queue []Finding, failure error, strict bool) ([]Finding, error) {
	return decide(queue, nil, failure, strict)
}

// KeeperForTest is the copy that survives a cluster.
func KeeperForTest(members []CorpusRow) string {
	return keeperOf(members).ID
}

// ClustersForTest folds judged pairs into clusters, each one named keeper
// first — the shape the report is built from.
func ClustersForTest(rows []CorpusRow, pairs [][2]string) [][]string {
	edges := make([]edge, 0, len(pairs))
	for _, judged := range pairs {
		edges = append(edges, edge{
			From: judged[0], To: judged[1], Score: dupeFloor, Reason: "the same proposal",
		})
	}

	clusters := clustersOf(edges, index(rows))
	found := make([][]string, 0, len(clusters))

	for _, group := range clusters {
		members := []string{group.Keeper.ID}
		for _, loser := range group.Losers {
			members = append(members, loser.ID)
		}

		found = append(found, members)
	}

	return found
}

// CorpusEntryForTest is one ticket as it lands in CorpusDir.
func CorpusEntryForTest(row CorpusRow) string {
	return row.render()
}

// SurvivingSectionsForTest is which of a document's sections a gate's answer
// keeps; the gate's own turns cannot run in a test.
func SurvivingSectionsForTest(document string, kept []Finding) []string {
	return SectionTitlesForTest(surviving(splitSections(document), kept))
}
