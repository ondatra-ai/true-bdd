package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

type recording struct {
	Placeholders []placeholder     `json:"placeholders,omitempty"   yaml:"placeholders,omitempty"`
	Unrecorded   []string          `json:"unrecorded,omitempty"     yaml:"unrecorded,omitempty"`
	Unread       []string          `json:"unread,omitempty"         yaml:"unread,omitempty"`
	ReadOnlyWhen map[string]string `json:"read_only_when,omitempty" yaml:"read_only_when,omitempty"`
}

type placeholder struct {
	Token     string `json:"token"     yaml:"token"`
	Pattern   string `json:"pattern"   yaml:"pattern"`
	Directory string `json:"directory" yaml:"directory"`
	compiled  *regexp.Regexp
}

func loadRecording(directory, name string) (recording, error) {
	data, err := disk.Read(filepath.Join(directory, "recording.json"))
	if err != nil {
		return recording{}, fmt.Errorf("read recording rules: %w", err)
	}

	var rules map[string]recording

	err = json.Unmarshal(data, &rules)
	if err != nil {
		return recording{}, fmt.Errorf("decode recording rules: %w", err)
	}

	selected := rules[name]

	return selected, selected.prepare()
}

func (r recording) prepare() error {
	for index := range r.Placeholders {
		value := &r.Placeholders[index]

		pattern, err := regexp.Compile(value.Pattern)
		if err != nil {
			return fmt.Errorf("placeholder %s: %w", value.Token, err)
		}

		value.compiled = pattern
	}

	return nil
}

func (r recording) normalize(text string) string {
	for _, value := range r.Placeholders {
		text = value.compiled.ReplaceAllString(text, value.Token)
	}

	return text
}

func resolvePlaceholders(cwd string, rules recording) map[string]string {
	values := make(map[string]string, len(rules.Placeholders))
	for _, placeholder := range rules.Placeholders {
		values[placeholder.Token] = newestDirectory(filepath.Join(cwd, placeholder.Directory), placeholder.compiled)
	}

	return values
}

func newestDirectory(directory string, pattern *regexp.Regexp) string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return ""
	}

	newest := ""

	var newestAt time.Time

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		match := pattern.FindString(entry.Name())
		if match != entry.Name() {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr == nil && (newest == "" || info.ModTime().After(newestAt)) {
			newest, newestAt = entry.Name(), info.ModTime()
		}
	}

	return newest
}

func (r recording) omits(relative string) bool {
	for _, pattern := range append(r.Unrecorded, r.Unread...) {
		if matchPath(strings.Split(filepath.ToSlash(pattern), "/"), strings.Split(filepath.ToSlash(relative), "/")) {
			return true
		}
	}

	return false
}

func matchPath(pattern, relative []string) bool {
	if len(pattern) == 0 {
		return len(relative) == 0
	}

	if pattern[0] == "**" {
		if len(pattern) == 1 {
			return true
		}

		return matchPath(pattern[1:], relative) || (len(relative) > 0 && matchPath(pattern, relative[1:]))
	}

	if len(relative) == 0 {
		return false
	}

	matched, _ := path.Match(pattern[0], relative[0])

	return matched && matchPath(pattern[1:], relative[1:])
}

func (r recording) readOnly() bool {
	if len(r.ReadOnlyWhen) == 0 {
		return false
	}

	for key, value := range r.ReadOnlyWhen {
		if os.Getenv(key) != value {
			return false
		}
	}

	return true
}
