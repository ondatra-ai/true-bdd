package architecture_test

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

const sharedSource = "shared/source.go"

func TestServiceCodePredicateUsesLanguageAndWritablePaths(t *testing.T) {
	for _, example := range []struct {
		name, language, file string
		writable             []string
		built                bool
	}{
		{name: "absent", language: "go"},
		{name: "empty", language: "go"},
		{name: "manifest only", language: "go", file: "services/calc/go.mod"},
		{name: "lock only", language: "typescript", file: "services/calc/package-lock.json"},
		{name: "partly written", language: "go", file: "services/calc/nested/main.go", built: true},
		{name: "other language", language: "go", file: "services/calc/main.js"},
		{name: "typescript jsx", language: "typescript", file: "services/calc/main.tsx", built: true},
		{name: "javascript module", language: "javascript", file: "services/calc/main.mjs", built: true},
		{name: "writable file", language: "go", file: sharedSource, writable: []string{sharedSource}, built: true},
		{name: "writable directory", language: "go", file: sharedSource, writable: []string{"shared"}, built: true},
		{name: "unknown empty", language: "rust"},
		{name: "unknown file", language: "rust", file: "services/calc/source", built: true},
	} {
		t.Run(example.name, func(t *testing.T) {
			root := t.TempDir()
			if example.name == "empty" {
				err := disk.Dir(filepath.Join(root, "services/calc"), disk.Private)
				if err != nil {
					t.Fatal(err)
				}
			}

			if example.file != "" {
				err := disk.Write(filepath.Join(root, example.file), []byte("incomplete source"), disk.Private)
				if err != nil {
					t.Fatal(err)
				}
			}

			service := architecture.Service{Path: "services/calc", Language: example.language, Writable: example.writable}

			built, err := service.HasCode(root)
			if err != nil || built != example.built {
				t.Fatalf("built=%v, want %v, error=%v", built, example.built, err)
			}
		})
	}
}

func TestServiceCodePredicateIsPerService(t *testing.T) {
	root := t.TempDir()

	err := disk.Write(filepath.Join(root, "services/built/main.go"), []byte("package main"), disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	for _, service := range []architecture.Service{
		{Name: "absent", Path: "services/absent", Language: "go"},
		{Name: "built", Path: "services/built", Language: "go"},
	} {
		built, err := service.HasCode(root)
		if err != nil || built != (service.Name == "built") {
			t.Fatalf("%s built=%v error=%v", service.Name, built, err)
		}
	}
}
