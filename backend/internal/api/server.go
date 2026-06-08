package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"a-series-oracle/backend/internal/auth"
	"a-series-oracle/backend/internal/domain"
	"a-series-oracle/backend/internal/oci"
	"a-series-oracle/backend/internal/store"
)

type Server struct {
	store           *store.Store
	mux             *http.ServeMux
	enqueue         func(string)
	executionMode   string
	ociReadiness    oci.ReadinessConfig
	profileResolver OCIProfileResolver
	auth            *auth.Manager
}

type OCIProfileResolver interface {
	Resolve(profileID string, region string) (oci.ReadinessConfig, domain.Profile, error)
}

type ServerOptions struct {
	Enqueue         func(string)
	ExecutionMode   string
	OCIReadiness    oci.ReadinessConfig
	ProfileResolver OCIProfileResolver
	Auth            *auth.Manager
}

func NewServer(store *store.Store) *Server {
	return NewServerWithEnqueuer(store, nil)
}

func NewServerWithEnqueuer(store *store.Store, enqueue func(string)) *Server {
	return NewServerWithOptions(store, ServerOptions{Enqueue: enqueue, ExecutionMode: "local"})
}

func NewServerWithOptions(store *store.Store, options ServerOptions) *Server {
	enqueue := options.Enqueue
	if enqueue == nil {
		enqueue = func(string) {}
	}
	executionMode := strings.TrimSpace(options.ExecutionMode)
	if executionMode == "" {
		executionMode = "local"
	}
	s := &Server{
		store:           store,
		mux:             http.NewServeMux(),
		enqueue:         enqueue,
		executionMode:   executionMode,
		ociReadiness:    options.OCIReadiness,
		profileResolver: options.ProfileResolver,
		auth:            options.Auth,
	}
	s.ociReadiness.ExecutionMode = executionMode
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return withCORS(s.withAuth(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/auth/me", s.handleAuthMe)
	s.mux.HandleFunc("POST /api/auth/login", s.handleAuthLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleAuthLogout)
	s.mux.HandleFunc("GET /api/oci/readiness", s.handleOCIReadiness)
	s.mux.HandleFunc("POST /api/oci/validate-readonly", s.handleOCIValidateReadOnly)
	s.mux.HandleFunc("POST /api/oci/smoke/e2-micro-create-delete", s.handleOCIE2MicroSmoke)
	s.mux.HandleFunc("POST /api/oci/smoke/e3-flex-lifecycle", s.handleOCIE3FlexLifecycle)
	s.mux.HandleFunc("POST /api/oci/smoke/reinstall-instance", s.handleOCIReinstallInstance)
	s.mux.HandleFunc("POST /api/oci/smoke/cleanup", s.handleOCISmokeCleanup)
	s.mux.HandleFunc("GET /api/profiles", s.handleProfiles)
	s.mux.HandleFunc("POST /api/profiles", s.handleCreateProfile)
	s.mux.HandleFunc("GET /api/profiles/{id}", s.handleProfile)
	s.mux.HandleFunc("POST /api/profiles/{id}/test", s.handleTestProfile)
	s.mux.HandleFunc("POST /api/profiles/{id}/enable", s.handleEnableProfile)
	s.mux.HandleFunc("POST /api/profiles/{id}/disable", s.handleDisableProfile)
	s.mux.HandleFunc("DELETE /api/profiles/{id}", s.handleDeleteProfile)
	s.mux.HandleFunc("GET /api/templates", s.handleTemplates)
	s.mux.HandleFunc("GET /api/launch-options", s.handleLaunchOptions)
	s.mux.HandleFunc("GET /api/instances", s.handleInstances)
	s.mux.HandleFunc("POST /api/instances", s.handleCreateInstance)
	s.mux.HandleFunc("GET /api/instances/{id}", s.handleInstance)
	s.mux.HandleFunc("POST /api/instances/{id}/actions", s.handleInstanceAction)
	s.mux.HandleFunc("POST /api/instances/{id}/reboot", s.handleRebootInstance)
	s.mux.HandleFunc("POST /api/instances/{id}/ip-tasks", s.handleCreateIPTask)
	s.mux.HandleFunc("GET /api/jobs", s.handleJobs)
	s.mux.HandleFunc("GET /api/jobs/{id}", s.handleJob)
	s.mux.HandleFunc("POST /api/jobs/{id}/cancel", s.handleCancelJob)
	s.mux.HandleFunc("POST /api/jobs/{id}/retry", s.handleRetryJob)
	s.mux.HandleFunc("GET /api/automations", s.handleAutomations)
	s.mux.HandleFunc("POST /api/automations/tasks", s.handleCreateAutomationTask)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"service":        "oci-lifecycle-api",
		"executionMode":  s.executionMode,
		"ociApiVerified": false,
		"timestamp":      time.Now().UTC(),
	})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"authEnabled":   s.auth != nil && s.auth.Enabled(),
		"authenticated": s.auth == nil || s.auth.IsAuthenticated(r),
	})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if s.auth == nil || !s.auth.Enabled() {
		writeJSON(w, http.StatusOK, map[string]any{
			"authEnabled":   false,
			"authenticated": true,
		})
		return
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if !s.auth.VerifyPassword(req.Password) {
		writeError(w, http.StatusUnauthorized, "INVALID_PASSWORD", "invalid panel password")
		return
	}
	s.auth.IssueSession(w)
	writeJSON(w, http.StatusOK, map[string]any{
		"authEnabled":   true,
		"authenticated": true,
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if s.auth != nil {
		s.auth.ClearSession(w)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": false,
	})
}

func (s *Server) handleOCIReadiness(w http.ResponseWriter, r *http.Request) {
	cfg, _, err := s.resolveOCIConfig(r.URL.Query().Get("profileId"), r.URL.Query().Get("region"))
	if err != nil {
		writeJSON(w, http.StatusOK, oci.Readiness{
			ExecutionMode: s.executionMode,
			Ready:         false,
			Message:       err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, oci.CheckReadiness(cfg))
}

func (s *Server) handleOCIValidateReadOnly(w http.ResponseWriter, r *http.Request) {
	var req oci.ReadOnlyValidationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	cfg, profile, err := s.resolveOCIConfig(req.ProfileID, req.Region)
	if err != nil {
		writeError(w, http.StatusBadRequest, "OCI_PROFILE_RESOLVE_FAILED", err.Error())
		return
	}
	result := oci.ValidateReadOnly(r.Context(), cfg, req)
	if result.Verified {
		_, _ = s.store.SetProfileStatus(profile.ID, "Healthy", result.ValidatedAt)
	} else {
		_, _ = s.store.SetProfileStatus(profile.ID, "Limited", result.ValidatedAt)
	}
	if !result.Verified {
		writeJSON(w, http.StatusBadGateway, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleOCIE2MicroSmoke(w http.ResponseWriter, r *http.Request) {
	var req oci.E2MicroSmokeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	result := oci.SmokeCreateDeleteE2Micro(r.Context(), s.ociReadiness, req)
	if !result.Verified {
		writeJSON(w, http.StatusBadGateway, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleOCIE3FlexLifecycle(w http.ResponseWriter, r *http.Request) {
	var req oci.E3FlexLifecycleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	result := oci.SmokeE3FlexLifecycle(r.Context(), s.ociReadiness, req)
	if !result.Verified {
		writeJSON(w, http.StatusBadGateway, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleOCISmokeCleanup(w http.ResponseWriter, r *http.Request) {
	var req oci.SmokeCleanupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	result := oci.CleanupSmokeInstances(r.Context(), s.ociReadiness, req)
	if !result.Verified {
		writeJSON(w, http.StatusBadGateway, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleOCIReinstallInstance(w http.ResponseWriter, r *http.Request) {
	var req oci.ReinstallInstanceSmokeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	result := oci.SmokeReinstallInstance(r.Context(), s.ociReadiness, req)
	if !result.Verified {
		writeJSON(w, http.StatusBadGateway, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.store.ListProfiles(),
	})
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	profile, err := s.store.CreateProfile(req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, profile)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.store.GetProfile(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "PROFILE_NOT_FOUND", "profile not found")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleTestProfile(w http.ResponseWriter, r *http.Request) {
	req := domain.ProfileTestRequest{}
	if r.Body != nil {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
			return
		}
	}
	if req.ProfileID == "" {
		req.ProfileID = r.PathValue("id")
	}
	cfg, profile, err := s.resolveOCIConfig(req.ProfileID, req.Region)
	if err != nil {
		writeError(w, http.StatusBadRequest, "OCI_PROFILE_RESOLVE_FAILED", err.Error())
		return
	}
	result := oci.ValidateReadOnly(r.Context(), cfg, oci.ReadOnlyValidationRequest{
		ProfileID:     profile.ID,
		Region:        cfg.Region,
		CompartmentID: req.CompartmentID,
	})
	if result.Verified {
		_, _ = s.store.SetProfileStatus(profile.ID, "Healthy", result.ValidatedAt)
		writeJSON(w, http.StatusOK, result)
		return
	}
	_, _ = s.store.SetProfileStatus(profile.ID, "Limited", result.ValidatedAt)
	writeJSON(w, http.StatusBadGateway, result)
}

func (s *Server) handleEnableProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := s.store.SetProfileStatus(r.PathValue("id"), "Enabled", time.Now().UTC())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleDisableProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := s.store.SetProfileStatus(r.PathValue("id"), "Disabled", time.Now().UTC())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteProfile(r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.store.ListTemplates(),
	})
}

func (s *Server) handleLaunchOptions(w http.ResponseWriter, r *http.Request) {
	base := s.store.GetLaunchOptions()
	if s.executionMode != "oci" {
		normalizeLaunchOptions(&base)
		writeJSON(w, http.StatusOK, base)
		return
	}

	query := r.URL.Query()
	profileID := query.Get("profileId")
	region := query.Get("region")
	cfg, profile, err := s.resolveOCIConfig(profileID, region)
	if err != nil {
		base.Verified = false
		base.ProfileID = profileID
		base.Region = region
		base.ErrorCode = "OCI_PROFILE_RESOLVE_FAILED"
		base.ErrorMessage = err.Error()
		normalizeLaunchOptions(&base)
		writeJSON(w, http.StatusOK, base)
		return
	}
	result := oci.DiscoverLaunchOptions(r.Context(), cfg, oci.LaunchOptionsRequest{
		ProfileID:          profile.ID,
		Region:             cfg.Region,
		CompartmentID:      query.Get("compartmentId"),
		AvailabilityDomain: query.Get("availabilityDomain"),
		VCNID:              query.Get("vcnId"),
		Shape:              query.Get("shape"),
	})
	result.ProfileID = profile.ID
	result.Profiles = base.Profiles
	result.Templates = base.Templates
	normalizeLaunchOptions(&result)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleInstances(w http.ResponseWriter, r *http.Request) {
	if s.executionMode == "oci" {
		cfg, profile, err := s.resolveOCIConfig(r.URL.Query().Get("profileId"), r.URL.Query().Get("region"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "OCI_PROFILE_RESOLVE_FAILED", err.Error())
			return
		}
		result := oci.ListInstanceInventory(r.Context(), cfg, oci.InstanceInventoryRequest{
			CompartmentID:  r.URL.Query().Get("compartmentId"),
			Status:         r.URL.Query().Get("status"),
			IncludeNetwork: true,
		})
		if !result.Verified {
			writeJSON(w, http.StatusBadGateway, result)
			return
		}
		for i := range result.Items {
			result.Items[i].ProfileID = profile.ID
			if result.Items[i].Region == "" {
				result.Items[i].Region = cfg.Region
			}
		}
		if err := s.store.SyncInstances(result.Items); err != nil {
			writeError(w, http.StatusInternalServerError, "INSTANCE_SYNC_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.store.ListInstances(r.URL.Query().Get("status")),
	})
}

func (s *Server) handleCreateInstance(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateInstanceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if s.executionMode == "oci" {
		cfg, profile, err := s.resolveOCIConfig(req.ProfileID, req.Region)
		if err != nil {
			writeError(w, http.StatusBadRequest, "OCI_PROFILE_RESOLVE_FAILED", err.Error())
			return
		}
		req.ProfileID = profile.ID
		if strings.TrimSpace(req.Region) == "" {
			req.Region = cfg.Region
		}
		if strings.TrimSpace(req.CompartmentID) == "" {
			req.CompartmentID = cfg.TenancyOCID
		}
		job, err := s.store.CreateOCIInstanceLaunchTask(req, actorFromRequest(r))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		s.enqueue(job.ID)
		writeJSON(w, http.StatusAccepted, job)
		return
	}

	result, err := s.store.CreateInstanceTask(req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.enqueue(result.Job.ID)
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleInstance(w http.ResponseWriter, r *http.Request) {
	instance, ok := s.store.GetInstance(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "INSTANCE_NOT_FOUND", "实例不存在")
		return
	}
	writeJSON(w, http.StatusOK, instance)
}

func (s *Server) handleInstanceAction(w http.ResponseWriter, r *http.Request) {
	var req domain.InstanceActionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if s.executionMode == "oci" {
		profileID, region, compartmentID := s.ociActionContext(r)
		job, err := s.store.CreateOCIInstanceActionTask(r.PathValue("id"), req, actorFromRequest(r), profileID, region, compartmentID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		s.enqueue(job.ID)
		writeJSON(w, http.StatusAccepted, job)
		return
	}

	job, err := s.store.CreateInstanceActionTask(r.PathValue("id"), req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.enqueue(job.ID)
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleRebootInstance(w http.ResponseWriter, r *http.Request) {
	var req domain.RebootInstanceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if !req.Graceful {
		req.Graceful = true
	}

	if s.executionMode == "oci" {
		profileID, region, compartmentID := s.ociActionContext(r)
		job, err := s.store.CreateOCIInstanceActionTask(r.PathValue("id"), domain.InstanceActionRequest{
			Action:         domain.InstanceActionReboot,
			Graceful:       req.Graceful,
			SnapshotBefore: true,
			Note:           req.Note,
		}, actorFromRequest(r), profileID, region, compartmentID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		s.enqueue(job.ID)
		writeJSON(w, http.StatusAccepted, job)
		return
	}

	job, err := s.store.CreateRebootTask(r.PathValue("id"), req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.enqueue(job.ID)
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleCreateIPTask(w http.ResponseWriter, r *http.Request) {
	var req domain.IPTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if s.executionMode == "oci" {
		profileID, region, compartmentID := s.ociActionContext(r)
		job, err := s.store.CreateOCIIPTask(r.PathValue("id"), req, actorFromRequest(r), profileID, region, compartmentID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		s.enqueue(job.ID)
		writeJSON(w, http.StatusAccepted, job)
		return
	}

	job, err := s.store.CreateIPTask(r.PathValue("id"), req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.enqueue(job.ID)
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.store.ListJobs(),
	})
}

func (s *Server) handleJob(w http.ResponseWriter, r *http.Request) {
	job, ok := s.store.GetJob(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "JOB_NOT_FOUND", "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.store.CancelJob(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleRetryJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.store.RetryJob(r.PathValue("id"), actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.enqueue(job.ID)
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleAutomations(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.store.ListAutomations(),
	})
}

func (s *Server) handleCreateAutomationTask(w http.ResponseWriter, r *http.Request) {
	var req domain.AutomationTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}

	result, err := s.store.CreateAutomationTask(req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.enqueue(result.Job.ID)
	writeJSON(w, http.StatusCreated, result)
}

func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("请求 JSON 无效：%w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "资源不存在")
	case errors.Is(err, store.ErrValidation):
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "CONFLICT", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "服务内部错误")
	}
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func actorFromRequest(r *http.Request) string {
	actor := strings.TrimSpace(r.Header.Get("X-Actor"))
	if actor == "" {
		return "admin"
	}
	return actor
}

func (s *Server) resolveOCIConfig(profileID string, region string) (oci.ReadinessConfig, domain.Profile, error) {
	if s.profileResolver != nil {
		return s.profileResolver.Resolve(profileID, region)
	}
	return s.ociReadiness, domain.Profile{
		ID:            "env-default",
		Name:          "ENV DEFAULT",
		TenancyOCID:   s.ociReadiness.TenancyOCID,
		UserOCID:      s.ociReadiness.UserOCID,
		Fingerprint:   s.ociReadiness.Fingerprint,
		DefaultRegion: s.ociReadiness.Region,
		Status:        "Configured",
	}, nil
}

func (s *Server) ociActionContext(r *http.Request) (string, string, string) {
	query := r.URL.Query()
	profileID := strings.TrimSpace(query.Get("profileId"))
	region := strings.TrimSpace(query.Get("region"))
	compartmentID := strings.TrimSpace(query.Get("compartmentId"))
	if instance, ok := s.store.GetInstance(r.PathValue("id")); ok {
		if profileID == "" {
			profileID = instance.ProfileID
		}
		if region == "" {
			region = instance.Region
		}
		if compartmentID == "" {
			compartmentID = instance.CompartmentID
		}
	}
	if profileID == "" {
		profileID = "DEFAULT"
	}
	if region == "" {
		region = s.ociReadiness.Region
	}
	if compartmentID == "" {
		compartmentID = s.ociReadiness.TenancyOCID
	}
	return profileID, region, compartmentID
}

func normalizeLaunchOptions(options *domain.LaunchOptions) {
	if options.Profiles == nil {
		options.Profiles = []domain.Profile{}
	}
	if options.Templates == nil {
		options.Templates = []domain.InstanceTemplate{}
	}
	if options.Regions == nil {
		options.Regions = []domain.LaunchOption{}
	}
	if options.Compartments == nil {
		options.Compartments = []domain.LaunchOption{}
	}
	if options.AvailabilityADs == nil {
		options.AvailabilityADs = []domain.LaunchOption{}
	}
	if options.Images == nil {
		options.Images = []domain.LaunchOption{}
	}
	if options.Shapes == nil {
		options.Shapes = []domain.ShapeOption{}
	}
	if options.VCNs == nil {
		options.VCNs = []domain.LaunchOption{}
	}
	if options.Subnets == nil {
		options.Subnets = []domain.LaunchOption{}
	}
	if options.ReservedIPs == nil {
		options.ReservedIPs = []domain.LaunchOption{}
	}
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || s.auth == nil || !s.auth.Enabled() || isPublicAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if !s.auth.IsAuthenticated(r) {
			writeError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "panel login required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isPublicAPIPath(path string) bool {
	return path == "/api/health" || strings.HasPrefix(path, "/api/auth/")
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Actor")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
