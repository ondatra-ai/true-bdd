package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

var errPlaceholderUnresolved = errors.New("recording references a placeholder with no matching directory")

func restoreText(text, cwd string, values map[string]string) (string, error) {
	text = strings.ReplaceAll(text, goRootPlaceholder, goRoot())
	text = strings.ReplaceAll(text, cwdPlaceholder, cwd)

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		text = strings.ReplaceAll(text, homePlaceholder, home)
	}

	for token, value := range values {
		if !strings.Contains(text, token) {
			continue
		}

		if value == "" {
			return "", fmt.Errorf("%w: %s", errPlaceholderUnresolved, token)
		}

		text = strings.ReplaceAll(text, token, value)
	}

	return text, nil
}
