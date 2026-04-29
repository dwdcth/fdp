package manifest

import (
	"encoding/json"
	"fmt"
	"strings"

	"docker_aria2c/internal/registry"
)

func IsManifestList(mediaType string) bool {
	normalized := normalizeMediaType(mediaType)
	return normalized == "application/vnd.oci.image.index.v1+json" ||
		normalized == "application/vnd.docker.distribution.manifest.list.v2+json"
}

func IsImageManifest(mediaType string) bool {
	normalized := normalizeMediaType(mediaType)
	return normalized == "application/vnd.oci.image.manifest.v1+json" ||
		normalized == "application/vnd.docker.distribution.manifest.v2+json"
}

func DetectMediaType(body []byte, header string) string {
	if header != "" {
		return normalizeMediaType(header)
	}
	var payload struct {
		MediaType string `json:"mediaType"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		return normalizeMediaType(payload.MediaType)
	}
	return ""
}

func ParsePlatform(raw string) (registry.Platform, error) {
	parts := strings.Split(raw, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return registry.Platform{}, fmt.Errorf("invalid platform: %s", raw)
	}
	platform := registry.Platform{
		OS:           parts[0],
		Architecture: parts[1],
	}
	if len(parts) == 3 {
		platform.Variant = parts[2]
	}
	if platform.OS == "" || platform.Architecture == "" {
		return registry.Platform{}, fmt.Errorf("invalid platform: %s", raw)
	}
	return platform, nil
}

func SelectPlatformManifest(raw []byte, platform registry.Platform) (Descriptor, error) {
	var list ManifestList
	if err := json.Unmarshal(raw, &list); err != nil {
		return Descriptor{}, err
	}
	for _, item := range list.Manifests {
		if item.Platform == nil {
			continue
		}
		if item.Platform.OS != platform.OS || item.Platform.Architecture != platform.Architecture {
			continue
		}
		if platform.Variant != "" && item.Platform.Variant != platform.Variant {
			continue
		}
		return item, nil
	}
	return Descriptor{}, fmt.Errorf("platform not found: %s/%s", platform.OS, platform.Architecture)
}

func DecodeImageManifest(raw []byte) (ImageManifest, error) {
	var doc ImageManifest
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ImageManifest{}, err
	}
	return doc, nil
}

func normalizeMediaType(value string) string {
	return strings.TrimSpace(strings.SplitN(value, ";", 2)[0])
}
