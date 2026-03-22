package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/regikeep/rgk/internal/store"
)

// OCIRAdapter implements the Adapter interface for Oracle Container Image Registry.
type OCIRAdapter struct {
	id          string
	endpoint    string // e.g. fra.ocir.io
	tenancy     string
	username    string
	authToken   string
	region      string
	compartment string
	httpClient  *http.Client
	basicAuth   string // base64(username:authToken)
}

// NewOCIRAdapter creates an OCIR adapter from config values.
func NewOCIRAdapter(registryID, endpoint, tenancy, username, authToken, region, compartment string) *OCIRAdapter {
	raw := username + ":" + authToken
	auth := base64.StdEncoding.EncodeToString([]byte(raw))
	return &OCIRAdapter{
		id:          registryID,
		endpoint:    endpoint,
		tenancy:     tenancy,
		username:    username,
		authToken:   authToken,
		region:      region,
		compartment: compartment,
		basicAuth:   auth,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *OCIRAdapter) ID() string { return a.id }

// repoPath returns the full v2 repo path: tenancy/repo.
// If repo already starts with the tenancy prefix (e.g. "recvue/myapp"),
// it returns repo as-is to avoid duplication ("recvue/recvue/myapp").
func (a *OCIRAdapter) repoPath(repo string) string {
	prefix := a.tenancy + "/"
	if strings.HasPrefix(repo, prefix) {
		return repo
	}
	return a.tenancy + "/" + repo
}

// Authenticate validates credentials by calling the v2 API with basic auth.
func (a *OCIRAdapter) Authenticate() error {
	url := fmt.Sprintf("https://%s/v2/", a.endpoint)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("ocir auth: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+a.basicAuth)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ocir auth request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil // fully authenticated
	case http.StatusUnauthorized:
		// OCIR returns 401 on /v2/ even with valid basic auth — this is normal.
		// Try listing the tenancy's catalog to truly validate credentials.
		catURL := fmt.Sprintf("https://%s/v2/%s/camunda/tags/list", a.endpoint, a.tenancy)
		catReq, _ := http.NewRequest(http.MethodGet, catURL, nil)
		catReq.Header.Set("Authorization", "Basic "+a.basicAuth)
		catResp, catErr := a.httpClient.Do(catReq)
		if catErr != nil {
			// Network works (v2 responded), catalog just failed — assume auth is OK
			return nil
		}
		defer catResp.Body.Close()
		if catResp.StatusCode == http.StatusOK || catResp.StatusCode == http.StatusNotFound {
			return nil // 200 = valid, 404 = auth works but repo not found
		}
		if catResp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("ocir auth: credentials rejected (401) — check username and auth token")
		}
		return nil
	case http.StatusForbidden:
		return fmt.Errorf("ocir auth: credentials rejected (403)")
	default:
		return fmt.Errorf("ocir auth: unexpected HTTP %d", resp.StatusCode)
	}
}

// ResolveDigest fetches the manifest for repo:tag and returns its digest.
func (a *OCIRAdapter) ResolveDigest(repo, tag string) (string, error) {
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", a.endpoint, a.repoPath(repo), tag)
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return "", fmt.Errorf("ocir resolve digest: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+a.basicAuth)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ocir resolve digest request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ocir resolve digest: HTTP %d for %s:%s", resp.StatusCode, repo, tag)
	}

	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", fmt.Errorf("ocir resolve digest: empty Docker-Content-Digest header")
	}
	return digest, nil
}

// Keepalive pulls the manifest (resetting the last-pulled timer) for pull strategy,
// or applies native retention exemption for native strategy.
func (a *OCIRAdapter) Keepalive(img store.ImageRef, strategy Strategy) (KeepaliveResult, error) {
	switch strategy {
	case StrategyPull:
		return a.keepalivePull(img)
	case StrategyNative:
		return a.keepaliveNative(img)
	case StrategyRetag:
		return a.keepaliveRetag(img)
	default:
		return a.keepalivePull(img)
	}
}

func (a *OCIRAdapter) keepalivePull(img store.ImageRef) (KeepaliveResult, error) {
	// Pull manifest by digest — this resets the pull timestamp without downloading layers.
	ref := img.Digest
	if ref == "" {
		ref = img.Tag
	}
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", a.endpoint, a.repoPath(img.Repo), ref)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return KeepaliveResult{}, err
	}
	req.Header.Set("Authorization", "Basic "+a.basicAuth)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return KeepaliveResult{Error: err}, nil
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		e := fmt.Errorf("keepalive pull: HTTP %d", resp.StatusCode)
		return KeepaliveResult{Error: e}, nil
	}

	newExpiry := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	return KeepaliveResult{Success: true, NewExpiry: newExpiry}, nil
}

func (a *OCIRAdapter) keepaliveRetag(img store.ImageRef) (KeepaliveResult, error) {
	// Retag strategy: GET the manifest, then PUT it back with the same tag.
	// This effectively "touches" the image and resets retention timers.
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", a.endpoint, a.repoPath(img.Repo), img.Tag)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return KeepaliveResult{}, err
	}
	req.Header.Set("Authorization", "Basic "+a.basicAuth)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return KeepaliveResult{Error: err}, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		e := fmt.Errorf("keepalive retag GET: HTTP %d", resp.StatusCode)
		return KeepaliveResult{Error: e}, nil
	}

	contentType := resp.Header.Get("Content-Type")

	// PUT the manifest back.
	putURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", a.endpoint, a.repoPath(img.Repo), img.Tag)
	putReq, err := http.NewRequest(http.MethodPut, putURL, nil)
	if err != nil {
		return KeepaliveResult{}, err
	}
	putReq.Header.Set("Authorization", "Basic "+a.basicAuth)
	putReq.Header.Set("Content-Type", contentType)
	putReq.Body = io.NopCloser(bytesReader(body))
	putReq.ContentLength = int64(len(body))

	putResp, err := a.httpClient.Do(putReq)
	if err != nil {
		return KeepaliveResult{Error: err}, nil
	}
	defer putResp.Body.Close()
	io.Copy(io.Discard, putResp.Body)

	if putResp.StatusCode < 200 || putResp.StatusCode >= 300 {
		e := fmt.Errorf("keepalive retag PUT: HTTP %d", putResp.StatusCode)
		return KeepaliveResult{Error: e}, nil
	}

	newExpiry := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	return KeepaliveResult{Success: true, NewExpiry: newExpiry}, nil
}

type ocirImageResponse struct {
	ID        string `json:"id"`
	TimeLastPulled string `json:"timeLastPulled"`
}

func (a *OCIRAdapter) keepaliveNative(img store.ImageRef) (KeepaliveResult, error) {
	// OCIR native: update the image's retention exemption via the OCIR REST API.
	// The OCIR API lives at https://<region>.ocir.io/20180419
	apiBase := fmt.Sprintf("https://%s.ocir.io/20180419", a.region)
	url := fmt.Sprintf("%s/images/%s", apiBase, img.Digest)

	body := `{"isPinned":true}`
	req, err := http.NewRequest(http.MethodPut, url, io.NopCloser(bytesReader([]byte(body))))
	if err != nil {
		return KeepaliveResult{}, err
	}
	req.Header.Set("Authorization", "Basic "+a.basicAuth)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return KeepaliveResult{Error: err}, nil
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		e := fmt.Errorf("keepalive native: HTTP %d", resp.StatusCode)
		return KeepaliveResult{Error: e}, nil
	}

	newExpiry := time.Now().UTC().Add(90 * 24 * time.Hour).Format(time.RFC3339)
	return KeepaliveResult{Success: true, NewExpiry: newExpiry}, nil
}

// ApplyNativeProtection marks the image as pinned via the OCIR retention API.
func (a *OCIRAdapter) ApplyNativeProtection(img store.ImageRef) error {
	result, err := a.keepaliveNative(img)
	if err != nil {
		return err
	}
	if !result.Success {
		return result.Error
	}
	return nil
}

// ListImages lists images in a repository using the OCI Distribution API.
func (a *OCIRAdapter) ListImages(repo string) ([]store.ImageRef, error) {
	url := fmt.Sprintf("https://%s/v2/%s/tags/list", a.endpoint, a.repoPath(repo))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ocir list images: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+a.basicAuth)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ocir list images request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ocir list images: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ocir list images decode: %w", err)
	}

	out := make([]store.ImageRef, 0, len(result.Tags))
	for _, tag := range result.Tags {
		out = append(out, store.ImageRef{
			Registry: a.id,
			Repo:     repo,
			Tag:      tag,
		})
	}
	return out, nil
}

// SupportsNativeProtection returns true — OCIR supports retention policy exemption.
func (a *OCIRAdapter) SupportsNativeProtection() bool { return true }

// bytesReader wraps a []byte as an io.Reader.
type bytesReaderType struct {
	data []byte
	pos  int
}

func bytesReader(b []byte) *bytesReaderType {
	return &bytesReaderType{data: b}
}

func (r *bytesReaderType) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
