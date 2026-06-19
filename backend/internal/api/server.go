package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"a-series-oracle/backend/internal/auth"
	"a-series-oracle/backend/internal/domain"
	"a-series-oracle/backend/internal/lifecyclenotify"
	"a-series-oracle/backend/internal/notify"
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

type requestUserContextKey struct{}

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
	s.mux.HandleFunc("GET /api/account", s.handleAccountSettings)
	s.mux.HandleFunc("PUT /api/account/profile", s.handleUpdateAccountProfile)
	s.mux.HandleFunc("POST /api/account/password", s.handleUpdateAccountPassword)
	s.mux.HandleFunc("GET /api/settings/appearance", s.handleAppearanceSettings)
	s.mux.HandleFunc("PUT /api/settings/appearance", s.handleUpdateAppearanceSettings)
	s.mux.HandleFunc("GET /api/access-control", s.handleAccessControlSettings)
	s.mux.HandleFunc("PUT /api/access-control", s.handleUpdateAccessControlSettings)
	s.mux.HandleFunc("POST /api/access-control/users/{id}/password", s.handleSetAccessUserPassword)
	s.mux.HandleFunc("GET /api/security/guardrails", s.handleSecurityGuardrails)
	s.mux.HandleFunc("PUT /api/security/guardrails", s.handleUpdateSecurityGuardrails)
	s.mux.HandleFunc("GET /api/budget/settings", s.handleBudgetSettings)
	s.mux.HandleFunc("PUT /api/budget/settings", s.handleUpdateBudgetSettings)
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
	s.mux.HandleFunc("POST /api/templates", s.handleCreateTemplate)
	s.mux.HandleFunc("GET /api/templates/{id}", s.handleTemplate)
	s.mux.HandleFunc("PATCH /api/templates/{id}", s.handleUpdateTemplate)
	s.mux.HandleFunc("DELETE /api/templates/{id}", s.handleDeleteTemplate)
	s.mux.HandleFunc("POST /api/templates/{id}/validate", s.handleValidateTemplate)
	s.mux.HandleFunc("GET /api/launch-options", s.handleLaunchOptions)
	s.mux.HandleFunc("GET /api/instances", s.handleInstances)
	s.mux.HandleFunc("POST /api/instances", s.handleCreateInstance)
	s.mux.HandleFunc("GET /api/instances/{id}", s.handleInstance)
	s.mux.HandleFunc("POST /api/instances/{id}/actions", s.handleInstanceAction)
	s.mux.HandleFunc("POST /api/instances/{id}/reboot", s.handleRebootInstance)
	s.mux.HandleFunc("POST /api/instances/{id}/ip-tasks", s.handleCreateIPTask)
	s.mux.HandleFunc("POST /api/instances/{id}/system/reinstall", s.handleInstanceReinstall)
	s.mux.HandleFunc("GET /api/network/inventory", s.handleNetworkInventory)
	s.mux.HandleFunc("POST /api/network/public-ips/batch", s.handlePublicIPBatchTask)
	s.mux.HandleFunc("GET /api/jobs", s.handleJobs)
	s.mux.HandleFunc("POST /api/jobs/clear-completed", s.handleClearCompletedJobs)
	s.mux.HandleFunc("GET /api/jobs/{id}", s.handleJob)
	s.mux.HandleFunc("POST /api/jobs/{id}/cancel", s.handleCancelJob)
	s.mux.HandleFunc("POST /api/jobs/{id}/retry", s.handleRetryJob)
	s.mux.HandleFunc("GET /api/notifications", s.handleNotifications)
	s.mux.HandleFunc("POST /api/notifications/{id}/read", s.handleReadNotification)
	s.mux.HandleFunc("DELETE /api/notifications/{id}", s.handleDeleteNotification)
	s.mux.HandleFunc("POST /api/notifications/read-all", s.handleReadAllNotifications)
	s.mux.HandleFunc("GET /api/audit-logs", s.handleAuditLogs)
	s.mux.HandleFunc("GET /api/email/settings", s.handleEmailSettings)
	s.mux.HandleFunc("PUT /api/email/settings", s.handleUpdateEmailSettings)
	s.mux.HandleFunc("POST /api/email/test", s.handleEmailTest)
	s.mux.HandleFunc("GET /api/webhook/settings", s.handleWebhookSettings)
	s.mux.HandleFunc("PUT /api/webhook/settings", s.handleUpdateWebhookSettings)
	s.mux.HandleFunc("POST /api/webhook/test", s.handleWebhookTest)
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
	userID, _ := s.currentUserID(r)
	user, _ := s.store.GetAccessUser(userID)
	writeJSON(w, http.StatusOK, map[string]any{
		"authEnabled":   s.auth != nil && s.auth.Enabled(),
		"authenticated": s.auth == nil || s.auth.IsAuthenticated(r),
		"user":          user,
	})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
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
	username := strings.TrimSpace(req.Username)
	if username != "" {
		user, ok := s.store.VerifyAccessUser(username, req.Password)
		if !ok {
			writeError(w, http.StatusUnauthorized, "INVALID_PASSWORD", "invalid username or password")
			return
		}
		s.auth.IssueSessionFor(w, user.ID)
		writeJSON(w, http.StatusOK, map[string]any{
			"authEnabled":   true,
			"authenticated": true,
			"user":          user,
		})
		return
	}
	if !s.auth.VerifyPassword(req.Password) {
		writeError(w, http.StatusUnauthorized, "INVALID_PASSWORD", "invalid panel password")
		return
	}
	s.auth.IssueSessionFor(w, "admin")
	user, _ := s.store.GetAccessUser("admin")
	writeJSON(w, http.StatusOK, map[string]any{
		"authEnabled":   true,
		"authenticated": true,
		"user":          user,
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

func (s *Server) handleAccountSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.GetAccountSettings())
}

func (s *Server) handleUpdateAccountProfile(w http.ResponseWriter, r *http.Request) {
	var req domain.AccountProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	settings, err := s.store.SetAccountProfile(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleUpdateAccountPassword(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil || !s.auth.Enabled() {
		writeError(w, http.StatusConflict, "AUTH_DISABLED", "panel authentication is disabled; enable panel auth before changing password")
		return
	}
	var req domain.AccountPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if !s.auth.VerifyPassword(req.CurrentPassword) {
		writeError(w, http.StatusUnauthorized, "INVALID_CURRENT_PASSWORD", "current password is invalid")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_NEW_PASSWORD", err.Error())
		return
	}
	settings, err := s.store.SetAccountPasswordHash(hash)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.auth.SetPasswordHash(hash)
	s.auth.IssueSession(w)
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleAppearanceSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.GetAppearanceSettings())
}

func (s *Server) handleUpdateAppearanceSettings(w http.ResponseWriter, r *http.Request) {
	var req domain.AppearanceSettings
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	settings, err := s.store.SetAppearanceSettings(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleAccessControlSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.GetAccessControlSettings())
}

func (s *Server) handleUpdateAccessControlSettings(w http.ResponseWriter, r *http.Request) {
	var req domain.AccessControlSettings
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	settings, err := s.store.SetAccessControlSettings(req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleSetAccessUserPassword(w http.ResponseWriter, r *http.Request) {
	var req domain.AccessPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		req.UserID = r.PathValue("id")
	}
	settings, err := s.store.SetAccessUserPassword(req.UserID, req.NewPassword, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleSecurityGuardrails(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.GetSecurityGuardrailSettings())
}

func (s *Server) handleUpdateSecurityGuardrails(w http.ResponseWriter, r *http.Request) {
	var req domain.SecurityGuardrailSettings
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	settings, err := s.store.SetSecurityGuardrailSettings(req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleBudgetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.GetBudgetSettings())
}

func (s *Server) handleUpdateBudgetSettings(w http.ResponseWriter, r *http.Request) {
	var req domain.BudgetSettings
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	settings, err := s.store.SetBudgetSettings(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
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
	cfg, _, err := s.resolveOCIConfig("", "")
	if err != nil {
		writeError(w, http.StatusBadRequest, "OCI_PROFILE_RESOLVE_FAILED", err.Error())
		return
	}
	result := oci.SmokeCreateDeleteE2Micro(r.Context(), cfg, req)
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
	cfg, _, err := s.resolveOCIConfig("", "")
	if err != nil {
		writeError(w, http.StatusBadRequest, "OCI_PROFILE_RESOLVE_FAILED", err.Error())
		return
	}
	result := oci.SmokeE3FlexLifecycle(r.Context(), cfg, req)
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
	cfg, _, err := s.resolveOCIConfig("", "")
	if err != nil {
		writeError(w, http.StatusBadRequest, "OCI_PROFILE_RESOLVE_FAILED", err.Error())
		return
	}
	result := oci.CleanupSmokeInstances(r.Context(), cfg, req)
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
	cfg, _, err := s.resolveOCIConfig("", "")
	if err != nil {
		writeError(w, http.StatusBadRequest, "OCI_PROFILE_RESOLVE_FAILED", err.Error())
		return
	}
	result := oci.SmokeReinstallInstance(r.Context(), cfg, req)
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
	query := r.URL.Query()
	items := s.store.ListTemplatesFiltered(domain.TemplateFilter{
		ProfileID:        query.Get("profileId"),
		Region:           query.Get("region"),
		CompartmentID:    query.Get("compartmentId"),
		Status:           query.Get("status"),
		ValidationStatus: query.Get("validationStatus"),
		Query:            query.Get("q"),
		Limit:            parsePositiveInt(query.Get("limit")),
		IncludeDeleted:   strings.EqualFold(query.Get("includeDeleted"), "true") || query.Get("includeDeleted") == "1",
	})
	for i := range items {
		items[i] = redactTemplate(items[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	template, err := s.store.CreateTemplate(req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, redactTemplate(template))
}

func (s *Server) handleTemplate(w http.ResponseWriter, r *http.Request) {
	template, ok := s.store.GetTemplate(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", "template not found")
		return
	}
	writeJSON(w, http.StatusOK, redactTemplate(template))
}

func (s *Server) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	template, err := s.store.UpdateTemplate(r.PathValue("id"), req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, redactTemplate(template))
}

func (s *Server) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteTemplate(r.PathValue("id"), actorFromRequest(r)); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleValidateTemplate(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.ValidateTemplate(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
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

func (s *Server) handleNetworkInventory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	profileID := query.Get("profileId")
	region := query.Get("region")
	if s.executionMode != "oci" {
		writeJSON(w, http.StatusOK, domain.NetworkInventory{
			Verified:      false,
			ExecutionMode: s.executionMode,
			ProfileID:     profileID,
			Region:        region,
			CompartmentID: query.Get("compartmentId"),
			ErrorCode:     "OCI_EXECUTION_MODE_REQUIRED",
			ErrorMessage:  "network inventory requires OCI execution mode and a valid OCI profile",
			LastSyncedAt:  time.Now().UTC(),
		})
		return
	}
	cfg, profile, err := s.resolveOCIConfig(profileID, region)
	if err != nil {
		writeJSON(w, http.StatusOK, domain.NetworkInventory{
			Verified:      false,
			ExecutionMode: s.executionMode,
			ProfileID:     profileID,
			Region:        region,
			CompartmentID: query.Get("compartmentId"),
			ErrorCode:     "PROFILE_RESOLVE_FAILED",
			ErrorMessage:  err.Error(),
			LastSyncedAt:  time.Now().UTC(),
		})
		return
	}
	result := oci.DiscoverNetworkInventory(r.Context(), cfg, domain.NetworkInventoryRequest{
		ProfileID:     profile.ID,
		Region:        cfg.Region,
		CompartmentID: query.Get("compartmentId"),
		VCNID:         query.Get("vcnId"),
	})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePublicIPBatchTask(w http.ResponseWriter, r *http.Request) {
	var req domain.PublicIPBatchTaskRequest
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
		req.Region = cfg.Region
		if strings.TrimSpace(req.CompartmentID) == "" {
			req.CompartmentID = cfg.TenancyOCID
		}
	}
	if err := s.enforcePublicIPBatchGuardrails(req); err != nil {
		writeError(w, http.StatusForbidden, "GUARDRAIL_BLOCKED", err.Error())
		return
	}
	job, err := s.store.CreatePublicIPBatchTask(req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.enqueue(job.ID)
	writeJSON(w, http.StatusAccepted, sanitizeJob(job))
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
		if err := s.enforceCreateInstanceGuardrails(req); err != nil {
			writeError(w, http.StatusForbidden, "GUARDRAIL_BLOCKED", err.Error())
			return
		}
		generatedRootPassword := ""
		if req.GenerateRootPassword && strings.EqualFold(strings.TrimSpace(req.CompartmentID), strings.TrimSpace(cfg.TenancyOCID)) {
			password, err := generateRootPassword()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "ROOT_PASSWORD_GENERATION_FAILED", err.Error())
				return
			}
			generatedRootPassword = password
			req.CloudInit = mergeRootPasswordCloudInit(req.CloudInit, password)
			req.NotifyRootPassword = true
		}
		job, err := s.store.CreateOCIInstanceLaunchTask(req, actorFromRequest(r))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if generatedRootPassword != "" {
			_ = s.createNotification(r.Context(), domain.NotificationRequest{
				Title:          "Root password generated: " + req.Name,
				Message:        rootPasswordNotificationMessage(req, job.ID, generatedRootPassword),
				Severity:       domain.NotificationWarning,
				Category:       "credential",
				ResourceType:   "instance",
				ResourceID:     job.ResourceID,
				ProfileID:      job.ProfileID,
				Region:         job.Region,
				CompartmentID:  job.CompartmentID,
				Sensitive:      true,
				EmailRequested: req.NotifyRootPassword,
			}, actorFromRequest(r))
		}
		s.enqueue(job.ID)
		writeJSON(w, http.StatusAccepted, sanitizeJob(job))
		return
	}

	if err := s.enforceCreateInstanceGuardrails(req); err != nil {
		writeError(w, http.StatusForbidden, "GUARDRAIL_BLOCKED", err.Error())
		return
	}
	result, err := s.store.CreateInstanceTask(req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.enqueue(result.Job.ID)
	result.Job = sanitizeJob(result.Job)
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
	if err := s.authorizeInstanceAction(r, req); err != nil {
		writeError(w, http.StatusForbidden, "PERMISSION_DENIED", err.Error())
		return
	}
	if err := s.enforceInstanceActionGuardrails(req); err != nil {
		writeError(w, http.StatusForbidden, "GUARDRAIL_BLOCKED", err.Error())
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
		writeJSON(w, http.StatusAccepted, sanitizeJob(job))
		return
	}

	job, err := s.store.CreateInstanceActionTask(r.PathValue("id"), req, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.enqueue(job.ID)
	writeJSON(w, http.StatusAccepted, sanitizeJob(job))
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

func (s *Server) handleInstanceReinstall(w http.ResponseWriter, r *http.Request) {
	var req domain.InstanceReinstallRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if err := s.authorizeInstanceSystemSettings(r); err != nil {
		writeError(w, http.StatusForbidden, "PERMISSION_DENIED", err.Error())
		return
	}
	if err := s.enforceInstanceReinstallGuardrails(req); err != nil {
		writeError(w, http.StatusForbidden, "GUARDRAIL_BLOCKED", err.Error())
		return
	}
	if s.executionMode != "oci" {
		writeError(w, http.StatusConflict, "OCI_EXECUTION_REQUIRED", "重装系统必须在真实 OCI 执行模式下运行；local executor 不会冒充重装成功")
		return
	}
	if req.GenerateRootPassword || strings.TrimSpace(req.CloudInit) != "" || strings.TrimSpace(req.SSHAuthorizedKey) != "" {
		writeError(w, http.StatusBadRequest, "REINSTALL_PASSWORD_INJECTION_UNSUPPORTED", "OCI UpdateInstance 重装路径不能修改已启动实例的 user_data 或 ssh_authorized_keys，因此当前版本不支持重装时注入 root 密码、cloud-init 或 SSH key")
		return
	}

	profileID, region, compartmentID := s.ociActionContext(r)
	if strings.TrimSpace(req.ProfileID) != "" {
		profileID = req.ProfileID
	}
	if strings.TrimSpace(req.Region) != "" {
		region = req.Region
	}
	if strings.TrimSpace(req.CompartmentID) != "" {
		compartmentID = req.CompartmentID
	}
	if _, _, err := s.resolveOCIConfig(profileID, region); err != nil {
		writeError(w, http.StatusBadRequest, "OCI_PROFILE_RESOLVE_FAILED", err.Error())
		return
	}

	job, err := s.store.CreateOCIInstanceReinstallTask(r.PathValue("id"), req, actorFromRequest(r), profileID, region, compartmentID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	lifecyclenotify.SendReinstallNotification(r.Context(), s.store, lifecyclenotify.ReinstallRequested, job, nil)
	s.enqueue(job.ID)
	writeJSON(w, http.StatusAccepted, sanitizeJob(job))
}

func (s *Server) handleCreateIPTask(w http.ResponseWriter, r *http.Request) {
	var req domain.IPTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if err := s.enforceIPGuardrails(req); err != nil {
		writeError(w, http.StatusForbidden, "GUARDRAIL_BLOCKED", err.Error())
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
	jobs := s.store.ListJobs()
	for i := range jobs {
		jobs[i] = sanitizeJob(jobs[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": jobs,
	})
}

func (s *Server) handleClearCompletedJobs(w http.ResponseWriter, r *http.Request) {
	deletedCount, err := s.store.ClearCompletedJobs(actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	jobs := s.store.ListJobs()
	for i := range jobs {
		jobs[i] = sanitizeJob(jobs[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deletedCount": deletedCount,
		"items":        jobs,
	})
}

func (s *Server) handleJob(w http.ResponseWriter, r *http.Request) {
	job, ok := s.store.GetJob(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "JOB_NOT_FOUND", "job not found")
		return
	}
	writeJSON(w, http.StatusOK, sanitizeJob(job))
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.store.CancelJob(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sanitizeJob(job))
}

func (s *Server) handleRetryJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.store.RetryJob(r.PathValue("id"), actorFromRequest(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.enqueue(job.ID)
	writeJSON(w, http.StatusAccepted, sanitizeJob(job))
}

func (s *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	unreadOnly := strings.EqualFold(r.URL.Query().Get("unread"), "true") || r.URL.Query().Get("unread") == "1"
	items := s.store.ListNotifications(unreadOnly)
	unread := 0
	for _, item := range s.store.ListNotifications(true) {
		if !item.Read {
			unread++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"unreadCount": unread,
	})
}

func (s *Server) handleReadNotification(w http.ResponseWriter, r *http.Request) {
	notification, err := s.store.MarkNotificationRead(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, notification)
}

func (s *Server) handleDeleteNotification(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteNotification(r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReadAllNotifications(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.store.MarkAllNotificationsRead(),
	})
}

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := domain.AuditLogFilter{
		Actor:            query.Get("actor"),
		Action:           query.Get("action"),
		ResourceType:     query.Get("resourceType"),
		ResourceID:       query.Get("resourceId"),
		ProfileID:        query.Get("profileId"),
		Region:           query.Get("region"),
		CompartmentID:    query.Get("compartmentId"),
		OCIRequestID:     query.Get("ociRequestId"),
		OCIWorkRequestID: query.Get("ociWorkRequestId"),
		Status:           query.Get("status"),
		Limit:            parsePositiveInt(query.Get("limit")),
	}
	items, err := s.store.ListAuditLogs(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AUDIT_LOG_LIST_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleEmailSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.GetEmailSettings())
}

func (s *Server) handleUpdateEmailSettings(w http.ResponseWriter, r *http.Request) {
	var req domain.EmailSettings
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	settings, err := s.store.SetEmailSettings(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "EMAIL_SETTINGS_SAVE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleEmailTest(w http.ResponseWriter, r *http.Request) {
	var req domain.EmailTestRequest
	if r.Body != nil {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
			return
		}
	}
	settings := s.store.GetEmailSettingsForSend()
	if !settings.Enabled {
		writeJSON(w, http.StatusBadGateway, domain.EmailTestResult{Verified: false, Message: "email delivery is disabled"})
		return
	}
	if strings.TrimSpace(req.To) != "" {
		settings.To = []string{strings.TrimSpace(req.To)}
	}
	if err := notify.SendEmail(r.Context(), settings, "OCI Lifecycle Platform test email", "This is a test email from OCI Lifecycle Platform."); err != nil {
		writeJSON(w, http.StatusBadGateway, domain.EmailTestResult{Verified: false, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, domain.EmailTestResult{Verified: true, Message: "test email sent"})
}

func (s *Server) handleWebhookSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.GetWebhookSettings())
}

func (s *Server) handleUpdateWebhookSettings(w http.ResponseWriter, r *http.Request) {
	var req domain.WebhookSettings
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	settings, err := s.store.SetWebhookSettings(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WEBHOOK_SETTINGS_SAVE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleWebhookTest(w http.ResponseWriter, r *http.Request) {
	settings := s.store.GetWebhookSettingsForSend()
	if !settings.Enabled {
		writeJSON(w, http.StatusBadGateway, domain.WebhookTestResult{Verified: false, Message: "webhook delivery is disabled"})
		return
	}
	payload := notify.WebhookPayload{
		Event:     "webhook.test",
		Title:     "OCI Lifecycle Platform webhook test",
		Message:   "This is a test webhook from OCI Lifecycle Platform.",
		Severity:  string(domain.NotificationInfo),
		CreatedAt: time.Now().UTC(),
	}
	if err := notify.SendWebhook(r.Context(), settings, payload); err != nil {
		writeJSON(w, http.StatusBadGateway, domain.WebhookTestResult{Verified: false, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, domain.WebhookTestResult{Verified: true, Message: "test webhook sent"})
}

func (s *Server) createNotification(ctx context.Context, req domain.NotificationRequest, actor string) domain.Notification {
	notification, err := s.store.CreateNotification(req, actor)
	if err != nil {
		return domain.Notification{}
	}
	notification = s.deliverEmailNotification(ctx, notification)
	notification = s.deliverWebhookNotification(ctx, notification)
	return notification
}

func (s *Server) deliverEmailNotification(ctx context.Context, notification domain.Notification) domain.Notification {
	if !notification.EmailRequested {
		return notification
	}
	settings := s.store.GetEmailSettingsForSend()
	if !settings.Enabled {
		updated, _ := s.store.UpdateNotificationEmailStatus(notification.ID, false, "email delivery is disabled")
		return updated
	}
	if err := notify.SendEmail(ctx, settings, notification.Title, notification.Message); err != nil {
		updated, _ := s.store.UpdateNotificationEmailStatus(notification.ID, false, err.Error())
		return updated
	}
	updated, _ := s.store.UpdateNotificationEmailStatus(notification.ID, true, "")
	return updated
}

func (s *Server) deliverWebhookNotification(ctx context.Context, notification domain.Notification) domain.Notification {
	settings := s.store.GetWebhookSettingsForSend()
	if !settings.Enabled {
		return notification
	}
	message := notification.Message
	if notification.Sensitive {
		message = "Sensitive notification created in panel. Open the console to view the protected content."
	}
	payload := notify.WebhookPayload{
		Event:        "notification.created",
		Notification: notification,
		Title:        notification.Title,
		Message:      message,
		Severity:     string(notification.Severity),
		CreatedAt:    notification.CreatedAt,
	}
	if err := notify.SendWebhook(ctx, settings, payload); err != nil {
		updated, _ := s.store.UpdateNotificationWebhookStatus(notification.ID, false, err.Error())
		return updated
	}
	updated, _ := s.store.UpdateNotificationWebhookStatus(notification.ID, true, "")
	return updated
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
	if err := s.enforceAutomationGuardrails(req); err != nil {
		writeError(w, http.StatusForbidden, "GUARDRAIL_BLOCKED", err.Error())
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

func (s *Server) enforceCreateInstanceGuardrails(req domain.CreateInstanceRequest) error {
	settings := s.store.GetSecurityGuardrailSettings()
	if !settings.Enabled {
		return nil
	}
	if err := guardrailRegionAllowed(settings, req.Region); err != nil {
		return err
	}
	if settings.RequireTemplateForLaunch && strings.TrimSpace(req.TemplateID) == "" {
		return fmt.Errorf("安全护栏阻止创建：必须基于模板创建实例")
	}
	if settings.MaxOCPUsPerInstance > 0 && req.OCPUs > settings.MaxOCPUsPerInstance {
		return fmt.Errorf("安全护栏阻止创建：OCPU %d 超过上限 %d", req.OCPUs, settings.MaxOCPUsPerInstance)
	}
	if settings.MaxMemoryGBPerInstance > 0 && req.MemoryGB > settings.MaxMemoryGBPerInstance {
		return fmt.Errorf("安全护栏阻止创建：内存 %dGB 超过上限 %dGB", req.MemoryGB, settings.MaxMemoryGBPerInstance)
	}
	if settings.MaxBootVolumeGB > 0 && req.BootVolumeGB > settings.MaxBootVolumeGB {
		return fmt.Errorf("安全护栏阻止创建：启动盘 %dGB 超过上限 %dGB", req.BootVolumeGB, settings.MaxBootVolumeGB)
	}
	if settings.MaxRetryAttempts > 0 && req.RetryMaxAttempts > settings.MaxRetryAttempts {
		return fmt.Errorf("安全护栏阻止创建：重试次数 %d 超过上限 %d", req.RetryMaxAttempts, settings.MaxRetryAttempts)
	}
	if settings.BlockRootPasswordWithoutEmail && req.GenerateRootPassword && !req.NotifyRootPassword {
		return fmt.Errorf("安全护栏阻止创建：生成 root 密码时必须启用通知")
	}
	return nil
}

func (s *Server) enforceInstanceActionGuardrails(req domain.InstanceActionRequest) error {
	settings := s.store.GetSecurityGuardrailSettings()
	if !settings.Enabled {
		return nil
	}
	if req.Action == domain.InstanceActionTerminate {
		if settings.RequireApprovalForTerminate && !req.SnapshotBefore {
			return fmt.Errorf("安全护栏阻止终止：需要先勾选快照/审批确认")
		}
		if settings.BlockBootVolumeDeletion && !req.PreserveBootVolume {
			return fmt.Errorf("安全护栏阻止终止：当前策略禁止删除启动盘")
		}
	}
	if req.Action == domain.InstanceActionResize {
		if settings.MaxOCPUsPerInstance > 0 && req.TargetOCPUs > settings.MaxOCPUsPerInstance {
			return fmt.Errorf("安全护栏阻止升降级：目标 OCPU %d 超过上限 %d", req.TargetOCPUs, settings.MaxOCPUsPerInstance)
		}
		if settings.MaxMemoryGBPerInstance > 0 && req.TargetMemoryGB > settings.MaxMemoryGBPerInstance {
			return fmt.Errorf("安全护栏阻止升降级：目标内存 %dGB 超过上限 %dGB", req.TargetMemoryGB, settings.MaxMemoryGBPerInstance)
		}
		if settings.MaxBootVolumeGB > 0 && req.TargetBootVolumeGB > settings.MaxBootVolumeGB {
			return fmt.Errorf("安全护栏阻止升降级：目标启动盘 %dGB 超过上限 %dGB", req.TargetBootVolumeGB, settings.MaxBootVolumeGB)
		}
	}
	return nil
}

func (s *Server) enforceInstanceReinstallGuardrails(req domain.InstanceReinstallRequest) error {
	settings := s.store.GetSecurityGuardrailSettings()
	if !settings.Enabled {
		return nil
	}
	if err := guardrailRegionAllowed(settings, req.Region); err != nil {
		return err
	}
	if settings.MaxBootVolumeGB > 0 && req.BootVolumeSizeGB > settings.MaxBootVolumeGB {
		return fmt.Errorf("安全护栏阻止重装：目标启动盘 %dGB 超过上限 %dGB", req.BootVolumeSizeGB, settings.MaxBootVolumeGB)
	}
	if settings.BlockRootPasswordWithoutEmail && req.GenerateRootPassword && !req.NotifyPasswordInApp && !req.NotifyPasswordByEmail {
		return fmt.Errorf("安全护栏阻止重装：生成 root 密码时必须启用通知")
	}
	return nil
}

func (s *Server) enforceIPGuardrails(req domain.IPTaskRequest) error {
	settings := s.store.GetSecurityGuardrailSettings()
	if !settings.Enabled {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(req.NetworkChangeMode))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(req.IPv6Strategy))
	}
	if settings.BlockPublicIPv6RouteChanges && (req.AutoConfigureIPv6 || req.AllowIrreversibleVCNIPv6 || mode == "replace_public_path" || mode == "clone_route_table" || mode == "additive") {
		return fmt.Errorf("安全护栏阻止 IPv6 网络改造：当前策略禁止自动修改 VCN/Subnet/Route/Security")
	}
	return nil
}

func (s *Server) enforcePublicIPBatchGuardrails(req domain.PublicIPBatchTaskRequest) error {
	settings := s.store.GetSecurityGuardrailSettings()
	if !settings.Enabled {
		return nil
	}
	if err := guardrailRegionAllowed(settings, req.Region); err != nil {
		return err
	}
	count := req.Count
	if strings.EqualFold(req.Action, "delete") {
		count = len(req.PublicIPIDs)
	}
	if settings.MaxPublicIPBatchCount > 0 && count > settings.MaxPublicIPBatchCount {
		return fmt.Errorf("安全护栏阻止批量公网 IP：数量 %d 超过上限 %d", count, settings.MaxPublicIPBatchCount)
	}
	return nil
}

func (s *Server) enforceAutomationGuardrails(req domain.AutomationTaskRequest) error {
	settings := s.store.GetSecurityGuardrailSettings()
	if !settings.Enabled {
		return nil
	}
	if settings.MaxRetryAttempts > 0 && req.MaxRetries > settings.MaxRetryAttempts {
		return fmt.Errorf("安全护栏阻止自动化：重试次数 %d 超过上限 %d", req.MaxRetries, settings.MaxRetryAttempts)
	}
	return nil
}

func guardrailRegionAllowed(settings domain.SecurityGuardrailSettings, region string) error {
	region = strings.TrimSpace(region)
	for _, denied := range settings.DeniedRegions {
		if strings.EqualFold(denied, region) {
			return fmt.Errorf("安全护栏阻止操作：区域 %s 已被禁止", region)
		}
	}
	if len(settings.AllowedRegions) == 0 || region == "" {
		return nil
	}
	for _, allowed := range settings.AllowedRegions {
		if strings.EqualFold(allowed, region) {
			return nil
		}
	}
	return fmt.Errorf("安全护栏阻止操作：区域 %s 不在允许列表", region)
}

func (s *Server) authorizeInstanceAction(r *http.Request, req domain.InstanceActionRequest) error {
	permission := "instance:operate"
	if req.Action == domain.InstanceActionTerminate || req.Action == domain.InstanceActionResize {
		permission = "instance:write"
	}
	userID, _ := s.currentUserID(r)
	profileID, region, compartmentID := s.ociActionContext(r)
	return s.store.Authorize(userID, permission, profileID, region, compartmentID)
}

func (s *Server) authorizeInstanceSystemSettings(r *http.Request) error {
	userID, _ := s.currentUserID(r)
	profileID, region, compartmentID := s.ociActionContext(r)
	return s.store.Authorize(userID, "instance:write", profileID, region, compartmentID)
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

func generateRootPassword() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func mergeRootPasswordCloudInit(existing string, password string) string {
	rootConfig := fmt.Sprintf(`#cloud-config
disable_root: false
ssh_pwauth: true
chpasswd:
  expire: false
  list: |
    root:%s
runcmd:
  - [ sh, -c, "sed -i 's/^#\\?PermitRootLogin .*/PermitRootLogin yes/' /etc/ssh/sshd_config || true" ]
  - [ sh, -c, "sed -i 's/^#\\?PasswordAuthentication .*/PasswordAuthentication yes/' /etc/ssh/sshd_config || true" ]
  - [ sh, -c, "systemctl reload sshd || systemctl reload ssh || true" ]
`, password)
	existing = strings.TrimSpace(existing)
	if existing == "" {
		return rootConfig
	}
	return rootConfig + "\n\n# User supplied cloud-init follows.\n" + existing + "\n"
}

func rootPasswordNotificationMessage(req domain.CreateInstanceRequest, jobID string, password string) string {
	return fmt.Sprintf(`A random root password was generated for an OCI instance launch task.

Instance: %s
Job: %s
Region: %s
Compartment: %s
Username: root
Password: %s

This password was generated because Root tenancy was selected and root password generation was enabled. Store it securely and rotate it after first login. SSH password login still depends on the image and cloud-init execution result.`, req.Name, jobID, req.Region, req.CompartmentID, password)
}

func sanitizeJob(job domain.Job) domain.Job {
	if job.Input == nil {
		return job
	}
	input := map[string]any{}
	for key, value := range job.Input {
		input[key] = value
	}
	if value, ok := input["cloudInitSensitive"].(bool); ok && value {
		delete(input, "cloudInit")
		input["cloudInitRedacted"] = true
	}
	job.Input = input
	return job
}

func redactTemplate(template domain.InstanceTemplate) domain.InstanceTemplate {
	if strings.TrimSpace(template.CloudInit) != "" {
		template.CloudInitSet = true
	}
	template.CloudInit = ""
	return template
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
	if value, ok := r.Context().Value(requestUserContextKey{}).(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return "admin"
}

func parsePositiveInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
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
	if options.ShapeImages == nil {
		options.ShapeImages = map[string][]domain.LaunchOption{}
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
		userID, ok := s.auth.Subject(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "panel login required")
			return
		}
		permission, profileID, region, compartmentID := s.permissionContext(r)
		if permission != "" {
			if err := s.store.Authorize(userID, permission, profileID, region, compartmentID); err != nil {
				writeError(w, http.StatusForbidden, "PERMISSION_DENIED", err.Error())
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestUserContextKey{}, userID)))
	})
}

func (s *Server) currentUserID(r *http.Request) (string, bool) {
	if value, ok := r.Context().Value(requestUserContextKey{}).(string); ok && strings.TrimSpace(value) != "" {
		return value, true
	}
	if s.auth == nil {
		return "admin", true
	}
	return s.auth.Subject(r)
}

func (s *Server) permissionContext(r *http.Request) (string, string, string, string) {
	path := r.URL.Path
	method := r.Method
	profileID, region, compartmentID := s.ociActionContext(r)
	switch {
	case path == "/api/auth/me", path == "/api/account":
		return "", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/access-control"):
		if method == http.MethodGet {
			return "user:read", profileID, region, compartmentID
		}
		return "user:write", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/security/guardrails"):
		if method == http.MethodGet {
			return "guardrail:read", profileID, region, compartmentID
		}
		return "guardrail:write", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/audit-logs"):
		return "audit:read", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/profiles"):
		if method == http.MethodGet {
			return "profile:read", profileID, region, compartmentID
		}
		return "profile:write", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/templates"):
		if method == http.MethodGet {
			return "template:read", profileID, region, compartmentID
		}
		return "template:write", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/instances"):
		if method == http.MethodGet {
			return "instance:read", profileID, region, compartmentID
		}
		if strings.Contains(path, "/actions") || strings.Contains(path, "/reboot") {
			return "instance:operate", profileID, region, compartmentID
		}
		return "instance:write", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/network"):
		if method == http.MethodGet {
			return "network:read", profileID, region, compartmentID
		}
		return "network:write", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/jobs"):
		if method == http.MethodGet {
			return "job:read", profileID, region, compartmentID
		}
		return "job:write", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/notifications"):
		if method == http.MethodGet {
			return "notification:read", profileID, region, compartmentID
		}
		return "notification:write", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/email"), strings.HasPrefix(path, "/api/webhook"), strings.HasPrefix(path, "/api/settings"), strings.HasPrefix(path, "/api/account/password"):
		return "settings:write", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/budget"):
		if method == http.MethodGet {
			return "budget:read", profileID, region, compartmentID
		}
		return "budget:write", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/automations"):
		if method == http.MethodGet {
			return "automation:read", profileID, region, compartmentID
		}
		return "automation:write", profileID, region, compartmentID
	case strings.HasPrefix(path, "/api/oci/"):
		return "settings:write", profileID, region, compartmentID
	default:
		return "", profileID, region, compartmentID
	}
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
