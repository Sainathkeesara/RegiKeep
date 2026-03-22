package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/regikeep/rgk/internal/registry"
	"github.com/regikeep/rgk/internal/store"
)

// ListRegistries serves GET /api/v1/registries
// Credentials are redacted in the response for security.
func (h *Handlers) ListRegistries(w http.ResponseWriter, r *http.Request) {
	registries, err := h.db.ListRegistries()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if registries == nil {
		registries = []store.RegistryConfig{}
	}
	// Redact credentials — never send tokens to the browser
	for i := range registries {
		if registries[i].AuthToken != "" {
			registries[i].AuthToken = "••••••••"
		}
	}
	jsonOK(w, map[string]any{"registries": registries, "total": len(registries)})
}

// CreateRegistry serves POST /api/v1/registries
func (h *Handlers) CreateRegistry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name             string `json:"name"`
		RegistryType     string `json:"registryType"`
		Endpoint         string `json:"endpoint"`
		Region           string `json:"region"`
		Tenancy          string `json:"tenancy"`
		CredentialSource string `json:"credentialSource"`
		AuthUsername     string `json:"authUsername"`
		AuthToken        string `json:"authToken"`
		AuthExtra        string `json:"authExtra"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Endpoint == "" {
		jsonError(w, http.StatusBadRequest, "name and endpoint required")
		return
	}
	if req.RegistryType == "" {
		jsonError(w, http.StatusBadRequest, "registryType required (ocir, ecr, dockerhub)")
		return
	}
	credSource := req.CredentialSource
	if credSource == "" {
		if req.AuthUsername != "" || req.AuthToken != "" {
			credSource = "db"
		} else {
			credSource = "env"
		}
	}

	// Check duplicate endpoint
	existing, _ := h.db.GetRegistryByEndpoint(req.Endpoint)
	if existing != nil {
		jsonError(w, http.StatusConflict, "registry with this endpoint already exists")
		return
	}

	reg, err := h.db.CreateRegistry(store.RegistryConfig{
		Name:             req.Name,
		RegistryType:     req.RegistryType,
		Endpoint:         req.Endpoint,
		Region:           req.Region,
		Tenancy:          req.Tenancy,
		CredentialSource: credSource,
		AuthUsername:     req.AuthUsername,
		AuthToken:        req.AuthToken,
		AuthExtra:        req.AuthExtra,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Live-reload: register the new adapter into the running registry manager
	// so keepalives start working immediately without a server restart.
	newMgr := registry.BuildFromDB([]store.RegistryConfig{*reg}, h.cfg)
	for _, id := range newMgr.IDs() {
		if a, err := newMgr.Get(id); err == nil {
			h.regMgr.Register(a)
		}
	}

	jsonCreated(w, reg)
}

// DeleteRegistry serves DELETE /api/v1/registries/:id
func (h *Handlers) DeleteRegistry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.DeleteRegistry(id); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonNoContent(w)
}

// TestRegistry serves POST /api/v1/registries/test
// Tests connectivity by calling Authenticate on the adapter.
func (h *Handlers) TestRegistry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	reg, err := h.db.GetRegistryByEndpoint(req.Endpoint)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if reg == nil {
		jsonError(w, http.StatusNotFound, "no registry found with endpoint '"+req.Endpoint+"'")
		return
	}

	mgr := registry.BuildFromDB([]store.RegistryConfig{*reg}, h.cfg)
	adapter, err := mgr.Get(reg.Name)
	if err != nil {
		jsonOK(w, map[string]any{
			"endpoint": req.Endpoint,
			"success":  false,
			"error":    "no adapter for registry type '" + reg.RegistryType + "'",
		})
		return
	}

	if err := adapter.Authenticate(); err != nil {
		jsonOK(w, map[string]any{
			"endpoint": req.Endpoint,
			"success":  false,
			"error":    err.Error(),
		})
		return
	}

	jsonOK(w, map[string]any{
		"endpoint": req.Endpoint,
		"success":  true,
	})
}
