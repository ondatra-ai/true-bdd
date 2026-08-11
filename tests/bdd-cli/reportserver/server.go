package reportserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
)

// Sentinel errors for the handlers.
var (
	errUnknownRun  = errors.New("unknown run")
	errUnknownTest = errors.New("unknown test")
	errUnknownRef  = errors.New("unknown artifact")
	errNeedTwoRuns = errors.New("comparison needs two distinct runs")
)

// Server answers report queries from a Store.
type Server struct {
	store *Store
}

// NewServer wires a server over a store.
func NewServer(store *Store) *Server {
	return &Server{store: store}
}

// Handler is the routing table.
//
// Go's own ServeMux does method and wildcard patterns, so there is no
// router dependency here.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("GET /api/runs", s.handleRuns)
	mux.HandleFunc("GET /api/matrix", s.handleMatrix)
	mux.HandleFunc("GET /api/runs/{run}", s.handleRun)
	mux.HandleFunc("GET /api/runs/{run}/tests/{test}", s.handleTest)
	mux.HandleFunc("GET /api/runs/{run}/tests/{test}/text", s.handleText)
	mux.HandleFunc("GET /api/compare/{left}/{right}", s.handleCompareRuns)
	mux.HandleFunc("GET /api/compare/{left}/{right}/tests/{test}", s.handleCompareTest)
	mux.HandleFunc("GET /api/diff/text", s.handleDiffText)

	mux.Handle("/", assetHandler())

	return mux
}

// handleState is the poll target.
func (s *Server) handleState(writer http.ResponseWriter, request *http.Request) {
	snapshot, err := s.store.Snapshot()
	if err != nil {
		fail(writer, request, http.StatusInternalServerError, err)

		return
	}

	writeJSON(writer, request, StateResponse{
		Version:   snapshot.Version,
		ScannedAt: stamp(snapshot.ScannedAt),
		RunsDir:   s.store.RunsDir(),
		Runs:      runSummaries(snapshot),
	})
}

// handleRuns lists every run.
func (s *Server) handleRuns(writer http.ResponseWriter, request *http.Request) {
	snapshot, err := s.store.Snapshot()
	if err != nil {
		fail(writer, request, http.StatusInternalServerError, err)

		return
	}

	writeJSON(writer, request, RunListResponse{Version: snapshot.Version, Runs: runSummaries(snapshot)})
}

// handleMatrix serves every test against every run on one grid.
func (s *Server) handleMatrix(writer http.ResponseWriter, request *http.Request) {
	snapshot, err := s.store.Snapshot()
	if err != nil {
		fail(writer, request, http.StatusInternalServerError, err)

		return
	}

	writeJSON(writer, request, BuildMatrix(snapshot))
}

// handleRun serves one run and its neighbours.
func (s *Server) handleRun(writer http.ResponseWriter, request *http.Request) {
	snapshot, run, found := s.lookupRun(writer, request, request.PathValue("run"))
	if !found {
		return
	}

	prev, next := snapshot.Neighbours(run.ID)

	tests := make([]TestSummary, 0, len(run.Tests))
	for _, fixture := range run.Tests {
		tests = append(tests, mapTestSummary(fixture))
	}

	writeJSON(writer, request, RunResponse{
		Version: snapshot.Version,
		Run:     mapRunSummary(run),
		Prev:    prev,
		Next:    next,
		Tests:   tests,
	})
}

// handleTest serves one fixture's detail.
func (s *Server) handleTest(writer http.ResponseWriter, request *http.Request) {
	snapshot, run, found := s.lookupRun(writer, request, request.PathValue("run"))
	if !found {
		return
	}

	fixture, present := run.Test(request.PathValue("test"))
	if !present {
		fail(writer, request, http.StatusNotFound, errUnknownTest)

		return
	}

	turns := make([]TurnDTO, 0, len(fixture.Turns))
	for position, turn := range fixture.Turns {
		turns = append(turns, mapTurn(position, turn))
	}

	writeJSON(writer, request, TestDetail{
		Version:  snapshot.Version,
		Run:      run.ID,
		Summary:  mapTestSummary(fixture),
		Expected: mapExpected(fixture.Manifest),
		Actual:   mapActual(fixture),
		Judge:    mapJudge(fixture),
		Discovery: DiscoveryDTO{
			Found:     fixture.Discovery.Found,
			Framework: fixture.Discovery.Framework,
			Outcome:   fixture.Discovery.Outcome,
		},
		Phases:    mapPhases(fixture),
		Turns:     turns,
		Files:     mapFiles(fixture),
		TestRuns:  mapTestRuns(fixture.TestRuns),
		Artifacts: fixtureRefs(fixture),
		Meta:      mapMeta(fixture.Meta),
		Warnings:  orEmpty(fixture.EmptyFailurePrompts),
	})
}

// handleText serves one artifact body as plain text.
func (s *Server) handleText(writer http.ResponseWriter, request *http.Request) {
	_, run, found := s.lookupRun(writer, request, request.PathValue("run"))
	if !found {
		return
	}

	fixture, present := run.Test(request.PathValue("test"))
	if !present {
		fail(writer, request, http.StatusNotFound, errUnknownTest)

		return
	}

	body, found := resolveText(fixture, request.URL.Query().Get("ref"))
	if !found {
		fail(writer, request, http.StatusNotFound, errUnknownRef)

		return
	}

	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = writer.Write([]byte(body))
}

// handleCompareRuns aligns two runs test by test.
func (s *Server) handleCompareRuns(writer http.ResponseWriter, request *http.Request) {
	snapshot, left, right, found := s.lookupPair(writer, request)
	if !found {
		return
	}

	writeJSON(writer, request, CompareRuns(snapshot.Version, left, right))
}

// handleCompareTest aligns one fixture across two runs.
func (s *Server) handleCompareTest(writer http.ResponseWriter, request *http.Request) {
	snapshot, left, right, found := s.lookupPair(writer, request)
	if !found {
		return
	}

	name := request.PathValue("test")

	leftFixture, leftOK := left.Test(name)
	rightFixture, rightOK := right.Test(name)

	if !leftOK || !rightOK {
		fail(writer, request, http.StatusNotFound, errUnknownTest)

		return
	}

	writeJSON(writer, request, CompareTests(snapshot.Version, left.ID, right.ID, leftFixture, rightFixture))
}

// handleDiffText line-diffs one artifact pair across two runs.
func (s *Server) handleDiffText(writer http.ResponseWriter, request *http.Request) {
	snapshot, err := s.store.Snapshot()
	if err != nil {
		fail(writer, request, http.StatusInternalServerError, err)

		return
	}

	query := request.URL.Query()

	left, leftOK := bodyAt(snapshot, query.Get("lrun"), query.Get("ltest"), query.Get("lref"))
	right, rightOK := bodyAt(snapshot, query.Get("rrun"), query.Get("rtest"), query.Get("rref"))

	if !leftOK && !rightOK {
		fail(writer, request, http.StatusNotFound, errUnknownRef)

		return
	}

	context := defaultDiffContext

	raw := query.Get("context")
	if raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr == nil {
			context = parsed
		}
	}

	writeJSON(writer, request, DiffText(left, right, context))
}

// bodyAt resolves one side of a text diff, tolerating an absent side so
// "this run has no judge transcript" diffs as a whole-file insertion
// rather than 404ing the request.
func bodyAt(snapshot *Snapshot, runID, test, ref string) (string, bool) {
	run, found := snapshot.Run(runID)
	if !found {
		return "", false
	}

	fixture, found := run.Test(test)
	if !found {
		return "", false
	}

	return resolveText(fixture, ref)
}

// lookupRun resolves a run id, replying 404 when it is unknown.
func (s *Server) lookupRun(
	writer http.ResponseWriter, request *http.Request, runID string,
) (*Snapshot, *Run, bool) {
	snapshot, err := s.store.Snapshot()
	if err != nil {
		fail(writer, request, http.StatusInternalServerError, err)

		return nil, nil, false
	}

	run, found := snapshot.Run(runID)
	if !found {
		fail(writer, request, http.StatusNotFound, errUnknownRun)

		return nil, nil, false
	}

	return snapshot, run, true
}

// lookupPair resolves the two runs of a comparison.
func (s *Server) lookupPair(
	writer http.ResponseWriter, request *http.Request,
) (*Snapshot, *Run, *Run, bool) {
	leftID, rightID := request.PathValue("left"), request.PathValue("right")
	if leftID == rightID {
		fail(writer, request, http.StatusBadRequest, errNeedTwoRuns)

		return nil, nil, nil, false
	}

	snapshot, left, found := s.lookupRun(writer, request, leftID)
	if !found {
		return nil, nil, nil, false
	}

	right, found := snapshot.Run(rightID)
	if !found {
		fail(writer, request, http.StatusNotFound, errUnknownRun)

		return nil, nil, nil, false
	}

	return snapshot, left, right, true
}

// runSummaries projects the snapshot's runs newest first — a reader
// almost always wants the most recent run, and the store keeps them
// oldest first so prev/next reads chronologically.
func runSummaries(snapshot *Snapshot) []RunSummary {
	summaries := make([]RunSummary, 0, len(snapshot.Runs))

	for index := len(snapshot.Runs) - 1; index >= 0; index-- {
		summaries = append(summaries, mapRunSummary(snapshot.Runs[index]))
	}

	return summaries
}

// writeJSON encodes a payload.
func writeJSON(writer http.ResponseWriter, request *http.Request, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")

	err := json.NewEncoder(writer).Encode(payload)
	if err != nil {
		slog.ErrorContext(request.Context(), "encode response", "path", request.URL.Path, "error", err)
	}
}

// fail replies with a JSON error.
func fail(writer http.ResponseWriter, request *http.Request, status int, err error) {
	slog.WarnContext(request.Context(), "request failed",
		"path", request.URL.Path, "status", status, "error", err)

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)

	encodeErr := json.NewEncoder(writer).Encode(map[string]string{"error": err.Error()})
	if encodeErr != nil {
		slog.ErrorContext(request.Context(), "encode error reply", "error", encodeErr)
	}
}
