package handlers

import "net/http"

// DaemonStatus serves GET /api/v1/daemon/status
func (h *Handlers) DaemonStatus(w http.ResponseWriter, r *http.Request) {
	status := h.scheduler.GetStatus()
	jsonOK(w, status)
}

// DaemonStart serves POST /api/v1/daemon/start
func (h *Handlers) DaemonStart(w http.ResponseWriter, r *http.Request) {
	if err := h.scheduler.Start(); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := h.scheduler.GetStatus()
	jsonOK(w, status)
}

// DaemonStop serves POST /api/v1/daemon/stop
func (h *Handlers) DaemonStop(w http.ResponseWriter, r *http.Request) {
	h.scheduler.Stop()
	status := h.scheduler.GetStatus()
	jsonOK(w, status)
}
