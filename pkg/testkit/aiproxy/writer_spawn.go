package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

const writerFixtureEnv = "TRUE_BDD_AIPROXY_WRITER_FIXTURE"
const privateWriterFile = 0o600
const privateWriterDirectory = 0o700
const initialWriterHomeFiles = 2

const dummyWriterAuth = "{\"true_bdd_dummy_auth\":\"issue-210-no-credentials\"}\n"

var errWriterSpawn = errors.New("codex writer spawn contract violated")

// verifyWriterSpawn opens authentication only for the opt-in dummy fixture (issue #210).
func verifyWriterSpawn(cfg config, name string) error {
	fixture := os.Getenv(writerFixtureEnv)
	if name != "codex" || fixture == "" {
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("writer workspace: %w", err)
	}

	home, err := filepath.EvalSymlinks(os.Getenv("CODEX_HOME"))
	if err != nil {
		return fmt.Errorf("%w: generated home: %w", errWriterSpawn, err)
	}

	expected := filepath.Join(cwd, "tmp", "codex-homes") + string(filepath.Separator)
	if !strings.HasPrefix(home, expected) || !strings.HasSuffix(home, "-codex-home") {
		return fmt.Errorf("%w: CODEX_HOME must be beneath host tmp/codex-homes", errWriterSpawn)
	}

	err = verifyDummyAuthentication(fixture, home)
	if err != nil {
		return err
	}

	err = verifyWriterRoots(cwd, home)
	if err != nil {
		return err
	}

	return writeWriterReceipt(cfg.StateDir)
}

func writeWriterReceipt(stateDir string) error {
	receipt := filepath.Join(stateDir, "writer-spawn.txt")

	_, err := os.Lstat(receipt)
	if !os.IsNotExist(err) {
		return fmt.Errorf("%w: expected exactly one writer spawn", errWriterSpawn)
	}

	err = disk.Write(receipt, []byte("fixture authentication and edit roots verified\n"), disk.Private)
	if err != nil {
		return fmt.Errorf("write writer spawn receipt: %w", err)
	}

	return nil
}

func verifyDummyAuthentication(fixture, home string) error {
	source, err := disk.Read(filepath.Join(fixture, "auth", "auth.json"))
	if err != nil {
		return fmt.Errorf("read dummy fixture authentication: %w", err)
	}

	if string(source) != dummyWriterAuth {
		return fmt.Errorf("%w: fixture must contain only declared dummy authentication", errWriterSpawn)
	}

	err = verifyPrivatePath(home, true)
	if err != nil {
		return err
	}

	path := filepath.Join(home, "auth.json")

	err = verifyPrivatePath(path, false)
	if err != nil {
		return err
	}

	copied, err := disk.Read(path)
	if err != nil {
		return fmt.Errorf("read dummy copy: %w", err)
	}

	if !bytes.Equal(copied, source) {
		return fmt.Errorf("%w: authentication differs from dummy fixture", errWriterSpawn)
	}

	entries, err := os.ReadDir(home)
	if err != nil {
		return fmt.Errorf("list generated home: %w", err)
	}

	if len(entries) != initialWriterHomeFiles {
		return fmt.Errorf("%w: generated home must contain only auth.json and config.toml at spawn", errWriterSpawn)
	}

	return nil
}

func verifyPrivatePath(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%w: private path: %w", errWriterSpawn, err)
	}

	want := os.FileMode(privateWriterFile)
	if directory {
		want = privateWriterDirectory
	}

	if info.IsDir() != directory || info.Mode().Perm() != want ||
		info.Mode()&os.ModeSymlink != 0 || (!directory && !info.Mode().IsRegular()) {
		return fmt.Errorf("%w: private path has wrong type or permissions", errWriterSpawn)
	}

	return nil
}

func verifyWriterRoots(cwd, home string) error {
	entries, err := os.ReadDir(filepath.Join(cwd, "services", "calc"))
	if err != nil {
		return fmt.Errorf("%w: edit root absent before spawn: %w", errWriterSpawn, err)
	}

	if len(entries) != 0 {
		return fmt.Errorf("%w: edit root must be empty before controlled writer", errWriterSpawn)
	}

	for _, name := range []string{"codex-shell-home", "codex-tmp", "codex-go-build"} {
		err = verifyPrivatePath(filepath.Join(cwd, "tmp", name), true)
		if err != nil {
			return err
		}
	}

	config := filepath.Join(home, "config.toml")

	err = verifyPrivatePath(config, false)
	if err != nil {
		return err
	}

	content, err := disk.Read(config)
	if err != nil {
		return fmt.Errorf("read generated writer configuration: %w", err)
	}

	denial := strconv.Quote(filepath.Join(cwd, "tmp", "codex-homes")) + " = \"deny\"\n"
	if !strings.Contains(string(content), denial) {
		return fmt.Errorf("%w: generated homes are not protected", errWriterSpawn)
	}

	return nil
}
