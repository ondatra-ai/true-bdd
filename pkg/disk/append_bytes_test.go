package disk_test

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func TestAppendBytesPreservesChunks(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "stream")
	for _, chunk := range []string{"session ", "id: 175\n", "tail"} {
		err := disk.AppendBytes(path, []byte(chunk), disk.Private)
		if err != nil {
			t.Fatal(err)
		}
	}

	data, err := disk.Read(path)
	if err != nil || string(data) != "session id: 175\ntail" {
		t.Fatalf("%q %v", data, err)
	}
}
