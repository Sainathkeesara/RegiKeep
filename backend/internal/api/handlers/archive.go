package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/regikeep/rgk/internal/core"
	"github.com/regikeep/rgk/internal/store"
)

// ListArchives serves GET /functions/v1/archive and GET /api/v1/archive
func (h *Handlers) ListArchives(w http.ResponseWriter, r *http.Request) {
	archives, err := h.db.ListArchives()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	stats, err := h.db.GetArchiveStats()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	views := make([]store.ArchivedImageView, 0, len(archives))
	for _, a := range archives {
		views = append(views, toArchivedImageView(a))
	}

	jsonOK(w, map[string]any{
		"archives":           views,
		"total":              len(views),
		"totalCompressedSize": formatBytes(stats.TotalCompressedBytes),
		"totalOriginalSize":   formatBytes(stats.TotalOriginalBytes),
	})
}

// ArchiveAction serves POST /functions/v1/archive
// Handles: archive, restore actions (Supabase-compatible shape)
func (h *Handlers) ArchiveAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ImageID       string `json:"imageId"`
		Action        string `json:"action"`
		TargetStorage string `json:"targetStorage"`
		TargetRegistry string `json:"targetRegistry"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.Action {
	case "archive":
		resp, err := h.archSvc.Archive(core.ArchiveRequest{
			ImageIDs:      []string{req.ImageID},
			TargetStorage: req.TargetStorage,
		})
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, resp)

	case "restore":
		// The UI sends the archive manifest ID (ArchivedImageView.id).
		// Look up by manifest ID first; fall back to image_ref_id for API clients.
		archive, err := h.db.GetArchiveManifest(req.ImageID)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if archive == nil {
			archive, err = h.db.GetArchiveByImageRef(req.ImageID)
			if err != nil {
				jsonError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		if archive == nil {
			jsonError(w, http.StatusNotFound, "no archive found for this image")
			return
		}
		resp, err := h.restSvc.Restore(core.RestoreRequest{
			ArchiveID:      archive.ID,
			TargetRegistry: req.TargetRegistry,
		})
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, map[string]any{"success": resp.Success})

	default:
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("unknown action %q", req.Action))
	}
}

// TriggerArchive serves POST /api/v1/archive
func (h *Handlers) TriggerArchive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ImageIDs      []string `json:"imageIds"`
		Group         string   `json:"group"`
		TargetStorage string   `json:"targetStorage"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.archSvc.Archive(core.ArchiveRequest{
		ImageIDs:      req.ImageIDs,
		Group:         req.Group,
		TargetStorage: req.TargetStorage,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, resp)
}

// RestoreArchive serves POST /api/v1/archive/:id/restore
func (h *Handlers) RestoreArchive(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		TargetRegistry string `json:"targetRegistry"`
	}
	_ = decode(r, &req)

	resp, err := h.restSvc.Restore(core.RestoreRequest{
		ArchiveID:      id,
		TargetRegistry: req.TargetRegistry,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, resp)
}

// DeleteArchive serves DELETE /api/v1/archive/:id
func (h *Handlers) DeleteArchive(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.DeleteArchive(id); err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	jsonNoContent(w)
}

// ArchiveStats serves GET /api/v1/archive/stats
func (h *Handlers) ArchiveStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetArchiveStats()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	savings := stats.TotalOriginalBytes - stats.TotalCompressedBytes
	savingsPct := 0.0
	if stats.TotalOriginalBytes > 0 {
		savingsPct = float64(savings) / float64(stats.TotalOriginalBytes) * 100
	}

	jsonOK(w, map[string]any{
		"totalOriginalMB":    toMB(stats.TotalOriginalBytes),
		"totalCompressedMB":  toMB(stats.TotalCompressedBytes),
		"savingsMB":          toMB(savings),
		"savingsPercent":     int(savingsPct),
	})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toArchivedImageView(a store.ArchiveManifest) store.ArchivedImageView {
	restoreStatus := a.RestoreStatus
	if restoreStatus == "" {
		restoreStatus = "idle"
	}
	return store.ArchivedImageView{
		ID:             a.ID,
		Repo:           a.Repo,
		Tag:            a.Tag,
		CompressedSize: formatBytes(a.CompressedBytes),
		OriginalSize:   formatBytes(a.OriginalBytes),
		ArchivedAt:     a.ArchivedAt.Format("2006-01-02T15:04:05Z"),
		Restorable:     restoreStatus != "restored",
		RestoreStatus:  restoreStatus,
		StorageBackend: a.StorageBackend,
	}
}

func formatBytes(b int64) string {
	const mb = 1024 * 1024
	const gb = 1024 * mb
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%d MB", b/mb)
	default:
		return fmt.Sprintf("%d KB", b/1024)
	}
}

func toMB(b int64) int64 {
	return b / (1024 * 1024)
}
