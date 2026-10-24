package drill_test

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/drill"
)

const messageKey = "msg"

func TestAuditReadsCodexCommandsAndFileChangesWithoutInferringReads(t *testing.T) {
	runner, record, cut := prepare(t, "#!/bin/sh\nexit 0\n")
	path := filepath.Join(cut.Directory, "tmp/turn/codex-turn002-codex-events.jsonl")
	put(t, path, `{"type":"item.completed","item":{"type":"command_execution",
"command":"cat /outside/marker; go test ./tests/suite","exit_code":1,"status":"completed"}}
{"type":"item.completed","item":{"type":"file_change",
"changes":[{"path":"services/demo/main.go","kind":"update"}],"status":"completed"}}
{"type":"turn.completed","usage":{"input_tokens":25000}}
invalid
`)
	audit, err := runner.Audit(t.Context(), record.Directory, "a")
	must(t, err)

	found := map[string]bool{}

	for _, entry := range audit.Entries {
		if entry.Evidence == path {
			found[entry.Tool+":"+entry.Target] = true
			if entry.Decision != "unknown" {
				t.Fatalf("inferred read success: %+v", entry)
			}
		}
	}

	for _, want := range []string{
		"codex:command_execution:/outside/marker",
		"codex:file_change:" + filepath.Join(cut.Directory, "services/demo/main.go"),
	} {
		if !found[want] {
			t.Errorf("missing %s: %+v", want, audit)
		}
	}

	if !strings.Contains(strings.Join(audit.Unknown, "\n"), "invalid codex event") {
		t.Fatal("invalid evidence hidden")
	}
}

func TestAuditExplicitlyExcludesEveryOracleReplayForTheCut(t *testing.T) {
	runner, record, _ := prepare(t, "#!/bin/sh\nexit 0\n")
	record, err := drill.Read(record.Directory)
	must(t, err)

	record.Runs = map[string]drill.Attempt{
		"oracle-a-1": {Cut: "a", Oracle: &drill.Oracle{Directory: "/oracle/first"}},
		"oracle-a-2": {Cut: "a", Oracle: &drill.Oracle{Directory: "/oracle/second"}},
		"oracle-b-1": {Cut: "b", Oracle: &drill.Oracle{Directory: "/oracle/other-cut"}},
	}
	data, err := json.Marshal(record)
	must(t, err)
	put(t, filepath.Join(record.Directory, "record.json"), string(data))

	var output strings.Builder

	prior := slog.Default()

	slog.SetDefault(slog.New(logging.Handler(&output, "")))
	t.Cleanup(func() { slog.SetDefault(prior) })

	audit, err := runner.Audit(t.Context(), record.Directory, "a")
	must(t, err)
	saved, err := drill.Read(record.Directory)
	must(t, err)

	for label, report := range map[string]string{
		"result": strings.Join(audit.Unknown, "\n"),
		"record": strings.Join(saved.Audits["a"][0].Unknown, "\n"),
		"output": output.String(),
	} {
		for _, want := range []string{"oracle-a-1", "oracle-a-2", "/oracle/first", "/oracle/second",
			"not audited", "-mode=services:replay,tests:replay", "no live AI turn",
			"recording-time paths"} {
			if !strings.Contains(report, want) {
				t.Errorf("%s missing %q: %s", label, want, report)
			}
		}

		if strings.Contains(report, "oracle-b-1") {
			t.Errorf("%s includes another cut's oracle: %s", label, report)
		}
	}
}

func TestAuditReportsPathsDenialsToolchainAndNetworkFromSQLite(t *testing.T) {
	runner, record, cut := prepare(t, "#!/bin/sh\nexit 0\n")
	rows := []map[string]any{
		{messageKey: enginelog.MsgToolUse,
			enginelog.KeyContent: `{"name":"Read","input":{"file_path":"/outside/secret.go"}}`,
			"allowed":            false},

		{messageKey: enginelog.MsgToolUse,
			enginelog.KeyContent: `{"name":"Read","input":{"file_path":"/toolchain/src/runtime.go"}}`},

		{messageKey: enginelog.MsgToolUse,
			enginelog.KeyContent: `{"name":"Glob","input":{"path":"docs/product"}}`,
			"allowed":            true},
	}

	var log strings.Builder

	for _, row := range rows {
		data, err := json.Marshal(row)
		must(t, err)

		log.Write(data)
		log.WriteByte('\n')
	}

	put(t, filepath.Join(cut.Directory, "tmp/true-bdd.log.json"), log.String())
	database := filepath.Join(cut.Directory, "tmp/turn/crush-data/crush.db")
	put(t, database, "fake pages: deliberately not scanned")
	put(t, database+"-wal", "fake WAL")
	bin := t.TempDir()
	rowsPath := filepath.Join(bin, "rows.json")
	parts := `[
{"name":"view","input":{"file_path":"/outside/from-db.go"}},
{"name":"fetch","input":{"url":"https://example.com/true-bdd"}},
{"name":"sourcegraph","input":{"query":"repo:true-bdd"}},
{"name":"mcp_context7_query","input":{"library":"go"}}]`

	databaseRows, err := json.Marshal([]map[string]string{{"parts": parts}})
	must(t, err)

	put(t, rowsPath, string(databaseRows))
	t.Setenv("DRILL_SQLITE_ROWS", rowsPath)
	put(t, filepath.Join(bin, "sqlite3"), `#!/bin/sh
case "$4" in
 *sqlite_schema*) printf '%s' '[{"name":"messages"}]';;
 *) cat "$DRILL_SQLITE_ROWS";;
esac
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	audit, err := runner.Audit(t.Context(), record.Directory, "a")
	must(t, err)

	found := map[string]bool{}
	for _, entry := range audit.Entries {
		found[entry.Target+":"+entry.Group+":"+entry.Decision] = true
		found[entry.Tool] = true
	}

	for _, key := range []string{"/outside/secret.go:outside:denied",
		"/toolchain/src/runtime.go:allowed_root:unknown",
		filepath.Join(cut.Directory,
			"docs/product") + ":inside:allowed",
		"/outside/from-db.go:outside:unknown",
		"fetch",
		"sourcegraph",
		"mcp_context7_query"} {
		if !found[key] {
			t.Errorf("missing %s in %+v", key, audit)
		}
	}

	if len(audit.Unknown) == 0 {
		t.Fatal("audit claims certainty")
	}
}

func TestAuditResolvesRelativeGlobAgainstCut(t *testing.T) {
	runner, record, cut := prepare(t, "#!/bin/sh\nexit 0\n")
	put(t, filepath.Join(cut.Directory, "tmp/true-bdd.log.json"),
		`{"msg":"ToolUseBlock details","content":"{\"name\":\"Glob\",\"input\":{\"pattern\":\"**/*.go\"}}"}`+"\n")
	audit, err := runner.Audit(t.Context(), record.Directory, "a")
	must(t, err)

	for _, entry := range audit.Entries {
		if entry.Tool == "Glob" {
			if entry.Target != filepath.Join(cut.Directory, "**/*.go") || entry.Group != "inside" {
				t.Fatalf("relative glob misreported: %+v", entry)
			}

			return
		}
	}

	t.Fatalf("Glob call missing: %+v", audit)
}
