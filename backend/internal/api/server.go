package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"

	"github.com/regikeep/rgk/internal/config"
	"github.com/regikeep/rgk/internal/core"
	"github.com/regikeep/rgk/internal/daemon"
	"github.com/regikeep/rgk/internal/registry"
	"github.com/regikeep/rgk/internal/store"
)

// Server holds all dependencies and the HTTP server.
type Server struct {
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
	httpSrv   *http.Server
}

// NewServer wires all services together and builds the HTTP router.
func NewServer(db *store.DB, cfg *config.Config, log zerolog.Logger) *Server {
	// Load registry adapters from DB first; fall back to env-var-only adapters
	regs, err := db.ListRegistries()
	var regMgr *registry.Manager
	if err != nil || len(regs) == 0 {
		regMgr = registry.NewManager()
		if cfg.OCIREndpoint != "" {
			regMgr.Register(registry.NewOCIRAdapter(
				"ocir-fra",
				cfg.OCIREndpoint,
				cfg.OCIRTenancy,
				cfg.OCIRUsername,
				cfg.OCIRAuthToken,
				cfg.OCIRRegion,
				cfg.OCIRCompartmentOCID,
			))
		}
		if cfg.ECRAccountID != "" {
			regMgr.Register(registry.NewECRAdapter("ecr-use1", cfg.ECRAccountID, cfg.AWSRegion))
		}
		if cfg.DockerHubNamespace != "" {
			regMgr.Register(registry.NewDockerHubAdapter("dockerhub", cfg.DockerHubNamespace))
		}
	} else {
		regMgr = registry.BuildFromDB(regs, cfg)
		log.Info().Int("count", len(regMgr.IDs())).Msg("loaded registries from database")
	}

	sched := daemon.NewScheduler(db, regMgr, cfg.DaemonWorkers, log)

	s := &Server{
		db:        db,
		cfg:       cfg,
		log:       log,
		regMgr:    regMgr,
		scheduler: sched,
		keepSvc:   core.NewKeepaliveService(db, regMgr, log),
		auditSvc:  core.NewAuditService(db),
		archSvc:   core.NewArchiveService(db, cfg, log),
		restSvc:   core.NewRestoreService(db, log),
		groupSvc:  core.NewGroupService(db),
	}
	s.archSvc.SetRegistryManager(regMgr)

	router := s.buildRouter()
	s.httpSrv = &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: router,
	}

	if cfg.DaemonAutoStart {
		_ = sched.Start()
	}

	return s
}

func (s *Server) buildRouter() http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Recoverer)
	r.Use(structuredLogger(s.log))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "apikey", "x-client-info"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Mount routes
	registerRoutes(r, s)

	return r
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	s.log.Info().Str("addr", s.cfg.ListenAddr).Msg("regikeep server starting")
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.scheduler.Stop()
	return s.httpSrv.Shutdown(ctx)
}
