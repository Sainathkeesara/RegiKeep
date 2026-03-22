package core

import (
	"fmt"

	"github.com/regikeep/rgk/internal/store"
)

// GroupService manages groups and their associated images.
type GroupService struct {
	db *store.DB
}

// NewGroupService creates a GroupService.
func NewGroupService(db *store.DB) *GroupService {
	return &GroupService{db: db}
}

// GroupWithStats augments a Group with image count and health info.
type GroupWithStats struct {
	store.Group
	ImageCount   int     `json:"imageCount"`
	HealthPct    float64 `json:"healthPct"` // % of images with "safe" status
	CriticalCount int   `json:"criticalCount"`
}

// ListGroupsWithStats returns all groups annotated with image statistics.
func (s *GroupService) ListGroupsWithStats() ([]GroupWithStats, error) {
	groups, err := s.db.ListGroups()
	if err != nil {
		return nil, err
	}

	out := make([]GroupWithStats, 0, len(groups))
	for _, g := range groups {
		imgs, err := s.db.ListImages(store.ImageFilter{GroupID: g.ID})
		if err != nil {
			return nil, fmt.Errorf("list images for group %s: %w", g.ID, err)
		}
		safe := 0
		critical := 0
		for _, img := range imgs {
			switch img.LastStatus {
			case "safe":
				safe++
			case "critical":
				critical++
			}
		}
		health := 0.0
		if len(imgs) > 0 {
			health = float64(safe) / float64(len(imgs)) * 100
		}
		out = append(out, GroupWithStats{
			Group:        g,
			ImageCount:   len(imgs),
			HealthPct:    health,
			CriticalCount: critical,
		})
	}
	return out, nil
}
