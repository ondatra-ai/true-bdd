package drill

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func (a *Audit) codexEvents(tree string, roots []string) {
	err := filepath.WalkDir(filepath.Join(tree, "tmp"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "-codex-events.jsonl") {
			a.codexStream(path, tree, roots)
		}

		return nil
	})
	if err != nil {
		a.Unknown = append(a.Unknown, fmt.Sprintf("walking codex evidence: %s", err))
	}
}

func (a *Audit) codexStream(path, tree string, roots []string) {
	data, err := disk.Read(path)
	if err != nil {
		a.Unknown = append(a.Unknown, err.Error())

		return
	}

	a.Files = append(a.Files, path)
	decoder := json.NewDecoder(strings.NewReader(string(data)))

	for {
		var event struct {
			Type string         `json:"type"`
			Item map[string]any `json:"item"`
		}

		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			return
		}

		if err != nil {
			a.Unknown = append(a.Unknown, fmt.Sprintf("%s: invalid codex event: %s", path, err))

			return
		}

		if event.Type == "item.started" || event.Type == "item.completed" {
			a.codexItem(event.Item, path, tree, roots)
		}
	}
}

func (a *Audit) codexItem(item map[string]any, path, tree string, roots []string) {
	kind, _ := item["type"].(string)

	raw, err := json.Marshal(item)
	if err != nil {
		a.Unknown = append(a.Unknown, err.Error())

		return
	}

	switch kind {
	case "command_execution":
		command, _ := item["command"].(string)

		targets := pathReferences(command)
		if len(targets) == 0 {
			targets = []string{""}
		}

		for _, target := range targets {
			a.addAccess("codex:"+kind, target, string(raw), path, tree, roots, unknownValue)
		}
	case "file_change":
		changes, _ := item["changes"].([]any)
		for _, change := range changes {
			file, ok := change.(map[string]any)
			if !ok {
				continue
			}

			target, _ := file["path"].(string)
			a.addAccess("codex:"+kind, target, string(raw), path, tree, roots, unknownValue)
		}
	}
}
