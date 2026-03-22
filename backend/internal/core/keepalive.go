package core

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/regikeep/rgk/internal/registry"
	"github.com/regikeep/rgk/internal/store"
)

// KeepaliveService runs keepalive operations for tracked images.
type KeepaliveService struct {
	db      *store.DB
	reg     *registry.Manager
	log     zerolog.Logger
}

// NewKeepaliveService creates a KeepaliveService.
func NewKeepaliveService(db *store.DB, reg *registry.Manager, log zerolog.Logger) *KeepaliveService {
	return &KeepaliveService{db: db, reg: reg, log: log}
}

// KeepaliveRequest specifies which images to process.
type KeepaliveRequest struct {
	ImageIDs []string
	Group    string
	Strategy string // "pull" | "retag" | "native"
}

// KeepaliveResult holds the outcome of a single image keepalive.
type KeepaliveItemResult struct {
	ImageID   string `json:"imageId"`
	Repo      string `json:"repo"`
	Tag       string `json:"tag"`
	Strategy  string `json:"strategy"`
	Success   bool   `json:"success"`
	NewExpiry string `json:"newExpiry"`
	Error     string `json:"error,omitempty"`
}

// KeepaliveResponse is the full result set returned to the API.
type KeepaliveResponse struct {
	Timestamp string                `json:"timestamp"`
	Strategy  string                `json:"strategy"`
	Processed int                   `json:"processed"`
	Results   []KeepaliveItemResult `json:"results"`
}

// Run executes keepalive for the given request.
func (s *KeepaliveService) Run(req KeepaliveRequest) (*KeepaliveResponse, error) {
	strategy := registry.Strategy(req.Strategy)
	if strategy == "" {
		strategy = registry.StrategyPull
	}

	images, err := s.resolveImages(req)
	if err != nil {
		return nil, err
	}

	resp := &KeepaliveResponse{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Strategy:  string(strategy),
		Processed: len(images),
		Results:   make([]KeepaliveItemResult, 0, len(images)),
	}

	for _, img := range images {
		result := s.keepaliveOne(img, strategy)
		resp.Results = append(resp.Results, result)
	}

	return resp, nil
}

func (s *KeepaliveService) resolveImages(req KeepaliveRequest) ([]store.ImageRef, error) {
	if len(req.ImageIDs) > 0 {
		var out []store.ImageRef
		for _, id := range req.ImageIDs {
			img, err := s.db.GetImage(id)
			if err != nil {
				return nil, fmt.Errorf("get image %s: %w", id, err)
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
			return nil, fmt.Errorf("get group %s: %w", req.Group, err)
		}
		if grp == nil {
			return nil, fmt.Errorf("group %q not found", req.Group)
		}
		return s.db.ListImages(store.ImageFilter{GroupID: grp.ID})
	}

	return nil, fmt.Errorf("must specify imageIds or group")
}

func (s *KeepaliveService) keepaliveOne(img store.ImageRef, strategy registry.Strategy) KeepaliveItemResult {
	start := time.Now()
	item := KeepaliveItemResult{
		ImageID:  img.ID,
		Repo:     img.Repo,
		Tag:      img.Tag,
		Strategy: string(strategy),
	}

	adapter, err := s.reg.Get(img.Registry)
	if err != nil {
		item.Error = err.Error()
		s.recordResult(img.ID, "failure", time.Since(start), &item.Error)
		s.warnIfConsecutiveFailures(img.ID)
		return item
	}

	result, err := adapter.Keepalive(img, strategy)
	durationMs := time.Since(start).Milliseconds()

	s.log.Info().
		Str("image_id", img.ID).
		Str("repo", img.Repo).
		Str("tag", img.Tag).
		Str("registry", img.Registry).
		Str("strategy", string(strategy)).
		Int64("duration_ms", durationMs).
		Bool("success", result.Success).
		Err(result.Error).
		Msg("keepalive attempt")

	if err != nil || !result.Success {
		errMsg := ""
		if result.Error != nil {
			errMsg = result.Error.Error()
		} else if err != nil {
			errMsg = err.Error()
		}
		item.Error = errMsg
		s.recordResult(img.ID, "failure", time.Since(start), &errMsg)
		s.warnIfConsecutiveFailures(img.ID)
		KeepaliveFailureTotal.WithLabelValues(img.Registry, string(strategy)).Inc()
		return item
	}

	item.Success = true
	item.NewExpiry = result.NewExpiry
	s.recordResult(img.ID, "safe", time.Since(start), nil)

	// Update expires_in_days from the adapter's NewExpiry
	if result.NewExpiry != "" {
		if expT, err := time.Parse(time.RFC3339, result.NewExpiry); err == nil {
			days := int(time.Until(expT).Hours() / 24)
			if days < 0 {
				days = 0
			}
			_ = s.db.UpdateExpiresIn(img.ID, days)
		}
	}

	KeepaliveSuccessTotal.WithLabelValues(img.Registry, string(strategy)).Inc()
	KeepaliveLastSuccessTimestamp.WithLabelValues(img.ID).SetToCurrentTime()
	return item
}

func (s *KeepaliveService) recordResult(imageID, status string, duration time.Duration, errMsg *string) {
	_ = s.db.UpdateKeepaliveStatus(imageID, status, errMsg)
	_ = s.db.WriteKeepaliveLog(imageID, status, duration.Milliseconds(), errMsg)
}

func (s *KeepaliveService) warnIfConsecutiveFailures(imageID string) {
	n, err := s.db.CountRecentFailures(imageID, 3)
	if err != nil {
		return
	}
	if n >= 3 {
		s.log.Warn().
			Str("image_id", imageID).
			Int("consecutive_failures", n).
			Msg("ALERT: 3 consecutive keepalive failures — image may be at risk")
	}
}
