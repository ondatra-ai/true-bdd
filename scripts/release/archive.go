package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func unpack(directory string, data []byte, compressed bool) error {
	var reader io.Reader = bytes.NewReader(data)
	if compressed {
		zipped, err := gzip.NewReader(reader)
		if err != nil {
			return fmt.Errorf("reading gzip: %w", err)
		}
		defer func() { _ = zipped.Close() }()

		reader = zipped
	}

	archive := tar.NewReader(reader)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		err = unpackEntry(directory, header, archive)
		if err != nil {
			return err
		}
	}
}

func unpackEntry(directory string, header *tar.Header, reader io.Reader) error {
	if header.Typeflag == tar.TypeDir || header.Typeflag == tar.TypeXGlobalHeader {
		return nil
	}

	if !filepath.IsLocal(header.Name) {
		return fmt.Errorf("%w: unsafe archive path %s", errRefused, header.Name)
	}

	target := filepath.Join(directory, header.Name)
	if header.Typeflag == tar.TypeSymlink {
		return unpackLink(target, header.Linkname)
	}

	if header.Typeflag != tar.TypeReg {
		return fmt.Errorf("%w: unsupported archive entry %s", errRefused, header.Name)
	}

	contents, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("reading %s: %w", header.Name, err)
	}

	err = disk.Write(target, contents, disk.Private)
	if err != nil {
		return fmt.Errorf("exporting %s: %w", target, err)
	}

	if header.Mode&0o100 != 0 {
		err = disk.Executable(target)
		if err != nil {
			return fmt.Errorf("exporting executable: %w", err)
		}
	}

	return nil
}

func unpackLink(target, link string) error {
	if !filepath.IsLocal(link) {
		return fmt.Errorf("%w: unsafe archive link %s", errRefused, link)
	}

	err := disk.Dir(filepath.Dir(target), disk.Private)
	if err != nil {
		return fmt.Errorf("preparing link: %w", err)
	}

	err = os.Symlink(link, target)
	if err != nil {
		return fmt.Errorf("exporting link: %w", err)
	}

	return nil
}

func packBinary(binary []byte) ([]byte, error) {
	var buffer bytes.Buffer

	zipped := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(zipped)

	const binaryMode = 0o755

	err := archive.WriteHeader(&tar.Header{Name: "true-bdd", Mode: binaryMode, Size: int64(len(binary))})
	if err == nil {
		_, err = archive.Write(binary)
	}

	if err == nil {
		err = archive.Close()
	}

	if err == nil {
		err = zipped.Close()
	}

	if err != nil {
		return nil, fmt.Errorf("packing release binary: %w", err)
	}

	return buffer.Bytes(), nil
}
