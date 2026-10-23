// Package sqlite reads reconstructed SQLite values, including overflow pages and WAL.
package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Text returns every text value in every ordinary table through SQLite's reader.
func Text(ctx context.Context, path string) ([]string, error) {
	tables,
		err := query(ctx,
		path,
		"SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}

	var text []string

	for _, table := range tables {
		name, _ := table["name"].(string)

		rows, err := query(ctx, path, `SELECT * FROM "`+strings.ReplaceAll(name, `"`, `""`)+`"`)
		if err != nil {
			return nil, err
		}

		for _, row := range rows {
			for _, value := range row {
				if value, ok := value.(string); ok {
					text = append(text, value)
				}
			}
		}
	}

	return text, nil
}

func query(ctx context.Context, path, statement string) ([]map[string]any, error) {
	const limit = time.Minute

	result,
		err := shell.Run(ctx,
		[]string{"sqlite3",
			"-readonly",
			"-json",
			path,
			statement},
		shell.Options{Group: true,
			Timeout: limit})
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("reading SQLite %s: %w", path, result.Err())
	}

	if strings.TrimSpace(result.Stdout) == "" {
		return nil, nil
	}

	var rows []map[string]any

	err = json.Unmarshal([]byte(result.Stdout), &rows)
	if err != nil {
		return nil, fmt.Errorf("decoding SQLite rows: %w", err)
	}

	return rows, nil
}
