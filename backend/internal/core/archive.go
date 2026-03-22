package core

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/rs/zerolog"
	"github.com/regikeep/rgk/internal/config"
	"github.com/regikeep/rgk/internal/registry"
	"github.com/regikeep/rgk/internal/store"
)

// ArchiveService handles cold archival of container images.
type ArchiveService struct {
	db     *store.DB
	cfg    *config.Config
	log    zerolog.Logger
	regMgr *registry.Manager
}

// NewArchiveService creates an ArchiveService.
func NewArchiveService(db *store.DB, cfg *config.Config, log zerolog.Logger) *ArchiveService {
	return &ArchiveService{db: db, cfg: cfg, log: log}
}

// SetRegistryManager sets the registry manager for manifest pulling.
func (s *ArchiveService) SetRegistryManager(mgr *registry.Manager) {
	s.regMgr = mgr
}

// ArchiveRequest specifies what to archive.
type ArchiveRequest struct {
	ImageIDs      []string
	Group         string
	TargetStorage string // "s3" | "oci-os"
}

// ArchiveStepResult describes a single pipeline step.
type ArchiveStepResult struct {
	Step       string `json:"step"`
	Status     string `json:"status"`
	Duration   string `json:"duration"`
	Detail     string `json:"detail,omitempty"`
}

// ArchiveOpResponse is the response for a single archive operation.
type ArchiveOpResponse struct {
	Success       bool                `json:"success"`
	Action        string              `json:"action"`
	TargetStorage string              `json:"targetStorage"`
	Timestamp     string              `json:"timestamp"`
	Steps         []ArchiveStepResult `json:"steps"`
}

// Archive performs the archive pipeline for all resolved images.
func (s *ArchiveService) Archive(req ArchiveRequest) (*ArchiveOpResponse, error) {
	images, err := s.resolveArchiveImages(req)
	if err != nil {
		return nil, err
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("no images found for archive request")
	}

	storage := req.TargetStorage
	if storage == "" {
		storage = "s3"
		if s.cfg.ArchiveOCIBucket != "" {
			storage = "oci-os"
		}
	}

	resp := &ArchiveOpResponse{
		Action:        "archive",
		TargetStorage: storage,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}

	succeeded := 0
	failed := 0
	for _, img := range images {
		steps, manifest, err := s.runArchivePipeline(img, storage)
		if err != nil {
			s.log.Error().Err(err).Str("image", img.Repo+":"+img.Tag).Msg("archive pipeline failed")
			failed++
			resp.Steps = steps // keep last failure steps for backward compat
			continue
		}

		_, dbErr := s.db.CreateArchiveManifest(*manifest)
		if dbErr != nil {
			s.log.Error().Err(dbErr).Msg("failed to save archive manifest")
		}
		succeeded++
		resp.Steps = steps
	}

	resp.Success = failed == 0
	return resp, nil
}

func (s *ArchiveService) runArchivePipeline(img store.ImageRef, storage string) ([]ArchiveStepResult, *store.ArchiveManifest, error) {
	var steps []ArchiveStepResult

	// Step 1: Pull manifest (lightweight — no layer download in MVP)
	t := time.Now()
	manifestData, err := s.pullManifest(img)
	steps = append(steps, ArchiveStepResult{
		Step:     "pull",
		Status:   stepStatus(err),
		Duration: fmtDuration(time.Since(t)),
	})
	if err != nil {
		return steps, nil, fmt.Errorf("pull: %w", err)
	}

	// Step 2: Dedup check (content-addressed layer check)
	t = time.Now()
	layersCount, dedupedCount := s.dedupCheck(manifestData)
	steps = append(steps, ArchiveStepResult{
		Step:     "dedup",
		Status:   "complete",
		Duration: fmtDuration(time.Since(t)),
		Detail:   fmt.Sprintf("%d layers, %d deduped", layersCount, dedupedCount),
	})

	// Step 3: Compress (Zstd level 3 in MVP — level 19 in production)
	t = time.Now()
	compressed, originalSize, compressedSize, err := s.compress(manifestData)
	steps = append(steps, ArchiveStepResult{
		Step:     "compress",
		Status:   stepStatus(err),
		Duration: fmtDuration(time.Since(t)),
		Detail:   fmt.Sprintf("ratio %.0f%%", ratio(originalSize, compressedSize)),
	})
	if err != nil {
		return steps, nil, fmt.Errorf("compress: %w", err)
	}

	// Step 4: Upload to object storage
	t = time.Now()
	bucket, key, err := s.upload(img, compressed, storage)
	steps = append(steps, ArchiveStepResult{
		Step:     "upload",
		Status:   stepStatus(err),
		Duration: fmtDuration(time.Since(t)),
	})
	if err != nil {
		return steps, nil, fmt.Errorf("upload: %w", err)
	}

	// Step 5: Verify
	t = time.Now()
	err = s.verify(bucket, key, storage)
	steps = append(steps, ArchiveStepResult{
		Step:     "verify",
		Status:   stepStatus(err),
		Duration: fmtDuration(time.Since(t)),
	})
	if err != nil {
		return steps, nil, fmt.Errorf("verify: %w", err)
	}

	manifest := &store.ArchiveManifest{
		ImageRefID:      img.ID,
		Repo:            img.Repo,
		Tag:             img.Tag,
		Digest:          img.Digest,
		Bucket:          bucket,
		Key:             key,
		LayersCount:     layersCount,
		OriginalBytes:   originalSize,
		CompressedBytes: compressedSize,
		StorageBackend:  storage,
	}

	return steps, manifest, nil
}

// pullManifest fetches the image manifest using the registry adapter.
func (s *ArchiveService) pullManifest(img store.ImageRef) ([]byte, error) {
	if s.regMgr != nil && img.Registry != "" {
		adapter, err := s.regMgr.Get(img.Registry)
		if err == nil {
			if err := adapter.Authenticate(); err != nil {
				return nil, fmt.Errorf("registry %s auth: %w", img.Registry, err)
			}

			ref := img.Digest
			if ref == "" {
				ref = img.Tag
			}

			switch a := adapter.(type) {
			case *registry.ECRAdapter:
				url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", a.Endpoint(), img.Repo, ref)
				req, _ := http.NewRequest(http.MethodGet, url, nil)
				req.Header.Set("Authorization", "Basic "+a.BasicAuth())
				req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json")
				resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
				if err != nil {
					return nil, fmt.Errorf("ecr manifest: %w", err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					return nil, fmt.Errorf("ecr manifest HTTP %d: %s", resp.StatusCode, string(body))
				}
				return io.ReadAll(resp.Body)

			case *registry.DockerHubAdapter:
				m, err := a.PullManifest(img.Repo, img.Tag)
				if err != nil {
					return nil, err
				}
				return m.Body, nil
			}
		}
	}

	// Fallback: OCIR
	endpoint := s.cfg.OCIREndpoint
	if endpoint == "" {
		return nil, fmt.Errorf("no registry adapter for %s:%s (registry=%s)", img.Repo, img.Tag, img.Registry)
	}
	var basicAuth string
	if s.cfg.OCIRUsername != "" && s.cfg.OCIRAuthToken != "" {
		basicAuth = base64.StdEncoding.EncodeToString([]byte(s.cfg.OCIRUsername + ":" + s.cfg.OCIRAuthToken))
	}
	ref := img.Digest
	if ref == "" {
		ref = img.Tag
	}
	url := fmt.Sprintf("https://%s/v2/%s/%s/manifests/%s", endpoint, s.cfg.OCIRTenancy, img.Repo, ref)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if basicAuth != "" {
		req.Header.Set("Authorization", "Basic "+basicAuth)
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching manifest", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// dedupCheck parses a manifest to count layers.
// Returns total layers and how many were already in the content store (0 for MVP).
func (s *ArchiveService) dedupCheck(manifestData []byte) (total int, deduped int) {
	// Parse layer count from manifest JSON
	// A simple heuristic: count occurrences of "digest" in the manifest
	count := 0
	for i := 0; i < len(manifestData)-7; i++ {
		if string(manifestData[i:i+8]) == `"digest"` {
			count++
		}
	}
	// Subtract 1 for config digest
	if count > 1 {
		count--
	}
	return count, 0 // MVP: no cross-image dedup store yet
}

// compress compresses data with Zstd and returns (compressed, originalSize, compressedSize, error).
func (s *ArchiveService) compress(data []byte) ([]byte, int64, int64, error) {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("create zstd encoder: %w", err)
	}
	compressed := enc.EncodeAll(data, nil)
	return compressed, int64(len(data)), int64(len(compressed)), nil
}

// upload sends the compressed data to the target object storage.
func (s *ArchiveService) upload(img store.ImageRef, data []byte, storage string) (bucket, key string, err error) {
	key = fmt.Sprintf("archives/%s/%s/%s.manifest.zst", img.Registry, img.Repo, img.ID)

	switch storage {
	case "oci-os":
		bucket = s.cfg.ArchiveOCIBucket
		if bucket == "" {
			bucket = "regikeep-archive"
		}
		// OCI Object Storage: Phase 2
		return bucket, key, fmt.Errorf("OCI Object Storage upload not yet implemented")
	default:
		bucket = s.cfg.ArchiveS3Bucket
		if bucket == "" {
			return "", "", fmt.Errorf("ARCHIVE_S3_BUCKET not configured")
		}
		region := s.cfg.ArchiveS3Region
		if region == "" {
			region = s.cfg.AWSRegion
		}
		err = s.uploadToS3(bucket, key, data, region)
		return bucket, key, err
	}
}

// uploadToS3 puts an object into S3 using AWS Signature V4.
func (s *ArchiveService) uploadToS3(bucket, key string, data []byte, region string) error {
	now := time.Now().UTC()
	host := fmt.Sprintf("%s.s3.%s.amazonaws.com", bucket, region)
	url := fmt.Sprintf("https://%s/%s", host, key)

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	payloadHash := s3sha256Hex(data)
	req.Header.Set("Host", host)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.ContentLength = int64(len(data))

	s.signS3V4(req, payloadHash, region, now)

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("s3 upload: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("s3 upload: HTTP %d: %s", resp.StatusCode, string(body))
	}

	s.log.Info().Str("bucket", bucket).Str("key", key).Int("size", len(data)).Msg("uploaded to S3")
	return nil
}

// verify checks that the uploaded object is accessible via S3 HEAD.
func (s *ArchiveService) verify(bucket, key, storage string) error {
	if storage == "oci-os" {
		return nil // skip for OCI
	}

	region := s.cfg.ArchiveS3Region
	if region == "" {
		region = s.cfg.AWSRegion
	}

	now := time.Now().UTC()
	host := fmt.Sprintf("%s.s3.%s.amazonaws.com", bucket, region)
	url := fmt.Sprintf("https://%s/%s", host, key)

	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return err
	}

	payloadHash := s3sha256Hex([]byte{}) // empty payload for HEAD
	req.Header.Set("Host", host)
	req.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	s.signS3V4(req, payloadHash, region, now)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("s3 verify: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("s3 verify: HTTP %d for %s/%s", resp.StatusCode, bucket, key)
	}
	return nil
}

// signS3V4 signs an HTTP request for S3 using AWS Signature V4.
func (s *ArchiveService) signS3V4(req *http.Request, payloadHash, region string, now time.Time) {
	datestamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	service := "s3"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", datestamp, region, service)

	host := req.Header.Get("Host")
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		host, payloadHash, amzDate)

	canonicalURI := req.URL.Path
	canonicalQueryString := ""

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method, canonicalURI, canonicalQueryString, canonicalHeaders, signedHeaders, payloadHash)

	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, credentialScope, s3sha256Hex([]byte(canonicalRequest)))

	signingKey := s3hmacSHA256(
		s3hmacSHA256(
			s3hmacSHA256(
				s3hmacSHA256([]byte("AWS4"+s.cfg.AWSSecretAccessKey), []byte(datestamp)),
				[]byte(region)),
			[]byte(service)),
		[]byte("aws4_request"))

	signature := hex.EncodeToString(s3hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.cfg.AWSAccessKeyID, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

func s3sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func s3hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func (s *ArchiveService) resolveArchiveImages(req ArchiveRequest) ([]store.ImageRef, error) {
	if len(req.ImageIDs) > 0 {
		var out []store.ImageRef
		for _, id := range req.ImageIDs {
			img, err := s.db.GetImage(id)
			if err != nil {
				return nil, err
			}
			if img != nil {
				out = append(out, *img)
			}
		}
		return out, nil
	}

	if req.Group != "" {
		grp, err := s.db.GetGroupByName(req.Group)
		if err != nil {
			return nil, err
		}
		if grp == nil {
			return nil, fmt.Errorf("group %q not found", req.Group)
		}
		return s.db.ListImages(store.ImageFilter{GroupID: grp.ID})
	}

	return nil, fmt.Errorf("must specify imageIds or group")
}

func stepStatus(err error) string {
	if err != nil {
		return "failed"
	}
	return "complete"
}

func fmtDuration(d time.Duration) string {
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func ratio(original, compressed int64) float64 {
	if original == 0 {
		return 0
	}
	return (1 - float64(compressed)/float64(original)) * 100
}
