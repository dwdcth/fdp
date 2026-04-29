package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"docker_aria2c/internal/cache"
	"docker_aria2c/internal/cli"
	"docker_aria2c/internal/downloader"
	"docker_aria2c/internal/export"
	"docker_aria2c/internal/manifest"
	"docker_aria2c/internal/reference"
	"docker_aria2c/internal/registry"
	"docker_aria2c/internal/scheduler"
	"docker_aria2c/internal/state"
)

func Run(args []string) error {
	opts, err := cli.Parse(args)
	if err != nil {
		return err
	}
	platform, err := manifest.ParsePlatform(opts.Platform)
	if err != nil {
		return err
	}
	imageRef, err := reference.Parse(opts.ImageRef)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(opts.CacheDir, 0o755); err != nil {
		return err
	}
	store, err := state.Open(opts.StateDB)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	client := registry.NewClient(opts.Mirrors)

	rootManifest, err := client.GetManifest(imageRef.Registry, imageRef.Repository, imageRef.Reference)
	if err != nil {
		return err
	}
	rootMediaType := manifest.DetectMediaType(rootManifest.Body, rootManifest.MediaType)
	indexDigest := firstNonEmpty(rootManifest.Digest, digestString(rootManifest.Body))
	platformManifest := rootManifest
	if manifest.IsManifestList(rootMediaType) {
		selected, err := manifest.SelectPlatformManifest(rootManifest.Body, platform)
		if err != nil {
			return err
		}
		platformManifest, err = client.GetManifestByDigest(imageRef.Registry, imageRef.Repository, selected.Digest)
		if err != nil {
			return err
		}
	}
	imageManifest, err := manifest.DecodeImageManifest(platformManifest.Body)
	if err != nil {
		return err
	}
	resolvedMediaType := manifest.DetectMediaType(platformManifest.Body, platformManifest.MediaType)
	if !manifest.IsImageManifest(resolvedMediaType) {
		return fmt.Errorf("unsupported manifest media type: %s", resolvedMediaType)
	}

	manifestPath := cache.ManifestPath(opts.CacheDir, imageRef.Registry, imageRef.Repository, imageRef.Reference)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(manifestPath, platformManifest.Body, 0o644); err != nil {
		return err
	}

	stateKey := state.ImageState{
		Registry:            imageRef.Registry,
		Repository:          imageRef.Repository,
		Reference:           imageRef.Reference,
		PlatformOS:          platform.OS,
		PlatformArch:        platform.Architecture,
		PlatformVariant:     platform.Variant,
		IndexDigest:         indexDigest,
		ImageManifestDigest: firstNonEmpty(platformManifest.Digest, digestString(platformManifest.Body)),
	}
	prevState, found, err := store.GetImageState(ctx, stateKey)
	if err != nil {
		return err
	}

	tasks := make([]downloader.Task, 0, len(imageManifest.Layers)+1)
	allDescriptors := append([]manifest.Descriptor{imageManifest.Config}, imageManifest.Layers...)
	allCached := true
	for idx, desc := range allDescriptors {
		kind := "layer"
		if idx == 0 {
			kind = "config"
		}
		blobPath := cache.BlobPath(opts.CacheDir, desc.Digest)
		if cache.ExistsAndValid(blobPath, desc.Digest, desc.Size) {
			if err := store.UpsertBlobState(ctx, state.BlobState{Digest: desc.Digest, Size: desc.Size, MediaType: desc.MediaType, LocalPath: blobPath, Kind: kind, Verified: true}); err != nil {
				return err
			}
			continue
		}
		allCached = false
		location, err := client.ResolveBlobURL(imageRef.Registry, imageRef.Repository, desc.Digest)
		if err != nil {
			return err
		}
		tasks = append(tasks, downloader.Task{
			Digest:    desc.Digest,
			Size:      desc.Size,
			MediaType: desc.MediaType,
			Kind:      kind,
			Path:      blobPath,
			URL:       location.URL,
			Headers:   location.Headers,
		})
	}

	if found && prevState.ImageManifestDigest == stateKey.ImageManifestDigest && allCached {
		return writeOutput(opts, platformManifest.Body, imageManifest, imageRef, inputFromManifest(opts.ImageRef, imageRef.Reference, imageManifest, opts.CacheDir))
	}

	dlCfg := downloader.Config{Aria2cPath: opts.Aria2c, Split: opts.AriaSplit, Conn: opts.AriaConn}
	if err := scheduler.Run(tasks, opts.Workers, func(task downloader.Task) error {
		if err := downloader.Download(dlCfg, task); err != nil {
			return err
		}
		if err := cache.VerifyDigest(task.Path, task.Digest); err != nil {
			return err
		}
		return store.UpsertBlobState(ctx, state.BlobState{
			Digest:    task.Digest,
			Size:      task.Size,
			MediaType: task.MediaType,
			LocalPath: task.Path,
			Kind:      task.Kind,
			Verified:  true,
		})
	}); err != nil {
		return err
	}

	if err := store.UpsertImageState(ctx, stateKey); err != nil {
		return err
	}
	return writeOutput(opts, platformManifest.Body, imageManifest, imageRef, inputFromManifest(opts.ImageRef, imageRef.Reference, imageManifest, opts.CacheDir))
}

func inputFromManifest(imageRefRaw, tag string, imageManifest manifest.ImageManifest, cacheDir string) export.ArchiveInput {
	input := export.ArchiveInput{
		ImageRef:         imageRefRaw,
		Tag:              tag,
		ConfigPath:       cache.BlobPath(cacheDir, imageManifest.Config.Digest),
		ConfigDescriptor: imageManifest.Config,
		Layers:           make([]export.LayerFile, 0, len(imageManifest.Layers)),
	}
	for _, layer := range imageManifest.Layers {
		input.Layers = append(input.Layers, export.LayerFile{
			Descriptor: layer,
			Path:       cache.BlobPath(cacheDir, layer.Digest),
		})
	}
	return input
}

func writeOutput(opts cli.Options, platformManifestBody []byte, imageManifest manifest.ImageManifest, imageRef reference.ImageReference, input export.ArchiveInput) error {
	if opts.Format == "oci" {
		files, indexJSON, layoutJSON, err := buildOCIArtifacts(platformManifestBody, imageManifest, input)
		if err != nil {
			return err
		}
		if err := export.ValidateOCIInput(files); err != nil {
			return err
		}
		return export.WriteOCIArchive(opts.Output, files, indexJSON, layoutJSON)
	}
	if err := export.ValidateDockerInput(input); err != nil {
		return err
	}
	_ = imageRef
	return export.WriteDockerArchive(opts.Output, input)
}

func buildOCIArtifacts(platformManifestBody []byte, imageManifest manifest.ImageManifest, input export.ArchiveInput) (map[string]string, []byte, []byte, error) {
	files := map[string]string{}
	configRel := filepath.Join("blobs", "sha256", strings.TrimPrefix(imageManifest.Config.Digest, "sha256:"))
	files[configRel] = input.ConfigPath
	for _, layer := range input.Layers {
		rel := filepath.Join("blobs", "sha256", strings.TrimPrefix(layer.Descriptor.Digest, "sha256:"))
		files[rel] = layer.Path
	}

	manifestDigest := digestString(platformManifestBody)
	manifestPath := filepath.Join(os.TempDir(), "dockerpull-manifest-"+strings.TrimPrefix(manifestDigest, "sha256:")+".json")
	if err := os.WriteFile(manifestPath, platformManifestBody, 0o644); err != nil {
		return nil, nil, nil, err
	}
	files[filepath.Join("blobs", "sha256", strings.TrimPrefix(manifestDigest, "sha256:"))] = manifestPath

	indexDoc := map[string]any{
		"schemaVersion": 2,
		"manifests": []map[string]any{{
			"mediaType": imageManifest.MediaType,
			"digest":    manifestDigest,
			"size":      len(platformManifestBody),
			"annotations": map[string]string{
				"org.opencontainers.image.ref.name": input.Tag,
			},
		}},
	}
	indexJSON, err := json.Marshal(indexDoc)
	if err != nil {
		return nil, nil, nil, err
	}
	layoutJSON := []byte("{\n  \"imageLayoutVersion\": \"1.0.0\"\n}\n")
	return files, indexJSON, layoutJSON, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func digestString(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + fmt.Sprintf("%x", sum[:])
}
