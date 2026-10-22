package codexrun

import "github.com/ondatra-ai/true-bdd/pkg/disk"

type logWriter struct{ path string }

func (w logWriter) Write(data []byte) (int, error) {
	err := disk.AppendBytes(w.path, data, disk.Private)
	if err != nil {
		return 0, err
	}

	return len(data), nil
}
