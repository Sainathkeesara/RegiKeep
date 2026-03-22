package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/regikeep/rgk/internal/core"
)

// ListGroupsAPI serves GET /api/v1/groups
func (h *Handlers) ListGroupsAPI(w http.ResponseWriter, r *http.Request) {
	groups, err := h.groupSvc.ListGroupsWithStats()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if groups == nil {
		groups = []core.GroupWithStats{}
	}
	jsonOK(w, map[string]any{"groups": groups, "total": len(groups)})
}

// GetGroup serves GET /api/v1/groups/:id
func (h *Handlers) GetGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	g, err := h.db.GetGroup(id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if g == nil {
		jsonError(w, http.StatusNotFound, "group not found")
		return
	}
	jsonOK(w, g)
}

// CreateGroup serves POST /api/v1/groups
func (h *Handlers) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Interval string `json:"interval"`
		Strategy string `json:"strategy"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name required")
		return
	}
	if req.Interval == "" {
		req.Interval = "7d"
	}
	if req.Strategy == "" {
		req.Strategy = "pull"
	}

	g, err := h.db.CreateGroup(req.Name, req.Interval, req.Strategy)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonCreated(w, g)
}

// UpdateGroup serves PATCH /api/v1/groups/:id
func (h *Handlers) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.db.GetGroup(id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		jsonError(w, http.StatusNotFound, "group not found")
		return
	}

	var req struct {
		Name     string `json:"name"`
		Interval string `json:"interval"`
		Strategy string `json:"strategy"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Apply patches
	if req.Name == "" {
		req.Name = existing.Name
	}
	if req.Interval == "" {
		req.Interval = existing.Interval
	}
	if req.Strategy == "" {
		req.Strategy = existing.Strategy
	}

	if err := h.db.UpdateGroup(id, req.Name, req.Interval, req.Strategy); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, _ := h.db.GetGroup(id)
	jsonOK(w, updated)
}

// DeleteGroup serves DELETE /api/v1/groups/:id
func (h *Handlers) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.DeleteGroup(id); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonNoContent(w)
}

// EnableGroup serves POST /api/v1/groups/:id/enable
func (h *Handlers) EnableGroup(w http.ResponseWriter, r *http.Request) {
	h.setGroupEnabled(w, chi.URLParam(r, "id"), true)
}

// DisableGroup serves POST /api/v1/groups/:id/disable
func (h *Handlers) DisableGroup(w http.ResponseWriter, r *http.Request) {
	h.setGroupEnabled(w, chi.URLParam(r, "id"), false)
}

func (h *Handlers) setGroupEnabled(w http.ResponseWriter, id string, enabled bool) {
	if err := h.db.SetGroupEnabled(id, enabled); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	g, _ := h.db.GetGroup(id)
	jsonOK(w, g)
}
