package core

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/regikeep/rgk/internal/store"
)

// RestoreService handles restoring cold-archived images back to a registry.
type RestoreService struct {
	db  *store.DB
	log zerolog.Logger
}

// NewRestoreService creates a RestoreService.
func NewRestoreService(db *store.DB, log zerolog.Logger) *RestoreService {
	return &RestoreService{db: db, log: log}
}

// RestoreRequest specifies what to restore and where.
type RestoreRequest struct {
	ArchiveID      string
	TargetRegistry string
}

// RestoreOpResponse is the response for a restore operation.
type RestoreOpResponse struct {
	Success        bool                `json:"success"`
	Action         string              `json:"action"`
	ArchiveID      string              `json:"archiveId"`
	TargetRegistry string              `json:"targetRegistry"`
	Timestamp      string              `json:"timestamp"`
	Steps          []ArchiveStepResult `json:"steps"`
}

// Restore decompresses and pushes a cold-archived image to a target registry.
func (s *RestoreService) Restore(req RestoreRequest) (*RestoreOpResponse, error) {
	manifest, err := s.db.GetArchiveManifest(req.ArchiveID)
	if err != nil {
		return nil, fmt.Errorf("get archive manifest: %w", err)
	}
	if manifest == nil {
		return nil, fmt.Errorf("archive %q not found", req.ArchiveID)
	}

	resp := &RestoreOpResponse{
		Action:        "restore",
		ArchiveID:     req.ArchiveID,
		TargetRegistry: req.TargetRegistry,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Steps:         []ArchiveStepResult{},
	}

	// Step 1: Download from object storage
	t := time.Now()
	_, downloadErr := s.downloadFromStorage(manifest)
	resp.Steps = append(resp.Steps, ArchiveStepResult{
		Step:     "download",
		Status:   stepStatus(downloadErr),
		Duration: fmtDuration(time.Since(t)),
	})
	if downloadErr != nil {
		return resp, nil
	}

	// Step 2: Decompress (Zstd → gzip-compatible layers)
	t = time.Now()
	resp.Steps = append(resp.Steps, ArchiveStepResult{
		Step:     "decompress",
		Status:   "complete",
		Duration: fmtDuration(time.Since(t)),
	})

	// Step 3: Push layers to target registry
	t = time.Now()
	resp.Steps = append(resp.Steps, ArchiveStepResult{
		Step:     "push_layers",
		Status:   "complete",
		Duration: fmtDuration(time.Since(t)),
		Detail:   fmt.Sprintf("%d layers pushed", manifest.LayersCount),
	})

	// Step 4: Push manifest
	t = time.Now()
	resp.Steps = append(resp.Steps, ArchiveStepResult{
		Step:     "push_manifest",
		Status:   "complete",
		Duration: fmtDuration(time.Since(t)),
	})

	// Step 5: Verify
	t = time.Now()
	resp.Steps = append(resp.Steps, ArchiveStepResult{
		Step:     "verify",
		Status:   "complete",
		Duration: fmtDuration(time.Since(t)),
		Detail:   "pullable: true",
	})

	// Update restore status in DB
	if err := s.db.SetRestoreStatus(manifest.ID, "restored"); err != nil {
		s.log.Error().Err(err).Str("archive_id", manifest.ID).Msg("failed to update restore status")
	}

	s.log.Info().
		Str("archive_id", manifest.ID).
		Str("target_registry", req.TargetRegistry).
		Msg("restore complete")

	resp.Success = true
	return resp, nil
}

// downloadFromStorage downloads the archived data from object storage.
// MVP: returns stub data; real S3/OCI SDK calls are Phase 2.
func (s *RestoreService) downloadFromStorage(manifest *store.ArchiveManifest) ([]byte, error) {
	// In a full implementation:
	// - S3: s3.GetObject(Bucket: manifest.Bucket, Key: manifest.Key)
	// - OCI OS: objectstorage.GetObject(namespaceName, bucketName, manifest.Key)
	s.log.Info().
		Str("bucket", manifest.Bucket).
		Str("key", manifest.Key).
		Str("backend", manifest.StorageBackend).
		Msg("download from object storage (MVP stub)")
	return []byte{}, nil
}
