package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/regikeep/rgk/internal/api/handlers"
)

// registerRoutes mounts all API routes onto the router.
func registerRoutes(r chi.Router, s *Server) {
	h := handlers.NewHandlers(s.db, s.cfg, s.log, s.regMgr, s.scheduler, s.keepSvc, s.auditSvc, s.archSvc, s.restSvc, s.groupSvc)

	// Prometheus metrics
	r.Handle("/metrics", promhttp.Handler())

	// ─── Supabase-compatible routes (/functions/v1/*)
	// The Lovable UI sets VITE_SUPABASE_URL=http://localhost:8080
	// and appends /functions/v1 to get BASE_URL. These routes match exactly.
	r.Route("/functions/v1", func(r chi.Router) {
		r.Get("/registry-images", h.ListImages)
		r.Post("/registry-images", h.ImageAction) // pin | unpin | export

		r.Post("/audit", h.RunAudit)
		r.Post("/keepalive", h.TriggerKeepalive)

		r.Get("/archive", h.ListArchives)
		r.Post("/archive", h.ArchiveAction) // archive | restore (by imageId+action)

		r.Post("/trivy-scan", h.TrivyScan)
	})

	// ─── Structured REST API (/api/v1/*)
	r.Route("/api/v1", func(r chi.Router) {

		// Groups
		r.Get("/groups", h.ListGroupsAPI)
		r.Post("/groups", h.CreateGroup)
		r.Route("/groups/{id}", func(r chi.Router) {
			r.Get("/", h.GetGroup)
			r.Patch("/", h.UpdateGroup)
			r.Delete("/", h.DeleteGroup)
			r.Post("/enable", h.EnableGroup)
			r.Post("/disable", h.DisableGroup)
		})

		// Images
		r.Get("/images", h.ListImages)
		r.Post("/images", h.CreateImage)
		r.Route("/images/{id}", func(r chi.Router) {
			r.Delete("/", h.DeleteImage)
			r.Post("/pin", h.PinImage)
			r.Post("/unpin", h.UnpinImage)
			r.Post("/keepalive", h.ManualKeepalive)
			r.Get("/history", h.ImageHistory)
			r.Patch("/registry", h.AssignRegistry)
		})

		// Audit
		r.Post("/audit", h.RunAudit)

		// Archive — static routes before parameterized
		r.Get("/archive", h.ListArchives)
		r.Post("/archive", h.TriggerArchive)
		r.Get("/archive/stats", h.ArchiveStats) // must be before /archive/{id}
		r.Route("/archive/{id}", func(r chi.Router) {
			r.Post("/restore", h.RestoreArchive)
			r.Delete("/", h.DeleteArchive)
		})

		// Export
		r.Get("/export", h.Export)

		// Daemon
		r.Get("/daemon/status", h.DaemonStatus)
		r.Post("/daemon/start", h.DaemonStart)
		r.Post("/daemon/stop", h.DaemonStop)

		// Registries
		r.Get("/registries", h.ListRegistries)
		r.Post("/registries", h.CreateRegistry)
		r.Post("/registries/test", h.TestRegistry)
		r.Delete("/registries/{id}", h.DeleteRegistry)

		// Keepalive (also on /api/v1)
		r.Post("/keepalive", h.TriggerKeepalive)

		// Docker Hub search proxy (avoids browser CORS)
		r.Get("/dockerhub/search", h.DockerHubSearch)

		// Cross-registry push (DockerHub → ECR)
		r.Post("/push", h.PushToRegistry)
	})

	// Catch-all OPTIONS for CORS preflight
	r.Options("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
