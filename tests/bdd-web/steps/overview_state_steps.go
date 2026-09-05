package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrUnknownSessionState is returned when a clause names a reported state this
// file establishes no setup for.
var ErrUnknownSessionState = errors.New("no setup for that reported state")

// ErrNoInventory is returned when a clause is about the reported inventory and
// the session's detail read carried none.
var ErrNoInventory = errors.New("the session detail carries no inventory")

const (
	// reportTimeout is how long the session has to REPORT a state a Given
	// established: the detail view is a live read the remote answers, so the
	// first read after the setup is a read before it.
	reportTimeout = 60 * time.Second
	// probeCommand is what a Given dispatches to hold a run open — prompt-probe
	// blocks on its own prompt, so the run stays active for the whole scenario.
	probeCommand = "prompt-probe"
	// ownRunLabel and siblingRunLabel file the runs these Givens dispatch, and
	// siblingLabel the second remote one of them starts.
	ownRunLabel     = "own"
	siblingRunLabel = "sibling"
	siblingLabel    = "sibling"
	// inventoryBudgetSetting is the relay setting that negotiates the remote's
	// snapshot-fit budget, and truncatingInventoryBytes the value E2E-096
	// truncates a real project tree with.
	inventoryBudgetSetting   = "MAX_INVENTORY_REQUEST_BYTES"
	truncatingInventoryBytes = "5120"
	// canonicalArchitectureRel is the path the scanner calls canonical, and
	// mirroredArchitectureRel where the divergence Given declares the spec
	// instead.
	canonicalArchitectureRel = "docs/architecture/architecture.yaml"
	mirroredArchitectureRel  = "docs/architecture/mirrored-architecture.yaml"
	// inventoryRowTestID, inventoryChipTestID and inventoryStatusAttribute are
	// the overview's inventory-health contract, as E2E-181 spells it.
	inventoryRowTestID       = "overview-inventory-row"
	inventoryChipTestID      = "overview-inventory-chip"
	inventoryStatusAttribute = "data-status"
)

// activeRun is the open run a session reports as its own.
type activeRun struct {
	RunID   string `json:"run_id"`
	OwnerID string `json:"owner_id"`
	Command string `json:"command"`
	State   string `json:"state"`
}

// sessionOwner is another owner holding a run in the same folder — owner_id IS
// that owner's session id.
type sessionOwner struct {
	OwnerID string `json:"owner_id"`
	RunID   string `json:"run_id"`
	Command string `json:"command"`
}

// inventoryReport is the part of the live scan the overview's clauses read.
type inventoryReport struct {
	Documents                map[string]string `json:"documents"`
	ArchitecturePathMismatch bool              `json:"architecture_path_mismatch"`
	SnapshotTruncated        bool              `json:"snapshot_truncated"`
	Unavailable              string            `json:"unavailable"`
}

// sessionDetail is GET /api/sessions/<id>: the session's own active run, the
// same-project owners beside it, and the inventory it scanned for that read.
type sessionDetail struct {
	SessionID string     `json:"session_id"`
	ActiveRun *activeRun `json:"active_run"`
	// Runs is the project's run history, newest first, which "the dispatched run"
	// is resolved out of once the page's action has been filed.
	Runs         []runSummary     `json:"runs"`
	ActiveOwners []sessionOwner   `json:"active_owners"`
	Inventory    *inventoryReport `json:"inventory"`
}

// registerOverviewStateSteps binds what the SESSION reports and the overview
// then renders: a run of its own, a sibling owner, a diverged architectural
// spec, an inventory that did not fit, and the health list built from it.
func registerOverviewStateSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the session reports (an active run of its own|`+
		`a same-project sibling owner|an architecture path divergence|`+
		`a truncated inventory)$`, reportSessionState)
	suite.Step(`^every inventory entry has a row with a chip carrying a known status$`,
		assertInventoryRows)
	suite.Step(`^every inventory chip has settled onto a live status$`, awaitSettledChips)
	// One measurement, two phrasings: both hold every chip to the scanner's own
	// status vocabulary, so the definition is reused rather than copied.
	suite.Step(`^every inventory chip carries a known status$`, awaitSettledChips)
	registerOverviewChipSteps(suite)
}

// knownInventoryStatuses is the document-chip vocabulary the scanner reports; a
// chip carrying anything else is a status no reader can be held to.
func knownInventoryStatuses() []string {
	return []string{"present", "missing", "invalid", "not_a_dir", "present_empty"}
}

// reportSessionState establishes the state the clause names AND holds the
// session to reporting it: a Given that only acts leaves the When racing the
// read it depends on.
func reportSessionState(state *State, args []string) error {
	switch args[0] {
	case "an active run of its own":
		return reportActiveRun(state)
	case "a same-project sibling owner":
		return reportSiblingOwner(state)
	case "an architecture path divergence":
		return reportPathDivergence(state)
	case "a truncated inventory":
		return reportTruncatedInventory(state)
	default:
		return state.fail("%w: %q", ErrUnknownSessionState, args[0])
	}
}

// reportActiveRun dispatches the probe run on the session's own remote and
// waits for the session to carry it as its active one.
func reportActiveRun(state *State) error {
	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	err = postDispatch(state, session, probeCommand, ownRunLabel)
	if err != nil {
		return err
	}

	return awaitReport(state, session, "an active run of its own",
		func(detail *sessionDetail) bool { return detail.ActiveRun != nil })
}

// reportSiblingOwner starts a SECOND remote in the same project tree and gives
// it a run of its own: a sibling owner is one holding a run in this folder, so
// a second registration alone would not be one.
func reportSiblingOwner(state *State) error {
	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	if state.Tree == nil {
		return state.fail("%w", ErrNoProjectTree)
	}

	sibling, err := launchRemote(state, state.Tree.Dir)
	if err != nil {
		return err
	}

	state.Remotes[siblingLabel] = sibling

	siblingSession, err := awaitSession(state, sibling)
	if err != nil {
		return err
	}

	state.Sessions[siblingLabel] = siblingSession

	err = postDispatch(state, siblingSession, probeCommand, siblingRunLabel)
	if err != nil {
		return err
	}

	return awaitReport(state, session, "a same-project sibling owner",
		func(detail *sessionDetail) bool {
			return holdsOwner(detail, siblingSession.SessionID)
		})
}

// holdsOwner answers whether the detail names that session among the owners
// working the same folder.
func holdsOwner(detail *sessionDetail, ownerID string) bool {
	for _, owner := range detail.ActiveOwners {
		if owner.OwnerID == ownerID {
			return true
		}
	}

	return false
}

// reportPathDivergence points the host's engine configuration at a mirrored
// architectural spec: the scanner calls any configured path but the canonical
// one a divergence, so the state is established in the folder the session scans.
func reportPathDivergence(state *State) error {
	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	err = mirrorArchitecture(state)
	if err != nil {
		return err
	}

	return awaitReport(state, session, "an architecture path divergence",
		func(detail *sessionDetail) bool {
			return detail.Inventory != nil && detail.Inventory.ArchitecturePathMismatch
		})
}

// mirrorArchitecture copies the spec beside its canonical path and declares the
// copy, so the document stays readable while its configured path diverges.
func mirrorArchitecture(state *State) error {
	document, err := fixtureFile(state, canonicalArchitectureRel)
	if err != nil {
		return err
	}

	err = disk.Write(filepath.Join(state.Tree.Dir, mirroredArchitectureRel),
		[]byte(document), disk.Shared)
	if err != nil {
		return state.fail("writing %s: %w", mirroredArchitectureRel, err)
	}

	return declareArchitecturePath(state, mirroredArchitectureRel)
}

// declareArchitecturePath rewrites documents.architecture_yaml in the host's
// engine configuration. Round-tripped through the parser, never appended: a
// second `documents:` key is a duplicate the loader reads as an invalid config.
func declareArchitecturePath(state *State, relPath string) error {
	raw, err := fixtureFile(state, engineConfigRel)
	if err != nil {
		return err
	}

	config := map[string]any{}

	err = yaml.Unmarshal([]byte(raw), &config)
	if err != nil {
		return state.fail("parsing %s: %w", engineConfigRel, err)
	}

	documents, held := config["documents"].(map[string]any)
	if !held {
		documents = map[string]any{}
	}

	documents["architecture_yaml"] = relPath
	config["documents"] = documents

	encoded, err := yaml.Marshal(config)
	if err != nil {
		return state.fail("encoding %s: %w", engineConfigRel, err)
	}

	err = disk.Write(filepath.Join(state.Tree.Dir, engineConfigRel), encoded, disk.Shared)
	if err != nil {
		return state.fail("writing %s: %w", engineConfigRel, err)
	}

	return nil
}

// reportTruncatedInventory reconnects the workspace through a relay negotiating a
// budget the folder cannot fit: the remote takes that budget from the register
// reply, and the new relay's registry is namespaced to this scenario alone.
func reportTruncatedInventory(state *State) error {
	if state.Tree == nil {
		return state.fail("%w", ErrNoProjectTree)
	}

	started, err := startScenarioRelay(state,
		append(redisEnv(state), inventoryBudgetSetting+"="+truncatingInventoryBytes)...)
	if err != nil {
		return state.fail("starting a relay with %s set to %s: %w",
			inventoryBudgetSetting, truncatingInventoryBytes, err)
	}

	state.RelayURL = started.BaseURL
	state.Relay = started
	state.Session = nil

	err = attachRemote(state, state.Tree.Dir)
	if err != nil {
		return err
	}

	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	return awaitReport(state, session, "a truncated inventory",
		func(detail *sessionDetail) bool {
			return detail.Inventory != nil && detail.Inventory.SnapshotTruncated
		})
}

// awaitReport polls the session's detail until it reports the state, naming
// what it reported instead when it never does.
func awaitReport(state *State, session *sessionSummary, wanted string,
	reports func(*sessionDetail) bool,
) error {
	deadline := time.Now().Add(reportTimeout)

	var reason string

	for {
		detail, err := getSessionDetail(state, session)

		switch {
		case err != nil:
			reason = err.Error()
		case reports(detail):
			return nil
		default:
			reason = describeReport(detail)
		}

		if !time.Now().Before(deadline) {
			return state.fail("the session never reported %s within %s: %s",
				wanted, reportTimeout, reason)
		}

		time.Sleep(runPollInterval)
	}
}

// getSessionDetail reads the session's own detail view — the read the overview
// itself makes, so a Given is held to what the page will see.
func getSessionDetail(state *State, session *sessionSummary) (*sessionDetail, error) {
	path := sessionsPath + "/" + session.SessionID

	response, err := apiGet(state.RelayURL, path)
	if err != nil {
		return nil, state.fail("%w", err)
	}

	if response.Status != http.StatusOK {
		return nil, state.fail("GET %s returned %d, want 200: %s",
			path, response.Status, response.snippet())
	}

	var detail sessionDetail

	err = json.Unmarshal(response.Body, &detail)
	if err != nil {
		return nil, state.fail("decode GET %s: %w\n%s", path, err, response.snippet())
	}

	return &detail, nil
}

// describeReport renders what the session DID report, so a Given that gave up
// carries the alternative rather than only the absence.
func describeReport(detail *sessionDetail) string {
	run := "no active run"
	if detail.ActiveRun != nil {
		run = fmt.Sprintf("an active %q run", detail.ActiveRun.Command)
	}

	if detail.Inventory == nil {
		return run + ", " + strconv.Itoa(len(detail.ActiveOwners)) +
			" same-project owner(s), no inventory"
	}

	return fmt.Sprintf("%s, %d same-project owner(s), inventory truncated=%t "+
		"mismatch=%t unavailable=%q", run, len(detail.ActiveOwners),
		detail.Inventory.SnapshotTruncated,
		detail.Inventory.ArchitecturePathMismatch, detail.Inventory.Unavailable)
}

// assertInventoryRows holds the health list to carrying a row per entry the
// SESSION reports — files, directories and checklists alike — each with a chip
// on a status the scanner's vocabulary holds.
func assertInventoryRows(state *State, _ []string) error {
	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	detail, err := getSessionDetail(state, session)
	if err != nil {
		return err
	}

	if detail.Inventory == nil || len(detail.Inventory.Documents) == 0 {
		return state.fail("%w", ErrNoInventory)
	}

	for _, key := range sortedEntryKeys(detail.Inventory.Documents) {
		err = assertInventoryRow(state, key)
		if err != nil {
			return err
		}
	}

	return nil
}

// assertInventoryRow holds one reported entry to rendering a row whose chip
// carries a status a reader can be held to.
func assertInventoryRow(state *State, key string) error {
	text := fmt.Sprintf("%s[key=%s] > %s", inventoryRowTestID, key, inventoryChipTestID)

	sel, chip, err := locateStep(state, text)
	if err != nil {
		return err
	}

	got, matched, err := await(readAttribute(chip, inventoryStatusAttribute), knownStatus)
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s carries %s = %q, want one of %s",
			sel, inventoryStatusAttribute, got,
			strings.Join(knownInventoryStatuses(), ", "))
	}

	return nil
}

// knownStatus answers whether a chip's status is one the scanner reports.
func knownStatus(got string) bool {
	return slices.Contains(knownInventoryStatuses(), got)
}

// awaitSettledChips holds every chip the health list rendered to carrying a
// status the scanner reports, so a clause after it grades a settled list rather
// than one still resolving.
func awaitSettledChips(state *State, _ []string) error {
	probe := fmt.Sprintf(`els => els.map(el => {
		const status = el.getAttribute(%[1]q) || ""
		return %[2]s.includes(status) ? %[3]q :
			"a chip carries " + %[1]q + " = " + JSON.stringify(status)
	})`, inventoryStatusAttribute, knownStatusArray(), verdictOK)

	return assertEveryElement(state, elementCSS(inventoryChipTestID, "", ""), probe,
		"every inventory chip must have settled onto one of "+
			strings.Join(knownInventoryStatuses(), ", "))
}

// knownStatusArray is that vocabulary as the probe's own array literal.
func knownStatusArray() string {
	quoted := make([]string, 0, len(knownInventoryStatuses()))

	for _, status := range knownInventoryStatuses() {
		quoted = append(quoted, fmt.Sprintf("%q", status))
	}

	return "[" + strings.Join(quoted, ", ") + "]"
}

// sortedEntryKeys is the reported entries in a stable order, so a failure names
// the same first offender on every run.
func sortedEntryKeys(documents map[string]string) []string {
	keys := make([]string, 0, len(documents))
	for key := range documents {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
