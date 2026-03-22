package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/regikeep/rgk/internal/config"
	"github.com/regikeep/rgk/internal/core"
	"github.com/regikeep/rgk/internal/daemon"
	"github.com/regikeep/rgk/internal/registry"
	"github.com/regikeep/rgk/internal/store"
)

// Handlers bundles all handler dependencies.
type Handlers struct {
	db        *store.DB
	cfg       *config.Config
	log       zerolog.Logger
	regMgr    *registry.Manager
	scheduler *daemon.Scheduler
	keepSvc   *core.KeepaliveService
	auditSvc  *core.AuditService
	archSvc   *core.ArchiveService
	restSvc   *core.RestoreService
	groupSvc  *core.GroupService
}

// NewHandlers constructs the Handlers struct.
func NewHandlers(
	db *store.DB,
	cfg *config.Config,
	log zerolog.Logger,
	regMgr *registry.Manager,
	scheduler *daemon.Scheduler,
	keepSvc *core.KeepaliveService,
	auditSvc *core.AuditService,
	archSvc *core.ArchiveService,
	restSvc *core.RestoreService,
	groupSvc *core.GroupService,
) *Handlers {
	return &Handlers{
		db:        db,
		cfg:       cfg,
		log:       log,
		regMgr:    regMgr,
		scheduler: scheduler,
		keepSvc:   keepSvc,
		auditSvc:  auditSvc,
		archSvc:   archSvc,
		restSvc:   restSvc,
		groupSvc:  groupSvc,
	}
}

// ─── Response helpers ────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(payload)
}

func jsonCreated(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(payload)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

func jsonNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// decode reads JSON from the request body into v.
func decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
