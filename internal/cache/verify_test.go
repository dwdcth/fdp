package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestBlobPath(t *testing.T) {
	got := BlobPath("/tmp/cache", "sha256:abc123")
	want := filepath.Join("/tmp/cache", "blobs", "sha256", "abc123")
	if got != want {
		t.Fatalf("BlobPath() = %q, want %q", got, want)
	}
}

func TestManifestPath(t *testing.T) {
	got := ManifestPath("/tmp/cache", "registry-1.docker.io", "library/nginx", "latest")
	want := filepath.Join("/tmp/cache", "manifests", "registry-1.docker.io", "library", "nginx", "latest.json")
	if got != want {
		t.Fatalf("ManifestPath() = %q, want %q", got, want)
	}
}

func TestExistsAndValid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "blob")
	content := []byte("hello cache")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	sum := sha256.Sum256(content)
	digest := fmt.Sprintf("sha256:%x", sum[:])

	if !ExistsAndValid(path, digest, int64(len(content))) {
		t.Fatal("ExistsAndValid() = false, want true")
	}
	if ExistsAndValid(path, digest, int64(len(content)+1)) {
		t.Fatal("ExistsAndValid() = true for wrong size, want false")
	}
	if ExistsAndValid(path, "sha256:deadbeef", int64(len(content))) {
		t.Fatal("ExistsAndValid() = true for wrong digest, want false")
	}
}
