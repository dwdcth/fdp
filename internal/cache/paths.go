package cache

import (
	"fmt"
	"path/filepath"
	"strings"
)

func BlobPath(cacheDir, digest string) string {
	algo, encoded, ok := splitDigest(digest)
	if !ok {
		return filepath.Join(cacheDir, "blobs", "invalid", sanitizePathPart(digest))
	}
	return filepath.Join(cacheDir, "blobs", algo, encoded)
}

func ManifestPath(cacheDir, registry, repository, reference string) string {
	return filepath.Join(cacheDir, "manifests", registry, filepath.FromSlash(repository), sanitizePathPart(reference)+".json")
}

func splitDigest(digest string) (algo, encoded string, ok bool) {
	parts := strings.SplitN(digest, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func sanitizePathPart(value string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
	)
	sanitized := replacer.Replace(strings.TrimSpace(value))
	if sanitized == "" {
		return "unknown"
	}
	return sanitized
}

func invalidDigestError(digest string) error {
	return fmt.Errorf("invalid digest: %s", digest)
}
