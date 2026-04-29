package registry

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	requestRetryAttempts = 3
	maxBlobRedirectDepth = 5
)

var manifestAccept = []string{
	"application/vnd.oci.image.index.v1+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.oci.image.manifest.v1+json",
	"application/vnd.docker.distribution.manifest.v2+json",
}

type Client struct {
	httpClient *http.Client
	selector   *MirrorSelector
}

type ManifestResponse struct {
	Body          []byte
	Digest        string
	MediaType     string
	Endpoint      string
	AuthReference string
}

type DownloadLocation struct {
	URL           string
	Endpoint      string
	AuthReference string
	Headers       []string
}

type endpointError struct {
	endpoint            string
	statusCode          int
	retryable           bool
	credentialsRequired bool
	err                 error
}

func (e *endpointError) Error() string {
	return e.err.Error()
}

func (e *endpointError) Unwrap() error {
	return e.err
}

func NewClient(mirrors []string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		selector:   NewMirrorSelector(mirrors),
	}
}

func (c *Client) GetManifest(registryHost, repository, reference string) (ManifestResponse, error) {
	var lastErr error
	for _, endpoint := range c.selector.Candidates(registryHost) {
		resp, err := c.getManifestFromEndpointWithRetry(endpoint, registryHost, repository, reference)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if isCredentialsRequired(err) && !isSourceEndpoint(endpoint, registryHost) {
			continue
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("manifest request failed")
	}
	return ManifestResponse{}, lastErr
}

func (c *Client) GetManifestByDigest(registryHost, repository, digest string) (ManifestResponse, error) {
	return c.GetManifest(registryHost, repository, digest)
}

func (c *Client) ResolveBlobURL(registryHost, repository, digest string) (DownloadLocation, error) {
	var lastErr error
	for _, endpoint := range c.selector.Candidates(registryHost) {
		loc, err := c.resolveBlobCandidateWithRetry(endpoint, registryHost, repository, digest)
		if err == nil {
			return loc, nil
		}
		lastErr = err
		if isCredentialsRequired(err) && !isSourceEndpoint(endpoint, registryHost) {
			continue
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("blob request failed")
	}
	return DownloadLocation{}, lastErr
}

func (c *Client) getManifestFromEndpointWithRetry(endpoint, registryHost, repository, reference string) (ManifestResponse, error) {
	var lastErr error
	for attempt := 0; attempt < requestRetryAttempts; attempt++ {
		resp, err := c.getManifestFromEndpoint(endpoint, registryHost, repository, reference, "")
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}
	return ManifestResponse{}, lastErr
}

func (c *Client) resolveBlobCandidateWithRetry(endpoint, registryHost, repository, digest string) (DownloadLocation, error) {
	var lastErr error
	for attempt := 0; attempt < requestRetryAttempts; attempt++ {
		loc, err := c.resolveBlobFromEndpoint(endpoint, registryHost, repository, digest, "")
		if err != nil {
			lastErr = err
			if !isRetryable(err) {
				break
			}
			continue
		}
		resolved, err := c.followBlobRedirect(loc, repository, digest, 0)
		if err == nil {
			return resolved, nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}
	return DownloadLocation{}, lastErr
}

func (c *Client) getManifestFromEndpoint(endpoint, registryHost, repository, reference, token string) (ManifestResponse, error) {
	requestURL, authRef, err := buildRegistryURL(endpoint, repository, path.Join("manifests", reference))
	if err != nil {
		return ManifestResponse{}, err
	}
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return ManifestResponse{}, err
	}
	for _, accept := range manifestAccept {
		req.Header.Add("Accept", accept)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ManifestResponse{}, newRetryableError(endpoint, fmt.Errorf("get manifest from %s failed: %w", endpoint, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized && token == "" {
		challenge, err := ParseWWWAuthenticate(resp.Header.Get("WWW-Authenticate"))
		if err != nil {
			return ManifestResponse{}, newCredentialsRequiredError(endpoint, resp.StatusCode, err)
		}
		if challenge.Scope == "" {
			challenge.Scope = "repository:" + repository + ":pull"
		}
		token, err := FetchBearerToken(c.httpClient, challenge)
		if err != nil {
			return ManifestResponse{}, newRetryableError(endpoint, fmt.Errorf("fetch bearer token for %s failed: %w", endpoint, err))
		}
		return c.getManifestFromEndpoint(endpoint, registryHost, repository, reference, token)
	}
	if resp.StatusCode == http.StatusForbidden && token == "" {
		return ManifestResponse{}, newCredentialsRequiredError(endpoint, resp.StatusCode, nil)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("get manifest from %s failed: %s body=%s", endpoint, resp.Status, string(body))
		return ManifestResponse{}, newStatusError(endpoint, resp.StatusCode, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ManifestResponse{}, newRetryableError(endpoint, err)
	}
	if len(body) == 0 {
		return ManifestResponse{}, fmt.Errorf("get manifest from %s failed: empty body", endpoint)
	}
	return ManifestResponse{
		Body:          body,
		Digest:        resp.Header.Get("Docker-Content-Digest"),
		MediaType:     resp.Header.Get("Content-Type"),
		Endpoint:      endpoint,
		AuthReference: authRefOrDefault(authRef, registryHost),
	}, nil
}

func (c *Client) resolveBlobFromEndpoint(endpoint, registryHost, repository, digest, token string) (DownloadLocation, error) {
	requestURL, authRef, err := buildRegistryURL(endpoint, repository, path.Join("blobs", digest))
	if err != nil {
		return DownloadLocation{}, err
	}
	client := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		client.Transport = transport.Clone()
	}
	if transport, ok := c.httpClient.Transport.(http.RoundTripper); ok && client.Transport == nil {
		client.Transport = transport
	}
	if c.httpClient.Jar != nil {
		client.Jar = c.httpClient.Jar
	}

	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return DownloadLocation{}, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return DownloadLocation{}, newRetryableError(endpoint, fmt.Errorf("get blob url from %s failed: %w", endpoint, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized && token == "" {
		challenge, err := ParseWWWAuthenticate(resp.Header.Get("WWW-Authenticate"))
		if err != nil {
			return DownloadLocation{}, newCredentialsRequiredError(endpoint, resp.StatusCode, err)
		}
		if challenge.Scope == "" {
			challenge.Scope = "repository:" + repository + ":pull"
		}
		token, err := FetchBearerToken(c.httpClient, challenge)
		if err != nil {
			return DownloadLocation{}, newRetryableError(endpoint, fmt.Errorf("fetch bearer token for %s failed: %w", endpoint, err))
		}
		return c.resolveBlobFromEndpoint(endpoint, registryHost, repository, digest, token)
	}
	if resp.StatusCode == http.StatusForbidden && token == "" {
		return DownloadLocation{}, newCredentialsRequiredError(endpoint, resp.StatusCode, nil)
	}

	headers := bearerHeaders(token)
	if isRedirect(resp.StatusCode) {
		location := resp.Header.Get("Location")
		if location == "" {
			return DownloadLocation{}, fmt.Errorf("redirect without location from %s", endpoint)
		}
		resolvedURL, err := resolveRedirectURL(requestURL, location)
		if err != nil {
			return DownloadLocation{}, err
		}
		return DownloadLocation{URL: resolvedURL, Endpoint: endpoint, AuthReference: authRefOrDefault(authRef, registryHost), Headers: headers}, nil
	}
	if resp.StatusCode == http.StatusOK {
		return DownloadLocation{URL: requestURL, Endpoint: endpoint, AuthReference: authRefOrDefault(authRef, registryHost), Headers: headers}, nil
	}
	body, _ := io.ReadAll(resp.Body)
	err = fmt.Errorf("get blob url from %s failed: %s body=%s", endpoint, resp.Status, string(body))
	return DownloadLocation{}, newStatusError(endpoint, resp.StatusCode, err)
}

func (c *Client) followBlobRedirect(loc DownloadLocation, repository, digest string, depth int) (DownloadLocation, error) {
	if depth >= maxBlobRedirectDepth {
		return DownloadLocation{}, fmt.Errorf("blob redirect too deep for %s", digest)
	}
	parsed, err := url.Parse(loc.URL)
	if err != nil {
		return DownloadLocation{}, err
	}
	if parsed.RawQuery != "" {
		loc.Headers = nil
		return loc, nil
	}
	if !isRegistryBlobPath(parsed.Path) {
		if isCrossHost(loc.Endpoint, parsed.Host) {
			loc.Headers = nil
		}
		return loc, nil
	}
	nextEndpoint := endpointBaseFromBlobURL(parsed)
	if nextEndpoint == "" || sameEndpointBase(nextEndpoint, loc.Endpoint) {
		return loc, nil
	}
	resolved, err := c.resolveBlobCandidateWithRetry(nextEndpoint, parsed.Host, repository, digest)
	if err != nil {
		return DownloadLocation{}, err
	}
	return c.followBlobRedirect(resolved, repository, digest, depth+1)
}

func buildRegistryURL(base, repository, suffix string) (string, string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", "", err
	}
	fullPath := strings.TrimRight(parsed.Path, "/") + "/v2/" + strings.TrimLeft(repository, "/") + "/" + strings.TrimLeft(suffix, "/")
	parsed.Path = fullPath
	return parsed.String(), parsed.Host, nil
}

func authRefOrDefault(authRef, fallback string) string {
	if authRef != "" {
		return authRef
	}
	return fallback
}

func bearerHeaders(token string) []string {
	if token == "" {
		return nil
	}
	return []string{"Authorization: Bearer " + token}
}

func isRedirect(code int) bool {
	return code == http.StatusTemporaryRedirect || code == http.StatusFound || code == http.StatusSeeOther || code == http.StatusPermanentRedirect
}

func newRetryableError(endpoint string, err error) error {
	return &endpointError{endpoint: endpoint, retryable: true, err: err}
}

func newCredentialsRequiredError(endpoint string, statusCode int, cause error) error {
	if cause != nil {
		return &endpointError{endpoint: endpoint, statusCode: statusCode, credentialsRequired: true, err: fmt.Errorf("endpoint %s requires credentials (status %d): %w", endpoint, statusCode, cause)}
	}
	return &endpointError{endpoint: endpoint, statusCode: statusCode, credentialsRequired: true, err: fmt.Errorf("endpoint %s requires credentials (status %d)", endpoint, statusCode)}
}

func newStatusError(endpoint string, statusCode int, err error) error {
	return &endpointError{endpoint: endpoint, statusCode: statusCode, retryable: isRetryableStatus(statusCode), err: err}
}

func isRetryable(err error) bool {
	var endpointErr *endpointError
	if errors.As(err, &endpointErr) {
		return endpointErr.retryable
	}
	var netErr net.Error
	return errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary())
}

func isCredentialsRequired(err error) bool {
	var endpointErr *endpointError
	return errors.As(err, &endpointErr) && endpointErr.credentialsRequired
}

func isRetryableStatus(code int) bool {
	return code == http.StatusRequestTimeout || code == http.StatusTooManyRequests || code == http.StatusBadGateway || code == http.StatusServiceUnavailable || code == http.StatusGatewayTimeout
}

func isSourceEndpoint(endpoint, registryHost string) bool {
	return sameEndpointBase(endpoint, normalizeBaseURL("https://"+registryHost))
}

func sameEndpointBase(a, b string) bool {
	return normalizeBaseURL(a) == normalizeBaseURL(b)
}

func isRegistryBlobPath(path string) bool {
	return strings.Contains(path, "/v2/") && strings.Contains(path, "/blobs/")
}

func endpointBaseFromBlobURL(parsed *url.URL) string {
	base := *parsed
	idx := strings.Index(base.Path, "/v2/")
	if idx >= 0 {
		base.Path = strings.TrimRight(base.Path[:idx], "/")
	}
	base.RawPath = ""
	base.RawQuery = ""
	base.Fragment = ""
	return strings.TrimRight(base.String(), "/")
}

func isCrossHost(endpoint, host string) bool {
	if host == "" {
		return false
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	return parsed.Host != host
}

func resolveRedirectURL(requestURL, location string) (string, error) {
	requestParsed, err := url.Parse(requestURL)
	if err != nil {
		return "", err
	}
	locationParsed, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return requestParsed.ResolveReference(locationParsed).String(), nil
}
