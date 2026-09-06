package steps

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoBrowserWrite is returned when a clause is about a write the page made
// and the page made none.
var ErrNoBrowserWrite = errors.New("the browser made no document write")

// ErrNoWriteReceipt is returned when a write was answered without the receipt
// the CLI owes it.
var ErrNoWriteReceipt = errors.New("the write's answer carries no receipt")

// ErrNoWorkID is returned when a receipt names no work, which leaves the audit
// clause with nothing to correlate on.
var ErrNoWorkID = errors.New("the receipt names no work id")

// ErrNoSavedPath is returned when a clause says "that path" and no earlier
// clause named a document.
var ErrNoSavedPath = errors.New("no earlier clause named the document written")

// ErrUnmappedStatus is returned when the write's status is one the relay maps
// to no error name, so there is nothing for the clause to hold it to.
var ErrUnmappedStatus = errors.New("that status maps to no error name")

const (
	// writeTimeout caps waiting for the page to make its write and hear back: the
	// save is fired off the editor's own debounce and brokered through the CLI.
	writeTimeout = 30 * time.Second
	// inFlightPoll is how often the in-flight clause looks, tight because the
	// window it is aiming at is one round trip wide.
	inFlightPoll = 10 * time.Millisecond
	// auditTimeout caps waiting for the relay to record the CLI's receipt.
	auditTimeout = 20 * time.Second
	// auditRoutePattern is where the relay serves its record of the writes it
	// brokered for one session.
	auditRoutePattern = "/api/sessions/%s/audit"
	// saveStateAttribute is where the page renders how the save ended.
	saveStateAttribute = "data-save-state"
	// statusListPattern is one status, or statuses joined by "," and " or ".
	statusListPattern = `\d+(?:, \d+)*(?: or \d+)?`
)

// writeProbeScript wraps fetch so every document write the PAGE makes is kept
// with the answer it got. A write is recognised by the CLI's own doc_write
// payload (services/bdd-cli/internal/app/remote/docs.go), never by a route.
const writeProbeScript = `(() => {
  if (window.__tbddWrites) { return }
  window.__tbddWrites = []
  const inner = window.fetch
  = function (input, init) {
    let sent = null
    try { sent = JSON.parse((init && init.body) || "") } catch (parseError) { sent = null }
    const isWrite = sent && typeof sent.path === "string" && typeof sent.content === "string"
    if (!isWrite) { return inner.apply(this, arguments) }
    const record = { path: sent.path, status: null, body: null, error: "" }
    window.__tbddWrites.push(record)
    return inner.apply(this, arguments).then(function (response) {
      record.status = response.status
      return response.clone().text().then(function (text) {
        try { record.body = JSON.parse(text) } catch (parseError) { record.body = { raw: text }
        return response
      })
    }, function (failure) { record.error = String(failure); throw failure })
  }
})()`

// writeLogProbe hands the recorded writes back as JSON.
const writeLogProbe = `() => JSON.stringify(window.__tbddWrites || [])`

// browserWrite is one document write the page made, as the page saw it.
type browserWrite struct {
	Path   string          `json:"path"`
	Status *int            `json:"status"`
	Body   json.RawMessage `json:"body"`
	Error  string          `json:"error"`
}

// writeReceipt is the CLI's receipt for one write — the fields docs.go commits
// plus the work the relay brokered it under.
type writeReceipt struct {
	Path              string `json:"path"`
	CommittedRevision string `json:"committed_revision"`
	ContentHash       string `json:"content_hash"`
	WorkID            string `json:"work_id"`
}

// auditEntry is one record the relay keeps of a write it brokered.
type auditEntry struct {
	WorkID            string `json:"work_id"`
	Path              string `json:"path"`
	CommittedRevision string `json:"committed_revision"`
	ContentHash       string `json:"content_hash"`
}

// registerDocWriteSteps binds what the browser, the document and the relay each
// say about one save: the request, its receipt, and the audit beside it.
func registerDocWriteSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the browser sent no write$`, assertNoWriteSent)
	suite.Step(`^the write returns status (`+statusListPattern+`)$`, assertWriteStatus)
	suite.Step(`^the response names the error its status maps to$`, assertWriteErrorMapped)
	suite.Step(`^the browser's write receipt names that path and the file's hash$`,
		assertWriteReceipt)
	suite.Step(`^the relay's audit holds the CLI's receipt for the same work, `+
		`revision and hash$`, assertAuditHoldsReceipt)
	suite.Step(`^the remote is stopped once the write is in flight$`, stopRemoteMidWrite)
	suite.Step(`^a successful write carries a receipt naming the path, the file's hash `+
		`and a work id$`, assertSuccessfulWriteReceipt)
	suite.Step(`^a successful write's receipt is corroborated by the relay's audit `+
		`for the same work$`, assertSuccessfulWriteAudited)
	suite.Step(`^a failed write leaves (`+selectorPattern+`) short of "([^"]*)"$`,
		assertFailedWriteShortOf)
}

// observeWrites installs the write probe BEFORE the page navigates, which is the
// only moment at which a scenario's own writes can all be seen.
func observeWrites(state *State, page playwright.Page) error {
	err := page.AddInitScript(playwright.Script{
		Content: playwright.String(writeProbeScript),
	})
	if err != nil {
		return state.fail("installing the write probe: %w", err)
	}

	return nil
}

// snippet renders the answer a failure quotes.
func (write browserWrite) snippet() string {
	if len(write.Body) > bodySnippet {
		return string(write.Body[:bodySnippet]) + "…"
	}

	return string(write.Body)
}

// receipt is the CLI's receipt as the browser saw it. The relay may hand it back
// inside an envelope carrying the work id, so both places are read.
func (write browserWrite) receipt() (writeReceipt, error) {
	var envelope struct {
		Receipt writeReceipt `json:"receipt"`
		WorkID  string       `json:"work_id"`
	}

	err := json.Unmarshal(write.Body, &envelope)
	if err != nil {
		return writeReceipt{}, fmt.Errorf("%w: %s", ErrNoWriteReceipt, write.snippet())
	}

	receipt := envelope.Receipt
	if receipt.WorkID == "" {
		receipt.WorkID = envelope.WorkID
	}

	if receipt.Path == "" || receipt.ContentHash == "" {
		return writeReceipt{}, fmt.Errorf("%w: %s", ErrNoWriteReceipt, write.snippet())
	}

	return receipt, nil
}

// browserWrites reads every write the page has recorded so far.
func browserWrites(state *State) ([]browserWrite, error) {
	page, err := state.page()
	if err != nil {
		return nil, err
	}

	raw, err := probeString(page, writeLogProbe)
	if err != nil {
		return nil, state.fail("reading the browser's writes: %w", err)
	}

	var writes []browserWrite

	err = json.Unmarshal([]byte(raw), &writes)
	if err != nil {
		return nil, state.fail("decoding the browser's writes: %w\n%s", err, raw)
	}

	return writes, nil
}

// awaitAnsweredWrite waits for the page to have made a write AND heard back:
// reading once after a When is reading before the request.
func awaitAnsweredWrite(state *State) (browserWrite, error) {
	deadline := time.Now().Add(writeTimeout)

	for {
		writes, err := browserWrites(state)
		if err != nil {
			return browserWrite{}, err
		}

		if len(writes) > 0 {
			last := writes[len(writes)-1]
			if last.Status != nil || last.Error != "" {
				return last, nil
			}
		}

		if !time.Now().Before(deadline) {
			return browserWrite{}, state.fail("%w within %s", ErrNoBrowserWrite, writeTimeout)
		}

		time.Sleep(valuePollInterval)
	}
}

// assertNoWriteSent holds the page to never having sent one. Watched, because a
// buffer that is sent a beat late would pass a single reading.
func assertNoWriteSent(state *State, _ []string) error {
	deadline := time.Now().Add(attributeSettle)

	for {
		writes, err := browserWrites(state)
		if err != nil {
			return err
		}

		if len(writes) > 0 {
			return state.fail("the browser wrote %q, want a buffer that never left the page",
				writes[0].Path)
		}

		if !time.Now().Before(deadline) {
			return nil
		}

		time.Sleep(valuePollInterval)
	}
}

// assertWriteStatus holds the write's answer to a status the step accepts.
func assertWriteStatus(state *State, args []string) error {
	want, err := parseStatusSet(strings.ReplaceAll(args[0], ",", " or "))
	if err != nil {
		return state.fail("%w", err)
	}

	write, err := awaitAnsweredWrite(state)
	if err != nil {
		return err
	}

	if write.Status == nil {
		return state.fail("the browser's write of %q was never answered: %s",
			write.Path, write.Error)
	}

	if !want.holds(*write.Status) {
		return state.fail("the browser's write of %q returned %d, want %s: %s",
			write.Path, *write.Status, want, write.snippet())
	}

	return nil
}

// writeErrorNames is what a status maps to in the relay's error vocabulary. 404
// is pinned by the protocol scenarios; nothing pins the timeout's spelling, so
// both names the relay could use are accepted.
func writeErrorNames(status int) ([]string, bool) {
	switch status {
	case http.StatusNotFound:
		return []string{"session_gone"}, true
	case http.StatusGatewayTimeout:
		return []string{"cli_timeout", "timeout"}, true
	default:
		return nil, false
	}
}

// assertWriteErrorMapped holds the refusal to naming its reason, which is what
// turns a bare status into something a reader can act on.
func assertWriteErrorMapped(state *State, _ []string) error {
	write, err := awaitAnsweredWrite(state)
	if err != nil {
		return err
	}

	if write.Status == nil {
		return state.fail("the browser's write of %q was never answered: %s",
			write.Path, write.Error)
	}

	wanted, mapped := writeErrorNames(*write.Status)
	if !mapped {
		return state.fail("%w: %d", ErrUnmappedStatus, *write.Status)
	}

	var body struct {
		Error string `json:"error"`
	}

	err = json.Unmarshal(write.Body, &body)
	if err != nil {
		return state.fail("the write's answer is not JSON: %w\n%s", err, write.snippet())
	}

	for _, name := range wanted {
		if body.Error == name {
			return nil
		}
	}

	return state.fail("the write's %d names error %q, want %s: %s",
		*write.Status, body.Error, strings.Join(wanted, " or "), write.snippet())
}

// documentHash is the document's own SHA-256, hex — the revision the CLI derives
// from the bytes it committed (remote/docs.go revisionOf).
func documentHash(state *State, relPath string) (string, error) {
	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(sum[:]), nil
}

// assertWriteReceipt holds the receipt the browser was handed to naming the
// document the previous clause read, and that file's own hash.
func assertWriteReceipt(state *State, _ []string) error {
	if state.SavedPath == "" {
		return state.fail("%w", ErrNoSavedPath)
	}

	receipt, err := successfulReceipt(state)
	if err != nil {
		return err
	}

	if receipt == nil {
		return state.fail("the browser's write was not accepted, so it carries no receipt")
	}

	return checkReceiptNames(state, *receipt, state.SavedPath)
}

// successfulReceipt is the receipt of the last write when it succeeded, and nil
// when it did not — the shape the "a successful write …" clauses read.
func successfulReceipt(state *State) (*writeReceipt, error) {
	write, err := awaitAnsweredWrite(state)
	if err != nil {
		return nil, err
	}

	if write.Status == nil || *write.Status != http.StatusOK {
		return nil, nil //nolint:nilnil // no receipt is the answer, not an error
	}

	receipt, err := write.receipt()
	if err != nil {
		return nil, state.fail("%w", err)
	}

	return &receipt, nil
}

// checkReceiptNames holds one receipt to the document it claims and to that
// document's bytes as they now stand.
func checkReceiptNames(state *State, receipt writeReceipt, relPath string) error {
	if receipt.Path != relPath {
		return state.fail("the write receipt names path %q, want %q", receipt.Path, relPath)
	}

	hash, err := documentHash(state, relPath)
	if err != nil {
		return err
	}

	if receipt.ContentHash != hash {
		return state.fail("the write receipt names hash %q for %s, want %q — "+
			"the hash of the file on disk", receipt.ContentHash, relPath, hash)
	}

	return nil
}

// assertSuccessfulWriteReceipt is the conditional form: a write that failed is
// graded by its own clause, and this one holds only an accepted write.
func assertSuccessfulWriteReceipt(state *State, _ []string) error {
	receipt, err := successfulReceipt(state)
	if err != nil || receipt == nil {
		return err
	}

	err = checkReceiptNames(state, *receipt, receipt.Path)
	if err != nil {
		return err
	}

	if receipt.WorkID == "" {
		return state.fail("%w: the receipt for %s names path and hash only",
			ErrNoWorkID, receipt.Path)
	}

	return nil
}

// assertAuditHoldsReceipt holds the relay's own record to the receipt the CLI
// answered with: one write, one work, agreed on by both sides.
func assertAuditHoldsReceipt(state *State, _ []string) error {
	receipt, err := successfulReceipt(state)
	if err != nil {
		return err
	}

	if receipt == nil {
		return state.fail("the browser's write was not accepted, so no work was audited")
	}

	return checkAudited(state, *receipt)
}

// assertSuccessfulWriteAudited is that clause in its conditional form.
func assertSuccessfulWriteAudited(state *State, _ []string) error {
	receipt, err := successfulReceipt(state)
	if err != nil || receipt == nil {
		return err
	}

	return checkAudited(state, *receipt)
}

// checkAudited holds the relay's audit entry for the work to the receipt's own
// path, revision and hash.
func checkAudited(state *State, receipt writeReceipt) error {
	if receipt.WorkID == "" {
		return state.fail("%w: the receipt for %s", ErrNoWorkID, receipt.Path)
	}

	entry, err := awaitAuditEntry(state, receipt.WorkID)
	if err != nil {
		return err
	}

	if entry.Path != receipt.Path || entry.CommittedRevision != receipt.CommittedRevision ||
		entry.ContentHash != receipt.ContentHash {
		return state.fail("the relay's audit of work %q reads path %q revision %q hash %q, "+
			"want the CLI's receipt: path %q revision %q hash %q",
			receipt.WorkID, entry.Path, entry.CommittedRevision, entry.ContentHash,
			receipt.Path, receipt.CommittedRevision, receipt.ContentHash)
	}

	return nil
}

// awaitAuditEntry waits for the relay to hold a record of one work: it writes
// the audit as the CLI's reply lands, which is after the browser was answered.
func awaitAuditEntry(state *State, workID string) (auditEntry, error) {
	session, err := ensureSession(state)
	if err != nil {
		return auditEntry{}, err
	}

	route := fmt.Sprintf(auditRoutePattern, session.SessionID)
	deadline := time.Now().Add(auditTimeout)
	last := "the audit listed no entry for it"

	for {
		entries, reason := readAudit(state, route)
		if reason != "" {
			last = reason
		}

		for _, entry := range entries {
			if entry.WorkID == workID {
				return entry, nil
			}
		}

		if !time.Now().Before(deadline) {
			return auditEntry{}, state.fail("the relay's audit never held work %q within %s: %s",
				workID, auditTimeout, last)
		}

		time.Sleep(valuePollInterval)
	}
}

// readAudit reads the relay's write audit once, tolerating both a bare list and
// one under an `entries` key, and says why when it could not.
func readAudit(state *State, route string) ([]auditEntry, string) {
	response, err := apiGet(state.RelayURL, route)
	if err != nil {
		return nil, err.Error()
	}

	if response.Status != http.StatusOK {
		return nil, fmt.Sprintf("GET %s returned %d: %s",
			route, response.Status, response.snippet())
	}

	var wrapped struct {
		Entries []auditEntry `json:"entries"`
	}

	err = json.Unmarshal(response.Body, &wrapped)
	if err == nil && wrapped.Entries != nil {
		return wrapped.Entries, ""
	}

	var bare []auditEntry

	err = json.Unmarshal(response.Body, &bare)
	if err != nil {
		return nil, fmt.Sprintf("decoding %s: %s: %s", route, err, response.snippet())
	}

	return bare, ""
}

// stopRemoteMidWrite kills the CLI between the page issuing its write and the
// relay answering it — the only window in which the outcome is genuinely either.
func stopRemoteMidWrite(state *State, _ []string) error {
	deadline := time.Now().Add(writeTimeout)

	for {
		writes, err := browserWrites(state)
		if err != nil {
			return err
		}

		if inFlight(writes) {
			return stopRemote(state, nil)
		}

		if !time.Now().Before(deadline) {
			return state.fail("%w within %s, so no write was in flight to interrupt",
				ErrNoBrowserWrite, writeTimeout)
		}

		time.Sleep(inFlightPoll)
	}
}

// inFlight answers whether the page is waiting on a write it has already sent.
func inFlight(writes []browserWrite) bool {
	for _, write := range writes {
		if write.Status == nil && write.Error == "" {
			return true
		}
	}

	return false
}

// assertFailedWriteShortOf holds a REFUSED write to never reporting the state a
// committed one reports; an accepted write is graded by the receipt clauses.
func assertFailedWriteShortOf(state *State, args []string) error {
	write, err := awaitAnsweredWrite(state)
	if err != nil {
		return err
	}

	if write.Status != nil && *write.Status == http.StatusOK {
		return nil
	}

	return refuteAttribute(state, []string{args[0], saveStateAttribute, args[1]})
}
