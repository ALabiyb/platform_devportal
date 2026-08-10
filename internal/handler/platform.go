// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// platform.go — Admin API for the Platform Engineering IDP layer.
//
// Routes (all admin-only, registered in handler.go under /api/v1/admin/):
//
//   GET    /admin/clusters                            — list clusters for org
//   POST   /admin/clusters                            — register a cluster
//   PUT    /admin/clusters/:clusterID                 — update cluster fields
//   GET    /admin/clusters/:clusterID/services        — list platform services
//   PUT    /admin/clusters/:clusterID/services/:type  — upsert service config
//
//   GET    /admin/manifest-templates                  — list all 15 templates
//   PUT    /admin/manifest-templates/:name            — update template content
//
//   GET    /admin/environment-profiles               — list dev/uat/prod profiles
//   PUT    /admin/environment-profiles/:name         — update one profile

package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	authpkg "github.com/ALabiyb/platform_devportal/internal/auth"
	"github.com/ALabiyb/platform_devportal/internal/db"
)

// ── Clusters ──────────────────────────────────────────────────────────────────

func (h *Handler) ListClusters(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())
	clusters, err := h.db.ListClusters(r.Context(), user.OrgID)
	if err != nil {
		jsonError(w, "list clusters: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if clusters == nil {
		clusters = []db.Cluster{}
	}
	jsonOK(w, clusters)
}

func (h *Handler) CreateCluster(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())

	var req struct {
		Name                   string  `json:"name"`
		DisplayName            string  `json:"display_name"`
		Environment            string  `json:"environment"`
		APIEndpoint            string  `json:"api_endpoint"`
		ArgoCDURL              string  `json:"argocd_url"`
		KubeconfigCredentialID *string `json:"kubeconfig_credential_id"`
		ArgoCDCredentialID     *string `json:"argocd_credential_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Environment == "" || req.APIEndpoint == "" {
		jsonError(w, "name, environment, and api_endpoint are required", http.StatusBadRequest)
		return
	}
	if req.Environment != "dev" && req.Environment != "uat" && req.Environment != "prod" {
		jsonError(w, "environment must be dev, uat, or prod", http.StatusBadRequest)
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Name
	}

	c := db.Cluster{
		OrgID:       user.OrgID,
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Environment: req.Environment,
		APIEndpoint: req.APIEndpoint,
		ArgoCDURL:   req.ArgoCDURL,
		Status:      "active",
		CreatedBy:   &user.ID,
	}
	if req.KubeconfigCredentialID != nil {
		id, err := uuid.Parse(*req.KubeconfigCredentialID)
		if err == nil {
			c.KubeconfigCredentialID = &id
		}
	}
	if req.ArgoCDCredentialID != nil {
		id, err := uuid.Parse(*req.ArgoCDCredentialID)
		if err == nil {
			c.ArgoCDCredentialID = &id
		}
	}

	cluster, err := h.db.CreateCluster(r.Context(), c)
	if err != nil {
		jsonError(w, "create cluster: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(cluster)
}

func (h *Handler) UpdateCluster(w http.ResponseWriter, r *http.Request) {
	clusterID, err := parseUUID(r, "clusterID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req struct {
		Name                   string  `json:"name"`
		DisplayName            string  `json:"display_name"`
		APIEndpoint            string  `json:"api_endpoint"`
		ArgoCDURL              string  `json:"argocd_url"`
		KubeconfigCredentialID *string `json:"kubeconfig_credential_id"`
		ArgoCDCredentialID     *string `json:"argocd_credential_id"`
		Status                 string  `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Status == "" {
		req.Status = "active"
	}

	c := db.Cluster{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		APIEndpoint: req.APIEndpoint,
		ArgoCDURL:   req.ArgoCDURL,
		Status:      req.Status,
	}
	if req.KubeconfigCredentialID != nil {
		id, err := uuid.Parse(*req.KubeconfigCredentialID)
		if err == nil {
			c.KubeconfigCredentialID = &id
		}
	}
	if req.ArgoCDCredentialID != nil {
		id, err := uuid.Parse(*req.ArgoCDCredentialID)
		if err == nil {
			c.ArgoCDCredentialID = &id
		}
	}

	cluster, err := h.db.UpdateCluster(r.Context(), clusterID, c)
	if err != nil {
		if err == pgx.ErrNoRows {
			jsonError(w, "cluster not found", http.StatusNotFound)
			return
		}
		jsonError(w, "update cluster: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, cluster)
}

// ── Cluster Platform Services ─────────────────────────────────────────────────

func (h *Handler) ListClusterServices(w http.ResponseWriter, r *http.Request) {
	clusterID, err := parseUUID(r, "clusterID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	svcs, err := h.db.ListClusterPlatformServices(r.Context(), clusterID)
	if err != nil {
		jsonError(w, "list cluster services: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if svcs == nil {
		svcs = []db.ClusterPlatformService{}
	}
	jsonOK(w, svcs)
}

func (h *Handler) UpsertClusterService(w http.ResponseWriter, r *http.Request) {
	clusterID, err := parseUUID(r, "clusterID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	serviceType := chi.URLParam(r, "serviceType")
	if serviceType == "" {
		jsonError(w, "service type is required", http.StatusBadRequest)
		return
	}

	user, _ := authpkg.UserFromContext(r.Context())

	var req struct {
		Enabled bool            `json:"enabled"`
		Config  json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Config == nil {
		req.Config = json.RawMessage("{}")
	}

	svc, err := h.db.UpsertClusterPlatformService(r.Context(), db.ClusterPlatformService{
		ClusterID:   clusterID,
		ServiceType: serviceType,
		Enabled:     req.Enabled,
		Config:      req.Config,
	}, user.ID)
	if err != nil {
		jsonError(w, "upsert cluster service: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, svc)
}

// ── Manifest Templates ────────────────────────────────────────────────────────

func (h *Handler) ListManifestTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.db.ListManifestTemplates(r.Context())
	if err != nil {
		jsonError(w, "list manifest templates: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if templates == nil {
		templates = []db.ManifestTemplate{}
	}
	jsonOK(w, templates)
}

func (h *Handler) UpsertManifestTemplate(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		jsonError(w, "template name is required", http.StatusBadRequest)
		return
	}

	user, _ := authpkg.UserFromContext(r.Context())

	var req struct {
		DisplayName string `json:"display_name"`
		Conditional string `json:"conditional"`
		Content     string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		jsonError(w, "content is required", http.StatusBadRequest)
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = name
	}

	t, err := h.db.UpsertManifestTemplate(r.Context(), db.ManifestTemplate{
		Name:        name,
		DisplayName: req.DisplayName,
		Conditional: req.Conditional,
		Content:     req.Content,
		UpdatedBy:   &user.ID,
	})
	if err != nil {
		jsonError(w, "upsert manifest template: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, t)
}

// ── Environment Profiles ──────────────────────────────────────────────────────

func (h *Handler) ListEnvironmentProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.db.ListEnvironmentProfiles(r.Context())
	if err != nil {
		jsonError(w, "list environment profiles: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if profiles == nil {
		profiles = []db.EnvironmentProfile{}
	}
	jsonOK(w, profiles)
}

func (h *Handler) UpdateEnvironmentProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name != "dev" && name != "uat" && name != "prod" {
		jsonError(w, "name must be dev, uat, or prod", http.StatusBadRequest)
		return
	}

	user, _ := authpkg.UserFromContext(r.Context())

	var req struct {
		CPURequest   string `json:"cpu_request"`
		MemRequest   string `json:"mem_request"`
		CPULimit     string `json:"cpu_limit"`
		MemLimit     string `json:"mem_limit"`
		Replicas     int    `json:"replicas"`
		StorageClass string `json:"storage_class"`
		HPAEnabled   bool   `json:"hpa_enabled"`
		HPAMin       int    `json:"hpa_min"`
		HPAMax       int    `json:"hpa_max"`
		CPUThreshold int    `json:"cpu_threshold"`
		MemThreshold int    `json:"mem_threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	p, err := h.db.UpdateEnvironmentProfile(r.Context(), db.EnvironmentProfile{
		Name:         name,
		CPURequest:   req.CPURequest,
		MemRequest:   req.MemRequest,
		CPULimit:     req.CPULimit,
		MemLimit:     req.MemLimit,
		Replicas:     req.Replicas,
		StorageClass: req.StorageClass,
		HPAEnabled:   req.HPAEnabled,
		HPAMin:       req.HPAMin,
		HPAMax:       req.HPAMax,
		CPUThreshold: req.CPUThreshold,
		MemThreshold: req.MemThreshold,
	}, user.ID)
	if err != nil {
		if err == pgx.ErrNoRows {
			jsonError(w, "profile not found", http.StatusNotFound)
			return
		}
		jsonError(w, "update environment profile: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, p)
}
