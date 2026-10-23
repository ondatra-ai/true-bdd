package drill_test

import (
	"encoding/json"
	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const messageKey = "msg"

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
