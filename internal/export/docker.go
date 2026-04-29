package export

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dwdcth/fdp/internal/manifest"
)

type ArchiveInput struct {
	ImageRef         string
	Tag              string
	ConfigPath       string
	ConfigDescriptor manifest.Descriptor
	ManifestJSON     []byte
	Layers           []LayerFile
}

type LayerFile struct {
	Descriptor manifest.Descriptor
	Path       string
}

func WriteDockerArchive(outputPath string, input ArchiveInput) error {
	writer, closer, err := newArchiveWriter(outputPath)
	if err != nil {
		return err
	}
	defer closer()

	tw := tar.NewWriter(writer)
	defer tw.Close()

	configName := strings.TrimPrefix(input.ConfigDescriptor.Digest, "sha256:") + ".json"
	if err := addFile(tw, configName, input.ConfigPath); err != nil {
		return err
	}

	layerNames := make([]string, 0, len(input.Layers))
	for _, layer := range input.Layers {
		layerName := strings.TrimPrefix(layer.Descriptor.Digest, "sha256:") + ".tar.gz"
		layerNames = append(layerNames, layerName)
		if err := addFile(tw, layerName, layer.Path); err != nil {
			return err
		}
	}

	manifestDoc := []map[string]any{{
		"Config":   configName,
		"RepoTags": []string{input.ImageRef},
		"Layers":   layerNames,
	}}
	manifestBytes, err := json.Marshal(manifestDoc)
	if err != nil {
		return err
	}
	if err := addBytes(tw, "manifest.json", manifestBytes); err != nil {
		return err
	}

	repositoriesDoc := map[string]map[string]string{}
	repo, tag := splitImageRef(input.ImageRef, input.Tag)
	repositoriesDoc[repo] = map[string]string{tag: strings.TrimSuffix(configName, ".json")}
	repositoriesBytes, err := json.Marshal(repositoriesDoc)
	if err != nil {
		return err
	}
	return addBytes(tw, "repositories", repositoriesBytes)
}

func newArchiveWriter(outputPath string) (io.Writer, func() error, error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil && filepath.Dir(outputPath) != "." {
		return nil, nil, err
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

func addFile(tw *tar.Writer, name, src string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: info.Size()}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, file)
	return err
}

func addBytes(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func splitImageRef(imageRef, defaultTag string) (string, string) {
	lastSlash := strings.LastIndex(imageRef, "/")
	lastColon := strings.LastIndex(imageRef, ":")
	if lastColon > lastSlash {
		return imageRef[:lastColon], imageRef[lastColon+1:]
	}
	return imageRef, defaultTag
}

func ValidateDockerInput(input ArchiveInput) error {
	if input.ConfigPath == "" || len(input.Layers) == 0 {
		return fmt.Errorf("docker archive requires config and layers")
	}
	return nil
}
