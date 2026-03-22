package handlers

import (
	"net/http"

	"github.com/regikeep/rgk/internal/core"
)

// RunAudit serves POST /functions/v1/audit and POST /api/v1/audit
func (h *Handlers) RunAudit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DryRun         bool   `json:"dryRun"`
		RegistryFilter string `json:"registryFilter"`
	}
	req.DryRun = true // safe default
	_ = decode(r, &req)

	resp, err := h.auditSvc.Run(core.AuditRequest{
		DryRun:         req.DryRun,
		RegistryFilter: req.RegistryFilter,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, resp)
}
