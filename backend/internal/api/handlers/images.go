package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/regikeep/rgk/internal/core"
	"github.com/regikeep/rgk/internal/registry"
	"github.com/regikeep/rgk/internal/store"
)

// ListImages serves GET /functions/v1/registry-images and GET /api/v1/images
// Query params: registry, status, search
func (h *Handlers) ListImages(w http.ResponseWriter, r *http.Request) {
	filter := store.ImageFilter{
		Registry: r.URL.Query().Get("registry"),
		Status:   r.URL.Query().Get("status"),
		Search:   r.URL.Query().Get("search"),
	}

	images, err := h.db.ListImages(filter)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build registry name→region map (img.Registry stores the adapter name, not DB id)
	registries, _ := h.db.ListRegistries()
	regionMap := make(map[string]string)
	for _, reg := range registries {
		regionMap[reg.Name] = reg.Region
	}

	// Build group ID→name map
	groups, _ := h.db.ListGroups()
	groupMap := make(map[string]string)
	for _, g := range groups {
		groupMap[g.ID] = g.Name
	}

	views := make([]store.RegistryImageView, 0, len(images))
	for _, img := range images {
		views = append(views, toRegistryImageView(img, regionMap, groupMap))
	}

	jsonOK(w, map[string]any{
		"images": views,
		"total":  len(views),
	})
}

// ImageAction serves POST /functions/v1/registry-images
// Handles: pin, unpin, export actions
func (h *Handlers) ImageAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ImageID        string `json:"imageId"`
		Action         string `json:"action"`
		SourceRegistry string `json:"sourceRegistry"`
		TargetRegistry string `json:"targetRegistry"`
		Repo           string `json:"repo"`
		Tag            string `json:"tag"`
		GroupName      string `json:"groupName"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.Action {
	case "pin":
		h.doPinUnpin(w, req.ImageID, true)
	case "unpin":
		h.doPinUnpin(w, req.ImageID, false)
	case "export":
		h.doExportImage(w, req.ImageID, req.SourceRegistry, req.TargetRegistry, req.Repo, req.Tag)
	case "set-group":
		h.doSetGroup(w, req.ImageID, req.GroupName)
	case "remove-group":
		h.doRemoveGroup(w, req.ImageID)
	default:
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("unknown action %q", req.Action))
	}
}

func (h *Handlers) doSetGroup(w http.ResponseWriter, imageID, groupName string) {
	if imageID == "" || groupName == "" {
		jsonError(w, http.StatusBadRequest, "imageId and groupName required")
		return
	}
	grp, err := h.db.GetGroupByName(groupName)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if grp == nil {
		jsonError(w, http.StatusNotFound, fmt.Sprintf("group '%s' not found", groupName))
		return
	}
	if err := h.db.SetImageGroup(imageID, &grp.ID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]any{"success": true, "imageId": imageID, "group": groupName})
}

// AssignRegistry serves PATCH /api/v1/images/:id/registry
// Updates the registry field so the image appears under the correct tab.
func (h *Handlers) AssignRegistry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Registry string `json:"registry"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Registry == "" {
		jsonError(w, http.StatusBadRequest, "registry required")
		return
	}
	if err := h.db.SetImageRegistry(id, req.Registry); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]any{"success": true, "imageId": id, "registry": req.Registry})
}

func (h *Handlers) doRemoveGroup(w http.ResponseWriter, imageID string) {
	if imageID == "" {
		jsonError(w, http.StatusBadRequest, "imageId required")
		return
	}
	if err := h.db.SetImageGroup(imageID, nil); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]any{"success": true, "imageId": imageID, "group": nil})
}

// PinImage serves POST /api/v1/images/:id/pin
func (h *Handlers) PinImage(w http.ResponseWriter, r *http.Request) {
	h.doPinUnpin(w, chi.URLParam(r, "id"), true)
}

// UnpinImage serves POST /api/v1/images/:id/unpin
func (h *Handlers) UnpinImage(w http.ResponseWriter, r *http.Request) {
	h.doPinUnpin(w, chi.URLParam(r, "id"), false)
}

func (h *Handlers) doPinUnpin(w http.ResponseWriter, imageID string, pin bool) {
	if imageID == "" {
		jsonError(w, http.StatusBadRequest, "imageId required")
		return
	}

	img, err := h.db.GetImage(imageID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if img == nil {
		jsonError(w, http.StatusNotFound, "image not found")
		return
	}

	if err := h.db.SetPinned(imageID, pin); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update status to reflect pin state
	status := "safe"
	if !pin {
		status = "unpinned"
	}
	_ = h.db.UpdateKeepaliveStatus(imageID, status, nil)

	jsonOK(w, map[string]any{
		"success": true,
		"imageId": imageID,
		"action":  pinAction(pin),
		"pinned":  pin,
	})
}

func (h *Handlers) doExportImage(w http.ResponseWriter, imageID, src, target, repo, tag string) {
	// Cross-registry export: in MVP we validate and return success.
	// Full push implementation is Phase 2 (requires docker daemon or skopeo).
	if imageID == "" {
		jsonError(w, http.StatusBadRequest, "imageId required")
		return
	}
	if src == "" || target == "" {
		jsonError(w, http.StatusBadRequest, "sourceRegistry and targetRegistry required")
		return
	}

	jsonOK(w, map[string]any{
		"success": true,
		"message": fmt.Sprintf("export of %s:%s from %s to %s queued (Phase 2 feature)", repo, tag, src, target),
	})
}

// CreateImage serves POST /api/v1/images
func (h *Handlers) CreateImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Registry string `json:"registry"`
		Repo     string `json:"repo"`
		Tag      string `json:"tag"`
		Digest   string `json:"digest"`
		GroupID  string `json:"groupId"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Repo == "" {
		jsonError(w, http.StatusBadRequest, "repo required")
		return
	}

	img := store.ImageRef{
		Registry: req.Registry,
		Repo:     req.Repo,
		Tag:      req.Tag,
		Digest:   req.Digest,
	}
	if req.GroupID != "" {
		img.GroupID = &req.GroupID
	}

	// Attempt to resolve digest via registry adapter if not provided
	if img.Digest == "" && img.Tag != "" && img.Registry != "" {
		if adapter, err := h.regMgr.Get(img.Registry); err == nil {
			if digest, err := adapter.ResolveDigest(img.Repo, img.Tag); err == nil {
				img.Digest = digest
			}
		}
	}

	created, err := h.db.CreateImage(img)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonCreated(w, created)
}

// DeleteImage serves DELETE /api/v1/images/:id
func (h *Handlers) DeleteImage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.DeleteImage(id); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonNoContent(w)
}

// ManualKeepalive serves POST /api/v1/images/:id/keepalive
func (h *Handlers) ManualKeepalive(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Strategy string `json:"strategy"`
	}
	_ = decode(r, &req)
	if req.Strategy == "" {
		req.Strategy = "pull"
	}

	resp, err := h.keepSvc.Run(core.KeepaliveRequest{
		ImageIDs: []string{id},
		Strategy: req.Strategy,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, resp)
}

// ImageHistory serves GET /api/v1/images/:id/history
func (h *Handlers) ImageHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	logs, err := h.db.ListKeepaliveLogs(id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if logs == nil {
		logs = []store.KeepaliveLog{}
	}
	jsonOK(w, map[string]any{"history": logs, "imageId": id})
}

// TriggerKeepalive serves POST /functions/v1/keepalive and POST /api/v1/keepalive
func (h *Handlers) TriggerKeepalive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ImageIDs []string `json:"imageIds"`
		Group    string   `json:"group"`
		Strategy string   `json:"strategy"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.keepSvc.Run(core.KeepaliveRequest{
		ImageIDs: req.ImageIDs,
		Group:    req.Group,
		Strategy: req.Strategy,
	})
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, resp)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toRegistryImageView(img store.ImageRef, regionMap, groupMap map[string]string) store.RegistryImageView {
	region := regionMap[img.Registry]
	groupName := ""
	if img.GroupID != nil {
		groupName = groupMap[*img.GroupID]
	}

	lastKeepalive := ""
	if img.LastKeepaliveAt != nil {
		lastKeepalive = img.LastKeepaliveAt.Format("2006-01-02T15:04:05Z")
	}

	lastError := ""
	if img.LastError != nil {
		lastError = *img.LastError
	}

	return store.RegistryImageView{
		ID:            img.ID,
		Repo:          img.Repo,
		Tag:           img.Tag,
		Digest:        img.Digest,
		Region:        region,
		Size:          img.Size,
		Group:         groupName,
		Pinned:        img.Pinned,
		ExpiresIn:     img.ExpiresInDays,
		LastKeepalive: lastKeepalive,
		Status:        img.LastStatus,
		Registry:      img.Registry,
		LastError:     lastError,
	}
}

func pinAction(pin bool) string {
	if pin {
		return "pin"
	}
	return "unpin"
}

// PushToRegistry pulls an image from Docker Hub and pushes it to a target registry.
func (h *Handlers) PushToRegistry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Image          string `json:"image"`          // e.g. "sainath2/nginx:v1"
		TargetRegistry string `json:"targetRegistry"` // e.g. "ecr"
		TargetRepo     string `json:"targetRepo"`     // optional override
		Group          string `json:"group"`           // optional group name
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Image == "" || req.TargetRegistry == "" {
		jsonError(w, http.StatusBadRequest, "image and targetRegistry are required")
		return
	}

	// Parse image reference
	repo, tag := req.Image, "latest"
	if idx := strings.LastIndex(req.Image, ":"); idx > 0 {
		repo = req.Image[:idx]
		tag = req.Image[idx+1:]
	}

	dstRepo := req.TargetRepo
	if dstRepo == "" {
		dstRepo = repo
	}

	// Get or create DockerHub adapter
	srcAdapter, _ := h.regMgr.Get("dockerhub")
	var dhAdapter *registry.DockerHubAdapter
	if srcAdapter != nil {
		dhAdapter, _ = srcAdapter.(*registry.DockerHubAdapter)
	}
	if dhAdapter == nil {
		dhAdapter = registry.NewDockerHubAdapterWithCreds("dockerhub", "",
			h.cfg.DockerHubUsername, h.cfg.DockerHubAccessToken)
	}

	// Get destination adapter
	dstAdapter, err := h.regMgr.Get(req.TargetRegistry)
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("target registry '%s' not found", req.TargetRegistry))
		return
	}
	ecrAdapter, ok := dstAdapter.(*registry.ECRAdapter)
	if !ok {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("target registry '%s' is not ECR", req.TargetRegistry))
		return
	}

	if err := ecrAdapter.Authenticate(); err != nil {
		jsonError(w, http.StatusInternalServerError, "ECR auth failed: "+err.Error())
		return
	}

	// Perform cross-registry copy
	var logs []string
	result, err := registry.CopyDockerHubToECR(dhAdapter, ecrAdapter, repo, tag, dstRepo, func(msg string) {
		logs = append(logs, msg)
		h.log.Info().Str("push", repo+":"+tag).Msg(msg)
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "push failed: "+err.Error())
		return
	}

	// Register in DB
	var groupID *string
	if req.Group != "" {
		grp, err := h.db.GetGroupByName(req.Group)
		if err == nil && grp != nil {
			groupID = &grp.ID
		}
	}

	img := store.ImageRef{
		Registry:      req.TargetRegistry,
		Repo:          dstRepo,
		Tag:           tag,
		Digest:        result.Digest,
		GroupID:       groupID,
		ExpiresInDays: -1,
	}
	created, err := h.db.CreateImage(img)
	if err != nil {
		// Push succeeded but DB insert failed — still return success with a warning
		h.log.Error().Err(err).Msg("push succeeded but failed to register image")
	}

	imageID := ""
	if created != nil {
		imageID = created.ID
	}

	jsonOK(w, map[string]any{
		"success":      true,
		"imageId":      imageID,
		"digest":       result.Digest,
		"blobsCopied":  result.BlobsCopied,
		"blobsSkipped": result.BlobsSkipped,
		"totalBytes":   result.TotalBytes,
		"logs":         logs,
	})
}

// DockerHubSearch proxies Docker Hub search to avoid browser CORS restrictions.
func (h *Handlers) DockerHubSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		jsonError(w, http.StatusBadRequest, "query parameter 'q' required")
		return
	}

	url := fmt.Sprintf("https://hub.docker.com/v2/search/repositories/?query=%s&page_size=15", q)
	resp, err := http.Get(url)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "could not reach Docker Hub: "+err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data json.RawMessage
	if err := json.Unmarshal(body, &data); err != nil {
		jsonError(w, http.StatusBadGateway, "invalid response from Docker Hub")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}
