package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TrivyScan serves POST /functions/v1/trivy-scan
func (h *Handlers) TrivyScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Image   string `json:"image"`
		ImageID string `json:"imageId"`
		Repo    string `json:"repo"`
		Tag     string `json:"tag"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Normalize image reference
	imageRef := req.Image
	if imageRef == "" && req.Repo != "" {
		imageRef = req.Repo
		if req.Tag != "" {
			imageRef += ":" + req.Tag
		}
	}
	if imageRef == "" {
		jsonError(w, http.StatusBadRequest, "image or repo required")
		return
	}

	if h.cfg.TrivyServerURL != "" {
		h.forwardToTrivy(w, imageRef)
		return
	}

	// No Trivy server configured — return empty scan result
	jsonOK(w, map[string]any{
		"scannedAt":       time.Now().UTC().Format(time.RFC3339),
		"image":           imageRef,
		"totalCritical":   0,
		"totalHigh":       0,
		"totalMedium":     0,
		"totalLow":        0,
		"vulnerabilities": []any{},
		"note":            "TRIVY_SERVER_URL not configured; set it to enable real vulnerability scanning",
	})
}

func (h *Handlers) forwardToTrivy(w http.ResponseWriter, imageRef string) {
	url := h.cfg.TrivyServerURL + "/v1/scan"
	body := fmt.Sprintf(`{"image":%q}`, imageRef)

	resp, err := http.Post(url, "application/json", bytes.NewBufferString(body))
	if err != nil {
		jsonError(w, http.StatusBadGateway, "trivy server unreachable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to read trivy response")
		return
	}

	// Validate it's JSON before forwarding
	if !json.Valid(data) {
		jsonError(w, http.StatusBadGateway, "trivy server returned invalid JSON")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(data)
}
