// Package drill runs isolated rebuild cuts and records their evidence.
package drill

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

var errRefused = errors.New("drill refused")

// Record is the iteration's atomic, private record.json.
type Record struct {
	Directory    string             `json:"directory"`
	Repository   string             `json:"repository"`
	Base         string             `json:"base"`
	Binary       string             `json:"binary"`
	Digest       string             `json:"digest"`
	StartedAt    time.Time          `json:"started_at"`
	AllowedRoots []string           `json:"allowed_roots"`
	Cuts         map[string]Cut     `json:"cuts"`
	Runs         map[string]Run     `json:"runs"`
	Audits       map[string][]Audit `json:"audits"`
}

// Cut binds one private tree to the iteration's immutable inputs.
type Cut struct {
	Directory string `json:"directory"`
	Base      string `json:"base"`
	Digest    string `json:"digest"`
	Commit    string `json:"commit"`
}

// Read loads and checks the iteration's engine identity.
func Read(directory string) (Record, error) {
	var record Record

	data, err := disk.Read(filepath.Join(directory, "record.json"))
	if err != nil {
		return record, err
	}

	err = json.Unmarshal(data, &record)
	if err != nil {
		return record, fmt.Errorf("reading iteration: %w", err)
	}

	if record.Directory != directory {
		return record, fmt.Errorf("%w: iteration directory changed", errRefused)
	}

	err = checkIdentity(record)
	if err != nil {
		return record, err
	}

	digest, err := fingerprint(record.Binary)
	if err != nil {
		return record, err
	}

	if digest != record.Digest {
		return record, fmt.Errorf("%w: binary changed", errRefused)
	}

	return record, nil
}

func fingerprint(path string) (string, error) {
	data, err := disk.Read(path)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:]), nil
}

func save(record Record) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding iteration: %w", err)
	}

	return disk.Write(filepath.Join(record.Directory, "record.json"), append(data, '\n'), disk.Private)
}

func change(directory string, edit func(*Record) error) error {
	return disk.Update(filepath.Join(directory, "record.json"), disk.Private, func(data []byte) ([]byte, error) {
		var record Record

		err := json.Unmarshal(data, &record)
		if err != nil {
			return nil, fmt.Errorf("decoding iteration update: %w", err)
		}

		err = edit(&record)
		if err != nil {
			return nil, err
		}

		after, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encoding iteration update: %w", err)
		}

		return append(after, '\n'), nil
	})
}

func (r Record) cut(name string) (Cut, error) {
	cut, ok := r.Cuts[name]
	if !ok || cut.Base != r.Base || cut.Digest != r.Digest {
		return Cut{}, fmt.Errorf("%w: cut base or binary mismatch", errRefused)
	}

	return cut, nil
}

func loadUnchecked(directory string) (Record, error) {
	var record Record

	data, err := disk.Read(filepath.Join(directory, "record.json"))
	if err != nil {
		return record, err
	}

	err = json.Unmarshal(data, &record)
	if err != nil {
		return record, fmt.Errorf("loading iteration: %w", err)
	}

	return record, nil
}
