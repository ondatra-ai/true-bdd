package lint

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/yamale"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

const (
	configPath   = "true-bdd/true-bdd.yaml"
	schemaGlob   = "true-bdd/*-schema.yaml"
	schemaSuffix = "-schema.yaml"
)

// Schemas validates each document that has a schema against it, with yamale.
// The pairing is convention driven by config, so a new schema needs no edit
// here: `true-bdd/<key>-schema.yaml` validates `documents.<key>`.
func Schemas(out io.Writer, files []string) error {
	err := yamale.Available()
	if err != nil {
		return err
	}

	schemas, err := filepath.Glob(schemaGlob)
	if err != nil {
		return fmt.Errorf("globbing %s: %w", schemaGlob, err)
	}

	if len(schemas) == 0 {
		_, _ = fmt.Fprintln(out, "No schemas under true-bdd/ — nothing to validate.")

		return nil
	}

	documents, err := declaredDocuments()
	if err != nil {
		return err
	}

	failed := false

	for _, schema := range schemas {
		if !validateSchema(out, schema, documents, files) {
			failed = true
		}
	}

	if failed {
		return ErrFailed
	}

	return nil
}

// validateSchema answers for one schema, and reports whether it passed.
func validateSchema(out io.Writer, schema string, documents map[string]string, files []string) bool {
	key := strings.TrimSuffix(filepath.Base(schema), schemaSuffix)
	doc := documents[key]

	// A scoped run answers only for the files it was given, so it also skips
	// the unmapped-schema failure below — that is a repository invariant, and
	// the whole-repo run at commit time is where it belongs.
	if len(files) > 0 {
		if doc == "" || !named(files, doc) {
			return true
		}
	}

	if doc == "" {
		_, _ = fmt.Fprintf(out,
			"FAIL %s — no documents.%s in %s, so this schema enforces nothing.\n",
			schema, key, configPath)

		return false
	}

	_, err := os.Stat(doc)
	if err != nil {
		// A host may legitimately not carry every document yet; the schema
		// only binds a document that exists.
		_, _ = fmt.Fprintf(out, "SKIP %s (documents.%s) — not present in this repo.\n", doc, key)

		return true
	}

	_, _ = fmt.Fprintf(out, "Validating %s against %s\n", doc, schema)

	result, err := yamale.Validate(out, schema, doc)

	return err == nil && result.Code == 0
}

func named(files []string, doc string) bool {
	for _, file := range files {
		if strings.TrimPrefix(file, "./") == strings.TrimPrefix(doc, "./") {
			return true
		}
	}

	return false
}

// declaredDocuments reads the `documents:` mapping the schemas pair against.
// Absorbed from scripts/cmd/yamlkey, which existed only to answer this.
func declaredDocuments() (map[string]string, error) {
	raw, err := disk.Read(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", configPath, err)
	}

	var config struct {
		Documents map[string]string `yaml:"documents"`
	}

	err = yaml.Unmarshal(raw, &config)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", configPath, err)
	}

	return config.Documents, nil
}
