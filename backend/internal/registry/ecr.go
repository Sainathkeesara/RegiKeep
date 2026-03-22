package registry

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/regikeep/rgk/internal/store"
)

// ECRAdapter implements the Adapter interface for Amazon Elastic Container Registry.
type ECRAdapter struct {
	id          string
	accountID   string
	region      string
	endpoint    string // e.g. 123456789.dkr.ecr.us-east-1.amazonaws.com
	accessKeyID string
	secretKey   string
	httpClient  *http.Client

	// cached registry auth (base64 "AWS:<password>", expires after 12h)
	registryToken   string
	tokenExpiration time.Time
}

// NewECRAdapter creates an ECR adapter.
func NewECRAdapter(registryID, accountID, region string) *ECRAdapter {
	endpoint := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", accountID, region)
	return &ECRAdapter{
		id:         registryID,
		accountID:  accountID,
		region:     region,
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewECRAdapterWithCreds creates an ECR adapter with credentials.
func NewECRAdapterWithCreds(registryID, accountID, region, accessKeyID, secretKey string) *ECRAdapter {
	endpoint := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", accountID, region)
	return &ECRAdapter{
		id:          registryID,
		accountID:   accountID,
		region:      region,
		endpoint:    endpoint,
		accessKeyID: accessKeyID,
		secretKey:   secretKey,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *ECRAdapter) ID() string { return a.id }

// Endpoint returns the ECR registry endpoint (e.g. 123456789.dkr.ecr.us-east-1.amazonaws.com).
func (a *ECRAdapter) Endpoint() string { return a.endpoint }

// Authenticate obtains a Docker registry auth token from ECR via GetAuthorizationToken.
func (a *ECRAdapter) Authenticate() error {
	if a.accountID == "" || a.region == "" {
		return fmt.Errorf("ecr auth: accountID and region are required")
	}
	if a.accessKeyID == "" || a.secretKey == "" {
		return fmt.Errorf("ecr auth: AWS credentials (accessKeyID, secretKey) are required")
	}

	token, expiry, err := a.getAuthorizationToken()
	if err != nil {
		return fmt.Errorf("ecr auth: %w", err)
	}
	a.registryToken = token
	a.tokenExpiration = expiry
	return nil
}

// ensureToken refreshes the ECR registry token if expired or missing.
func (a *ECRAdapter) ensureToken() error {
	if a.registryToken != "" && time.Now().Before(a.tokenExpiration.Add(-5*time.Minute)) {
		return nil
	}
	return a.Authenticate()
}

// getAuthorizationToken calls the ECR GetAuthorizationToken API using AWS Sig V4.
// Returns the base64-decoded password and expiration time.
func (a *ECRAdapter) getAuthorizationToken() (string, time.Time, error) {
	// ECR API endpoint for private ECR (not ECR Public)
	ecrAPI := fmt.Sprintf("https://api.ecr.%s.amazonaws.com", a.region)
	body := "{}"
	now := time.Now().UTC()

	req, err := http.NewRequest(http.MethodPost, ecrAPI, strings.NewReader(body))
	if err != nil {
		return "", time.Time{}, err
	}

	// Required headers for ECR GetAuthorizationToken
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AmazonEC2ContainerRegistry_V20150921.GetAuthorizationToken")
	req.Header.Set("Host", fmt.Sprintf("api.ecr.%s.amazonaws.com", a.region))
	req.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))

	// Sign the request with AWS Signature V4
	a.signV4(req, []byte(body), now)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "no such host") {
			return "", time.Time{}, fmt.Errorf("endpoint api.ecr.%s.amazonaws.com not reachable — check region", a.region)
		}
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		AuthorizationData []struct {
			AuthorizationToken string `json:"authorizationToken"`
			ExpiresAt          float64 `json:"expiresAt"`
			ProxyEndpoint      string `json:"proxyEndpoint"`
		} `json:"authorizationData"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", time.Time{}, fmt.Errorf("decode response: %w", err)
	}
	if len(result.AuthorizationData) == 0 {
		return "", time.Time{}, fmt.Errorf("no authorization data returned")
	}

	// Token is base64("AWS:<password>") — decode to get the password
	decoded, err := base64.StdEncoding.DecodeString(result.AuthorizationData[0].AuthorizationToken)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("decode token: %w", err)
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", time.Time{}, fmt.Errorf("unexpected token format")
	}
	password := parts[1]

	expiry := time.Unix(int64(result.AuthorizationData[0].ExpiresAt), 0)
	return password, expiry, nil
}

// signV4 signs an HTTP request using AWS Signature Version 4.
func (a *ECRAdapter) signV4(req *http.Request, payload []byte, now time.Time) {
	datestamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	service := "ecr"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", datestamp, a.region, service)

	// Canonical headers (must be sorted)
	host := req.Header.Get("Host")
	contentType := req.Header.Get("Content-Type")
	xAmzTarget := req.Header.Get("X-Amz-Target")
	signedHeaders := "content-type;host;x-amz-date;x-amz-target"
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-date:%s\nx-amz-target:%s\n",
		contentType, host, amzDate, xAmzTarget)

	// Hash the payload
	payloadHash := sha256Hex(payload)

	// Canonical request
	canonicalRequest := fmt.Sprintf("%s\n/\n\n%s\n%s\n%s",
		http.MethodPost, canonicalHeaders, signedHeaders, payloadHash)

	// String to sign
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, credentialScope, sha256Hex([]byte(canonicalRequest)))

	// Signing key
	signingKey := hmacSHA256(
		hmacSHA256(
			hmacSHA256(
				hmacSHA256([]byte("AWS4"+a.secretKey), []byte(datestamp)),
				[]byte(a.region)),
			[]byte(service)),
		[]byte("aws4_request"))

	// Signature
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// Authorization header
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		a.accessKeyID, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

// basicAuth returns the Basic auth header value for ECR registry requests.
func (a *ECRAdapter) basicAuth() string {
	return base64.StdEncoding.EncodeToString([]byte("AWS:" + a.registryToken))
}

// BasicAuth returns the Basic auth header value (exported for archive service).
func (a *ECRAdapter) BasicAuth() string {
	return a.basicAuth()
}

// ResolveDigest fetches the manifest digest for the given repo:tag.
func (a *ECRAdapter) ResolveDigest(repo, tag string) (string, error) {
	if err := a.ensureToken(); err != nil {
		return "", fmt.Errorf("ecr resolve digest: %w", err)
	}

	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", a.endpoint, repo, tag)
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Basic "+a.basicAuth())
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ecr resolve digest request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ecr resolve digest: HTTP %d for %s:%s", resp.StatusCode, repo, tag)
	}

	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", fmt.Errorf("ecr resolve digest: empty Docker-Content-Digest header")
	}
	return digest, nil
}

// Keepalive performs a keepalive action for the given image.
func (a *ECRAdapter) Keepalive(img store.ImageRef, strategy Strategy) (KeepaliveResult, error) {
	switch strategy {
	case StrategyPull:
		return a.keepalivePull(img)
	case StrategyRetag:
		return a.keepaliveRetag(img)
	default:
		return a.keepalivePull(img)
	}
}

func (a *ECRAdapter) keepalivePull(img store.ImageRef) (KeepaliveResult, error) {
	// Use ECR BatchGetImage API — this is the only method ECR counts as a real "pull"
	// that updates the lastRecordedPullTime. The Docker v2 registry API (manifest/blob
	// GET) does NOT update the pull timestamp.
	tag := img.Tag
	if tag == "" {
		return KeepaliveResult{Error: fmt.Errorf("ecr keepalive: image tag is required")}, nil
	}

	body := fmt.Sprintf(`{"repositoryName":"%s","imageIds":[{"imageTag":"%s"}],"acceptedMediaTypes":["application/vnd.docker.distribution.manifest.v2+json","application/vnd.oci.image.manifest.v1+json"]}`, img.Repo, tag)
	now := time.Now().UTC()

	ecrAPI := fmt.Sprintf("https://api.ecr.%s.amazonaws.com", a.region)
	req, err := http.NewRequest(http.MethodPost, ecrAPI, strings.NewReader(body))
	if err != nil {
		return KeepaliveResult{}, err
	}

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AmazonEC2ContainerRegistry_V20150921.BatchGetImage")
	req.Header.Set("Host", fmt.Sprintf("api.ecr.%s.amazonaws.com", a.region))
	req.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))

	a.signV4(req, []byte(body), now)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return KeepaliveResult{Error: fmt.Errorf("ecr keepalive BatchGetImage: %w", err)}, nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return KeepaliveResult{Error: fmt.Errorf("ecr keepalive read response: %w", err)}, nil
	}

	if resp.StatusCode != http.StatusOK {
		e := fmt.Errorf("ecr keepalive BatchGetImage: HTTP %d: %s", resp.StatusCode, string(respBody))
		return KeepaliveResult{Error: e}, nil
	}

	// Verify we got image data back
	var result struct {
		Images   []json.RawMessage `json:"images"`
		Failures []struct {
			FailureCode   string `json:"failureCode"`
			FailureReason string `json:"failureReason"`
		} `json:"failures"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return KeepaliveResult{Error: fmt.Errorf("ecr keepalive decode: %w", err)}, nil
	}
	if len(result.Failures) > 0 {
		e := fmt.Errorf("ecr keepalive BatchGetImage failed: %s — %s", result.Failures[0].FailureCode, result.Failures[0].FailureReason)
		return KeepaliveResult{Error: e}, nil
	}
	if len(result.Images) == 0 {
		return KeepaliveResult{Error: fmt.Errorf("ecr keepalive: no images returned")}, nil
	}

	// ECR default lifecycle: images without a pull in 14+ days may be expired
	newExpiry := time.Now().UTC().Add(14 * 24 * time.Hour).Format(time.RFC3339)
	return KeepaliveResult{Success: true, NewExpiry: newExpiry}, nil
}

func (a *ECRAdapter) keepaliveRetag(img store.ImageRef) (KeepaliveResult, error) {
	if err := a.ensureToken(); err != nil {
		return KeepaliveResult{Error: fmt.Errorf("ecr keepalive auth: %w", err)}, nil
	}

	ref := img.Digest
	if ref == "" {
		ref = img.Tag
	}

	// GET manifest
	getURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", a.endpoint, img.Repo, ref)
	getReq, err := http.NewRequest(http.MethodGet, getURL, nil)
	if err != nil {
		return KeepaliveResult{}, err
	}
	getReq.Header.Set("Authorization", "Basic "+a.basicAuth())
	getReq.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json")

	getResp, err := a.httpClient.Do(getReq)
	if err != nil {
		return KeepaliveResult{Error: err}, nil
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		e := fmt.Errorf("ecr keepalive retag GET: HTTP %d", getResp.StatusCode)
		return KeepaliveResult{Error: e}, nil
	}

	manifest, err := io.ReadAll(getResp.Body)
	if err != nil {
		return KeepaliveResult{Error: fmt.Errorf("ecr keepalive retag read: %w", err)}, nil
	}
	contentType := getResp.Header.Get("Content-Type")

	// PUT manifest back with same tag
	putURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", a.endpoint, img.Repo, img.Tag)
	putReq, err := http.NewRequest(http.MethodPut, putURL, strings.NewReader(string(manifest)))
	if err != nil {
		return KeepaliveResult{}, err
	}
	putReq.Header.Set("Authorization", "Basic "+a.basicAuth())
	putReq.Header.Set("Content-Type", contentType)

	putResp, err := a.httpClient.Do(putReq)
	if err != nil {
		return KeepaliveResult{Error: err}, nil
	}
	defer putResp.Body.Close()
	io.Copy(io.Discard, putResp.Body)

	if putResp.StatusCode != http.StatusCreated && putResp.StatusCode != http.StatusOK {
		e := fmt.Errorf("ecr keepalive retag PUT: HTTP %d", putResp.StatusCode)
		return KeepaliveResult{Error: e}, nil
	}

	newExpiry := time.Now().UTC().Add(14 * 24 * time.Hour).Format(time.RFC3339)
	return KeepaliveResult{Success: true, NewExpiry: newExpiry}, nil
}

// ApplyNativeProtection — ECR supports lifecycle policy exclusions (Phase 2).
func (a *ECRAdapter) ApplyNativeProtection(img store.ImageRef) error {
	return ErrNotImplemented{Method: "ApplyNativeProtection", Registry: "ecr"}
}

// ListImages lists tags in a repository.
func (a *ECRAdapter) ListImages(repo string) ([]store.ImageRef, error) {
	if err := a.ensureToken(); err != nil {
		return nil, fmt.Errorf("ecr list images: %w", err)
	}

	url := fmt.Sprintf("https://%s/v2/%s/tags/list", a.endpoint, repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic "+a.basicAuth())

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ecr list images request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ecr list images: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ecr list images decode: %w", err)
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

func (a *ECRAdapter) SupportsNativeProtection() bool { return false }

// CreateRepository creates a repository in ECR. Returns nil if it already exists.
func (a *ECRAdapter) CreateRepository(repoName string) error {
	body := fmt.Sprintf(`{"repositoryName":"%s"}`, repoName)
	now := time.Now().UTC()

	ecrAPI := fmt.Sprintf("https://api.ecr.%s.amazonaws.com", a.region)
	req, err := http.NewRequest(http.MethodPost, ecrAPI, strings.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AmazonEC2ContainerRegistry_V20150921.CreateRepository")
	req.Header.Set("Host", fmt.Sprintf("api.ecr.%s.amazonaws.com", a.region))
	req.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))

	a.signV4(req, []byte(body), now)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ecr create repository: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Check if repository already exists (not an error)
	if strings.Contains(string(respBody), "RepositoryAlreadyExistsException") {
		return nil
	}

	return fmt.Errorf("ecr create repository: HTTP %d: %s", resp.StatusCode, string(respBody))
}

// CheckBlobExists checks if a blob already exists in ECR.
func (a *ECRAdapter) CheckBlobExists(repo, digest string) (bool, error) {
	if err := a.ensureToken(); err != nil {
		return false, err
	}

	url := fmt.Sprintf("https://%s/v2/%s/blobs/%s", a.endpoint, repo, digest)
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Basic "+a.basicAuth())

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("ecr check blob: HTTP %d", resp.StatusCode)
}

// PushBlob uploads a blob to ECR using the OCI distribution two-step upload.
func (a *ECRAdapter) PushBlob(repo, digest string, size int64, reader io.Reader) error {
	if err := a.ensureToken(); err != nil {
		return fmt.Errorf("ecr push blob auth: %w", err)
	}

	// Step 1: Initiate upload
	initURL := fmt.Sprintf("https://%s/v2/%s/blobs/uploads/", a.endpoint, repo)
	initReq, err := http.NewRequest(http.MethodPost, initURL, nil)
	if err != nil {
		return err
	}
	initReq.Header.Set("Authorization", "Basic "+a.basicAuth())

	initResp, err := a.httpClient.Do(initReq)
	if err != nil {
		return fmt.Errorf("ecr push blob initiate: %w", err)
	}
	defer initResp.Body.Close()
	io.Copy(io.Discard, initResp.Body)

	if initResp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("ecr push blob initiate: HTTP %d", initResp.StatusCode)
	}

	location := initResp.Header.Get("Location")
	if location == "" {
		return fmt.Errorf("ecr push blob: no Location header in upload initiation response")
	}

	// Make location absolute if relative
	if strings.HasPrefix(location, "/") {
		location = fmt.Sprintf("https://%s%s", a.endpoint, location)
	}

	// Step 2: Complete upload
	sep := "?"
	if strings.Contains(location, "?") {
		sep = "&"
	}
	putURL := fmt.Sprintf("%s%sdigest=%s", location, sep, digest)
	putReq, err := http.NewRequest(http.MethodPut, putURL, reader)
	if err != nil {
		return err
	}
	putReq.Header.Set("Authorization", "Basic "+a.basicAuth())
	putReq.Header.Set("Content-Type", "application/octet-stream")
	if size > 0 {
		putReq.ContentLength = size
	}

	blobClient := &http.Client{Timeout: 10 * time.Minute}
	putResp, err := blobClient.Do(putReq)
	if err != nil {
		return fmt.Errorf("ecr push blob upload: %w", err)
	}
	defer putResp.Body.Close()
	io.Copy(io.Discard, putResp.Body)

	if putResp.StatusCode != http.StatusCreated {
		return fmt.Errorf("ecr push blob complete: HTTP %d", putResp.StatusCode)
	}

	return nil
}

// PushManifest uploads a manifest to ECR and returns the digest.
func (a *ECRAdapter) PushManifest(repo, tag string, manifest *Manifest) (string, error) {
	if err := a.ensureToken(); err != nil {
		return "", fmt.Errorf("ecr push manifest auth: %w", err)
	}

	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", a.endpoint, repo, tag)
	req, err := http.NewRequest(http.MethodPut, url, strings.NewReader(string(manifest.Body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Basic "+a.basicAuth())
	req.Header.Set("Content-Type", manifest.MediaType)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ecr push manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ecr push manifest: HTTP %d: %s", resp.StatusCode, string(body))
	}

	digest := resp.Header.Get("Docker-Content-Digest")
	return digest, nil
}

// --- helpers ---

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
