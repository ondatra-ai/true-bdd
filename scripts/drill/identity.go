package drill

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

type identity struct {
	Base   string `json:"base"`
	Binary string `json:"binary"`
	Digest string `json:"digest"`
}

func saveIdentity(record Record) error {
	data, err := json.Marshal(identity{Base: record.Base, Binary: record.Binary, Digest: record.Digest})
	if err != nil {
		return fmt.Errorf("encoding iteration identity: %w", err)
	}

	return disk.Write(filepath.Join(record.Directory, "identity.json"), data, disk.Private)
}

func checkIdentity(record Record) error {
	data, err := disk.Read(filepath.Join(record.Directory, "identity.json"))
	if err != nil {
		return err
	}

	var original identity

	err = json.Unmarshal(data, &original)
	if err != nil {
		return fmt.Errorf("decoding iteration identity: %w", err)
	}

	if original.Base != record.Base || original.Binary != record.Binary || original.Digest != record.Digest {
		return fmt.Errorf("%w: iteration base or binary differs from start", errRefused)
	}

	return nil
}
