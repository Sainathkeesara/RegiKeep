package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/regikeep/rgk/internal/store"
)

// Export serves GET /api/v1/export?format=oracle-json|ecr-json|csv
func (h *Handlers) Export(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "oracle-json"
	}

	images, err := h.db.ListImages(store.ImageFilter{})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	switch format {
	case "oracle-json", "ocir-json":
		h.exportOCIRJSON(w, images, "ocir-fra")
	case "ecr-json":
		h.exportOCIRJSON(w, images, "ecr-use1") // same shape, different registry ID
	case "csv":
		h.exportCSV(w, images)
	default:
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("unknown format %q; use oracle-json, ecr-json, or csv", format))
	}
}

func (h *Handlers) exportOCIRJSON(w http.ResponseWriter, images []store.ImageRef, registryID string) {
	type imageEntry struct {
		Repo         string `json:"repo"`
		Tag          string `json:"tag"`
		Digest       string `json:"digest"`
		Pinned       bool   `json:"pinned"`
		Group        string `json:"group"`
		ExpiresInDays int   `json:"expires_in_days"`
		LastKeepalive string `json:"last_keepalive"`
	}

	// Build group map
	groups, _ := h.db.ListGroups()
	groupMap := make(map[string]string)
	for _, g := range groups {
		groupMap[g.ID] = g.Name
	}

	entries := make([]imageEntry, 0, len(images))
	for _, img := range images {
		// Only include pinned images or those in a group for OCIR export
		if !img.Pinned && img.GroupID == nil {
			continue
		}
		groupName := ""
		if img.GroupID != nil {
			groupName = groupMap[*img.GroupID]
		}
		lastKeepalive := ""
		if img.LastKeepaliveAt != nil {
			lastKeepalive = img.LastKeepaliveAt.Format(time.RFC3339)
		}
		entries = append(entries, imageEntry{
			Repo:          img.Repo,
			Tag:           img.Tag,
			Digest:        img.Digest,
			Pinned:        img.Pinned,
			Group:         groupName,
			ExpiresInDays: img.ExpiresInDays,
			LastKeepalive: lastKeepalive,
		})
	}

	payload := map[string]any{
		"version":     "rgk/v1",
		"registry":    registryID,
		"exported_at": time.Now().UTC().Format(time.RFC3339),
		"images":      entries,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func (h *Handlers) exportCSV(w http.ResponseWriter, images []store.ImageRef) {
	// Build group map
	groups, _ := h.db.ListGroups()
	groupMap := make(map[string]string)
	for _, g := range groups {
		groupMap[g.ID] = g.Name
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=regikeep-export.csv")
	w.WriteHeader(http.StatusOK)

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"repo", "tag", "digest", "pinned", "group", "expires_in_days", "last_keepalive"})

	for _, img := range images {
		groupName := ""
		if img.GroupID != nil {
			groupName = groupMap[*img.GroupID]
		}
		lastKeepalive := ""
		if img.LastKeepaliveAt != nil {
			lastKeepalive = img.LastKeepaliveAt.Format(time.RFC3339)
		}
		_ = cw.Write([]string{
			img.Repo,
			img.Tag,
			img.Digest,
			fmt.Sprintf("%v", img.Pinned),
			groupName,
			fmt.Sprintf("%d", img.ExpiresInDays),
			lastKeepalive,
		})
	}
	cw.Flush()
}
