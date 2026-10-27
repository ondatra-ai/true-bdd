package drill

import (
	"context"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// ManifestCheck records deletion and offline reconstruction before any cut is made.
type ManifestCheck struct {
	Deleted []string `json:"deleted"`
	Cache   string   `json:"cache"`
	Offline bool     `json:"offline"`
	Passed  bool     `json:"passed"`
	Error   string   `json:"error,omitempty"`
	Log     string   `json:"log"`
}

func checkManifests(ctx context.Context, record Record) (ManifestCheck, error) {
	evidence := ManifestCheck{Cache: filepath.Join(record.Directory, "module-cache"),
		Log: filepath.Join(record.Directory, "manifest-check.log")}
	err := recreateManifests(ctx, record, &evidence)

	evidence.Passed = err == nil
	if err != nil {
		evidence.Error = err.Error()
	}

	return evidence, err
}

func recreateManifests(ctx context.Context, record Record, evidence *ManifestCheck) error {
	source, err := exportBase(ctx, record)
	if err != nil {
		return err
	}
	defer func() { _ = disk.RemoveTree(source) }()

	engine := filepath.Join(source, "services/true-bdd")

	seed, download, err := libraryManifests(source, engine)
	if err != nil {
		return err
	}

	err = prepareManifestCache(ctx, source, download, evidence.Cache)
	if err != nil {
		return err
	}

	err = resetManifests(engine, seed, evidence)
	if err != nil {
		return err
	}

	evidence.Offline = true

	transcript, err := gotool.RestoreOffline(ctx, engine, filepath.Join(source, "restored"), evidence.Cache)

	writeErr := disk.Write(evidence.Log, []byte(transcript), disk.Private)
	if writeErr != nil {
		return writeErr
	}

	if err != nil {
		return err
	}

	return saveRestoredManifests(record.Directory, engine)
}

func resetManifests(engine string, seed []byte, evidence *ManifestCheck) error {
	for _, name := range []string{goModFile, goSumFile} {
		err := disk.Remove(filepath.Join(engine, name))
		if err != nil {
			return err
		}

		evidence.Deleted = append(evidence.Deleted, "services/true-bdd/"+name)
	}

	return disk.Write(filepath.Join(engine, goModFile), seed, disk.Private)
}

func saveRestoredManifests(directory, engine string) error {
	for _, name := range []string{goModFile, goSumFile} {
		data, err := disk.Read(filepath.Join(engine, name))
		if err != nil {
			return err
		}

		err = disk.Write(filepath.Join(directory, "restored", name), data, disk.Private)
		if err != nil {
			return err
		}
	}

	return nil
}

func prepareManifestCache(ctx context.Context, source string, seed []byte, cache string) error {
	directory := filepath.Join(source, "tmp", "manifest-download")

	err := disk.Write(filepath.Join(directory, goModFile), seed, disk.Private)
	if err != nil {
		return err
	}

	_, err = gotool.DownloadModulesInCache(ctx, directory, cache)

	return err
}
