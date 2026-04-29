package export

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func WriteOCIArchive(outputPath string, files map[string]string, indexJSON []byte, layoutJSON []byte) error {
	writer, closer, err := newOCIWriter(outputPath)
	if err != nil {
		return err
	}
	defer closer()

	tw := tar.NewWriter(writer)
	defer tw.Close()

	if err := addBytes(tw, "oci-layout", layoutJSON); err != nil {
		return err
	}
	if err := addBytes(tw, "index.json", indexJSON); err != nil {
		return err
	}
	for name, src := range files {
		if err := addFile(tw, name, src); err != nil {
			return err
		}
	}
	return nil
}

func newOCIWriter(outputPath string) (io.Writer, func() error, error) {
	if dir := filepath.Dir(outputPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, err
		}
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return nil, nil, err
	}
	if strings.HasSuffix(outputPath, ".gz") {
		gz := gzip.NewWriter(f)
		return gz, func() error {
			if err := gz.Close(); err != nil {
				_ = f.Close()
				return err
			}
			return f.Close()
		}, nil
	}
	return f, f.Close, nil
}

func ValidateOCIInput(files map[string]string) error {
	if len(files) == 0 {
		return fmt.Errorf("oci archive requires blobs")
	}
	return nil
}
