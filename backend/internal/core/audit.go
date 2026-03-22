package core

import (
	"time"

	"github.com/regikeep/rgk/internal/store"
)

// AuditService performs dry-run retention risk analysis.
type AuditService struct {
	db *store.DB
}

// NewAuditService creates an AuditService.
func NewAuditService(db *store.DB) *AuditService {
	return &AuditService{db: db}
}

// AuditRequest specifies audit parameters.
type AuditRequest struct {
	DryRun         bool
	RegistryFilter string
}

// AuditResult describes a single at-risk image.
type AuditResult struct {
	ImageID        string `json:"imageId"`
	Repo           string `json:"repo"`
	Tag            string `json:"tag"`
	Region         string `json:"region"`
	ExpiresIn      int    `json:"expiresIn"`
	Status         string `json:"status"`
	Risk           string `json:"risk"` // "critical" | "warning" | "unpinned"
	Recommendation string `json:"recommendation"`
}

// AuditSummary holds aggregate counts.
type AuditSummary struct {
	TotalScanned int `json:"totalScanned"`
	AtRisk       int `json:"atRisk"`
	Critical     int `json:"critical"`
	Warning      int `json:"warning"`
	Unpinned     int `json:"unpinned"`
}

// AuditResponse is the full audit result.
type AuditResponse struct {
	DryRun    bool         `json:"dryRun"`
	Timestamp string       `json:"timestamp"`
	Summary   AuditSummary `json:"summary"`
	Results   []AuditResult `json:"results"`
}

// Run executes the audit and returns at-risk images.
func (s *AuditService) Run(req AuditRequest) (*AuditResponse, error) {
	filter := store.ImageFilter{}
	if req.RegistryFilter != "" && req.RegistryFilter != "all" {
		filter.Registry = req.RegistryFilter
	}

	images, err := s.db.ListImages(filter)
	if err != nil {
		return nil, err
	}

	// Load registry region map for the response.
	registries, _ := s.db.ListRegistries()
	regionMap := make(map[string]string)
	for _, r := range registries {
		regionMap[r.ID] = r.Region
	}

	resp := &AuditResponse{
		DryRun:    req.DryRun,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Results:   []AuditResult{},
	}
	resp.Summary.TotalScanned = len(images)

	for _, img := range images {
		risk, rec := assessRisk(img)
		if risk == "" {
			continue // image is safe
		}

		resp.Summary.AtRisk++
		switch risk {
		case "critical":
			resp.Summary.Critical++
		case "warning":
			resp.Summary.Warning++
		case "unpinned":
			resp.Summary.Unpinned++
		}

		region := regionMap[img.Registry]
		resp.Results = append(resp.Results, AuditResult{
			ImageID:        img.ID,
			Repo:           img.Repo,
			Tag:            img.Tag,
			Region:         region,
			ExpiresIn:      img.ExpiresInDays,
			Status:         img.LastStatus,
			Risk:           risk,
			Recommendation: rec,
		})
	}

	return resp, nil
}

// assessRisk returns the risk level and recommendation for an image.
// Returns ("", "") if the image is safe.
func assessRisk(img store.ImageRef) (risk, recommendation string) {
	if !img.Pinned && img.LastStatus == "unpinned" {
		return "unpinned", "Register this image into a group to enable keepalive protection"
	}
	if img.ExpiresInDays == -1 {
		return "", "" // unknown expiry — assume safe for now
	}
	if img.ExpiresInDays <= 2 {
		return "critical", "Pin immediately or archive to cold storage"
	}
	if img.ExpiresInDays <= 7 {
		return "warning", "Schedule keepalive or archive within the next few days"
	}
	return "", ""
}
