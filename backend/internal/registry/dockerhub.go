package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/regikeep/rgk/internal/store"
)

// DockerHubAdapter implements the Adapter interface for Docker Hub.
type DockerHubAdapter struct {
	id          string
	namespace   string
	username    string
	accessToken string
	httpClient  *http.Client
	bearerToken string // JWT obtained from auth.docker.io
}

// NewDockerHubAdapter creates a Docker Hub adapter.
func NewDockerHubAdapter(registryID, namespace string) *DockerHubAdapter {
	return &DockerHubAdapter{
		id:        registryID,
		namespace: namespace,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewDockerHubAdapterWithCreds creates a Docker Hub adapter with credentials.
func NewDockerHubAdapterWithCreds(registryID, namespace, username, accessToken string) *DockerHubAdapter {
	return &DockerHubAdapter{
		id:          registryID,
		namespace:   namespace,
		username:    username,
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *DockerHubAdapter) ID() string { return a.id }

// Authenticate obtains a bearer token from Docker Hub's auth service.
func (a *DockerHubAdapter) Authenticate() error {
	// Docker Hub uses token-based auth via auth.docker.io
	url := "https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/alpine:pull"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("dockerhub auth: %w", err)
	}

	// If credentials are provided, use them for authenticated token
	if a.username != "" && a.accessToken != "" {
		req.SetBasicAuth(a.username, a.accessToken)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dockerhub auth request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("dockerhub auth: invalid credentials (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dockerhub auth: HTTP %d", resp.StatusCode)
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("dockerhub auth decode: %w", err)
	}
	if tokenResp.Token == "" {
		return fmt.Errorf("dockerhub auth: empty token")
	}

	a.bearerToken = tokenResp.Token
	return nil
}

// getToken obtains a scoped bearer token for the given repo.
func (a *DockerHubAdapter) getToken(repo, actions string) (string, error) {
	scope := fmt.Sprintf("repository:%s:%s", repo, actions)
	url := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=%s", scope)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if a.username != "" && a.accessToken != "" {
		req.SetBasicAuth(a.username, a.accessToken)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("dockerhub token: HTTP %d", resp.StatusCode)
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}
	return tokenResp.Token, nil
}

// fullRepo returns the full repository path for Docker Hub.
func (a *DockerHubAdapter) fullRepo(repo string) string {
	// Docker Hub official images are under "library/"
	if !strings.Contains(repo, "/") {
		return "library/" + repo
	}
	return repo
}

// ResolveDigest fetches the manifest digest for the given repo:tag.
func (a *DockerHubAdapter) ResolveDigest(repo, tag string) (string, error) {
	fullRepo := a.fullRepo(repo)
	token, err := a.getToken(fullRepo, "pull")
	if err != nil {
		return "", fmt.Errorf("dockerhub resolve digest: %w", err)
	}

	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/manifests/%s", fullRepo, tag)
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("dockerhub resolve digest request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("dockerhub resolve digest: HTTP %d for %s:%s", resp.StatusCode, repo, tag)
	}

	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", fmt.Errorf("dockerhub resolve digest: empty Docker-Content-Digest header")
	}
	return digest, nil
}

// Keepalive performs a pull-strategy keepalive (pull manifest to reset timer).
func (a *DockerHubAdapter) Keepalive(img store.ImageRef, strategy Strategy) (KeepaliveResult, error) {
	fullRepo := a.fullRepo(img.Repo)
	token, err := a.getToken(fullRepo, "pull")
	if err != nil {
		return KeepaliveResult{Error: err}, nil
	}

	ref := img.Digest
	if ref == "" {
		ref = img.Tag
	}
	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/manifests/%s", fullRepo, ref)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return KeepaliveResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
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

// ApplyNativeProtection — Docker Hub supports immutable tags but not via standard API.
func (a *DockerHubAdapter) ApplyNativeProtection(img store.ImageRef) error {
	return ErrNotImplemented{Method: "ApplyNativeProtection", Registry: "dockerhub"}
}

// ListImages lists tags in a repository.
func (a *DockerHubAdapter) ListImages(repo string) ([]store.ImageRef, error) {
	fullRepo := a.fullRepo(repo)
	token, err := a.getToken(fullRepo, "pull")
	if err != nil {
		return nil, fmt.Errorf("dockerhub list images: %w", err)
	}

	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/tags/list", fullRepo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dockerhub list images request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dockerhub list images: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("dockerhub list images decode: %w", err)
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

// SupportsNativeProtection — Docker Hub has immutable tags but not via standard registry API.
func (a *DockerHubAdapter) SupportsNativeProtection() bool { return false }

// SearchImages searches Docker Hub for repositories matching the query.
func (a *DockerHubAdapter) SearchImages(query string) ([]SearchResult, error) {
	url := fmt.Sprintf("https://hub.docker.com/v2/search/repositories/?query=%s&page_size=25", query)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dockerhub search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dockerhub search: HTTP %d", resp.StatusCode)
	}

	var data struct {
		Results []struct {
			RepoName         string `json:"repo_name"`
			ShortDescription string `json:"short_description"`
			StarCount        int    `json:"star_count"`
			IsOfficial       bool   `json:"is_official"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("dockerhub search decode: %w", err)
	}

	out := make([]SearchResult, 0, len(data.Results))
	for _, r := range data.Results {
		out = append(out, SearchResult{
			Name:        r.RepoName,
			Description: r.ShortDescription,
			StarCount:   r.StarCount,
			IsOfficial:  r.IsOfficial,
		})
	}
	return out, nil
}

// PullManifest fetches the full manifest for a given repo:tag.
// If the image is a manifest list (multi-arch), it resolves to the linux/amd64 manifest.
func (a *DockerHubAdapter) PullManifest(repo, tag string) (*Manifest, error) {
	fullRepo := a.fullRepo(repo)
	token, err := a.getToken(fullRepo, "pull")
	if err != nil {
		return nil, fmt.Errorf("dockerhub pull manifest auth: %w", err)
	}

	m, err := a.fetchManifest(fullRepo, tag, token)
	if err != nil {
		return nil, err
	}

	// Check if this is a manifest list / OCI image index — resolve to linux/amd64
	if m.MediaType == "application/vnd.docker.distribution.manifest.list.v2+json" ||
		m.MediaType == "application/vnd.oci.image.index.v1+json" {
		var index struct {
			Manifests []struct {
				MediaType string `json:"mediaType"`
				Digest    string `json:"digest"`
				Platform  struct {
					Architecture string `json:"architecture"`
					OS           string `json:"os"`
				} `json:"platform"`
			} `json:"manifests"`
		}
		if err := json.Unmarshal(m.Body, &index); err != nil {
			return nil, fmt.Errorf("dockerhub parse manifest list: %w", err)
		}

		// Find linux/amd64 manifest
		resolved := false
		for _, entry := range index.Manifests {
			if entry.Platform.OS == "linux" && entry.Platform.Architecture == "amd64" {
				m, err = a.fetchManifest(fullRepo, entry.Digest, token)
				if err != nil {
					return nil, fmt.Errorf("dockerhub resolve platform manifest: %w", err)
				}
				resolved = true
				break
			}
		}
		if !resolved {
			return nil, fmt.Errorf("dockerhub: no linux/amd64 manifest found in manifest list")
		}
	}

	return m, nil
}

// fetchManifest fetches a manifest by tag or digest.
func (a *DockerHubAdapter) fetchManifest(fullRepo, ref, token string) (*Manifest, error) {
	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/manifests/%s", fullRepo, ref)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", "))

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dockerhub fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dockerhub fetch manifest: HTTP %d for %s", resp.StatusCode, ref)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("dockerhub fetch manifest read: %w", err)
	}

	return &Manifest{
		MediaType: resp.Header.Get("Content-Type"),
		Body:      body,
		Digest:    resp.Header.Get("Docker-Content-Digest"),
	}, nil
}

// PullBlob downloads a blob from Docker Hub. Caller must close the returned ReadCloser.
func (a *DockerHubAdapter) PullBlob(repo, digest string) (io.ReadCloser, int64, error) {
	fullRepo := a.fullRepo(repo)
	token, err := a.getToken(fullRepo, "pull")
	if err != nil {
		return nil, 0, fmt.Errorf("dockerhub pull blob auth: %w", err)
	}

	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/blobs/%s", fullRepo, digest)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	// Use a longer timeout for blob downloads
	blobClient := &http.Client{Timeout: 10 * time.Minute}
	resp, err := blobClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("dockerhub pull blob: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("dockerhub pull blob: HTTP %d for %s", resp.StatusCode, digest)
	}

	return resp.Body, resp.ContentLength, nil
}

// ParseManifestDescriptors extracts config and layer descriptors from a manifest.
func ParseManifestDescriptors(m *Manifest) (config BlobDescriptor, layers []BlobDescriptor, err error) {
	var parsed struct {
		Config struct {
			MediaType string `json:"mediaType"`
			Digest    string `json:"digest"`
			Size      int64  `json:"size"`
		} `json:"config"`
		Layers []struct {
			MediaType string `json:"mediaType"`
			Digest    string `json:"digest"`
			Size      int64  `json:"size"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(m.Body, &parsed); err != nil {
		return BlobDescriptor{}, nil, fmt.Errorf("parse manifest: %w", err)
	}

	config = BlobDescriptor{
		MediaType: parsed.Config.MediaType,
		Digest:    parsed.Config.Digest,
		Size:      parsed.Config.Size,
	}

	layers = make([]BlobDescriptor, 0, len(parsed.Layers))
	for _, l := range parsed.Layers {
		layers = append(layers, BlobDescriptor{
			MediaType: l.MediaType,
			Digest:    l.Digest,
			Size:      l.Size,
		})
	}
	return config, layers, nil
}

