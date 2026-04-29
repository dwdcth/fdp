package registry

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGetManifestRetriesTransientErrors(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/library/nginx/manifests/latest" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		call := calls.Add(1)
		if call < 3 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", manifestAccept[0])
		w.Header().Set("Docker-Content-Digest", "sha256:test-manifest")
		_, _ = w.Write([]byte(`{"schemaVersion":2}`))
	}))
	defer server.Close()

	client := NewClient(nil)
	client.httpClient = insecureTestClient()

	resp, err := client.GetManifest(strings.TrimPrefix(server.URL, "https://"), "library/nginx", "latest")
	if err != nil {
		t.Fatalf("GetManifest returned error: %v", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected 3 manifest attempts, got %d", calls.Load())
	}
	if got := string(resp.Body); got != `{"schemaVersion":2}` {
		t.Fatalf("unexpected manifest body: %s", got)
	}
	if resp.Digest != "sha256:test-manifest" {
		t.Fatalf("unexpected digest: %s", resp.Digest)
	}
}

func TestResolveBlobURLSkipsCredentialMirrorAndFallsBackToSource(t *testing.T) {
	var mirrorCalls atomic.Int32
	mirror := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mirrorCalls.Add(1)
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer mirror.Close()

	var sourceURL string
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/library/nginx/blobs/sha256:test-blob" {
			t.Fatalf("unexpected source path: %s", r.URL.Path)
		}
		w.Header().Set("Location", sourceURL+"/signed/blob?sig=ok")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	sourceURL = source.URL

	client := NewClient([]string{mirror.URL})
	client.httpClient = insecureTestClient()

	loc, err := client.ResolveBlobURL(strings.TrimPrefix(source.URL, "https://"), "library/nginx", "sha256:test-blob")
	if err != nil {
		t.Fatalf("ResolveBlobURL returned error: %v", err)
	}
	if mirrorCalls.Load() == 0 {
		t.Fatalf("expected mirror to be attempted")
	}
	if want := source.URL + "/signed/blob?sig=ok"; loc.URL != want {
		t.Fatalf("unexpected blob url: got %s want %s", loc.URL, want)
	}
	if len(loc.Headers) != 0 {
		t.Fatalf("expected signed URL to clear auth headers, got %v", loc.Headers)
	}
}

func TestResolveBlobURLReauthsOnRedirectedRegistryHost(t *testing.T) {
	const digest = "sha256:test-blob"
	var tokenCalls atomic.Int32
	var authCalls atomic.Int32

	var redirectTarget string
	var registryBURL string
	registryB := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"registry-b-token"}`))
		case "/v2/library/nginx/blobs/" + digest:
			if r.Header.Get("Authorization") == "" {
				w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s/token",service="registry-b"`, registryBURL))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			authCalls.Add(1)
			w.Header().Set("Location", registryBURL+"/signed/blob?sig=ok")
			w.WriteHeader(http.StatusTemporaryRedirect)
		default:
			t.Fatalf("unexpected registryB path: %s", r.URL.Path)
		}
	}))
	defer registryB.Close()
	registryBURL = registryB.URL
	redirectTarget = registryB.URL + "/v2/library/nginx/blobs/" + digest

	registryA := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/library/nginx/blobs/"+digest {
			t.Fatalf("unexpected registryA path: %s", r.URL.Path)
		}
		w.Header().Set("Location", redirectTarget)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer registryA.Close()

	client := NewClient([]string{registryA.URL})
	client.httpClient = insecureTestClient()

	loc, err := client.ResolveBlobURL("registry-1.docker.io", "library/nginx", digest)
	if err != nil {
		t.Fatalf("ResolveBlobURL returned error: %v", err)
	}
	if tokenCalls.Load() == 0 {
		t.Fatalf("expected token endpoint to be called")
	}
	if authCalls.Load() == 0 {
		t.Fatalf("expected authenticated blob resolution on redirected host")
	}
	if want := registryB.URL + "/signed/blob?sig=ok"; loc.URL != want {
		t.Fatalf("unexpected final blob url: got %s want %s", loc.URL, want)
	}
	if len(loc.Headers) != 0 {
		t.Fatalf("expected final signed URL to omit auth headers, got %v", loc.Headers)
	}
}

func insecureTestClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}
