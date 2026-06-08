package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"a-series-oracle/backend/internal/auth"
	"a-series-oracle/backend/internal/domain"
	"a-series-oracle/backend/internal/oci"
	"a-series-oracle/backend/internal/store"
)

type apiTestSink struct{}

func (apiTestSink) SaveProfile(domain.Profile, domain.ProfileSecret) error { return nil }
func (apiTestSink) SaveJob(domain.Job) error                               { return nil }
func (apiTestSink) SaveInstance(domain.Instance) error                     { return nil }
func (apiTestSink) RecordAudit(domain.AuditLog) error                      { return nil }

func TestHealth(t *testing.T) {
	ts := newTestServer()
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	ts.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var body map[string]any
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Fatalf("expected ok status, got %#v", body["status"])
	}
	if body["ociApiVerified"] != false {
		t.Fatalf("expected OCI API to be unverified in local tests, got %#v", body["ociApiVerified"])
	}
}

func TestPanelAuthProtectsAPIs(t *testing.T) {
	hash, err := auth.HashPassword("secret-password")
	if err != nil {
		t.Fatal(err)
	}
	authManager, err := auth.New(auth.Config{
		PasswordHash:  hash,
		SessionSecret: "test-session-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := NewServerWithOptions(store.NewSeeded(), ServerOptions{Auth: authManager}).Handler()

	health := httptest.NewRecorder()
	ts.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("expected health to stay public, got %d", health.Code)
	}

	blocked := httptest.NewRecorder()
	ts.ServeHTTP(blocked, httptest.NewRequest(http.MethodGet, "/api/instances", nil))
	if blocked.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated API call to be blocked, got %d", blocked.Code)
	}

	loginRes := postJSON(t, ts, "/api/auth/login", map[string]string{"password": "secret-password"})
	if loginRes.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d body=%s", loginRes.Code, loginRes.Body.String())
	}
	cookies := loginRes.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected login cookie")
	}

	allowed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/instances", nil)
	req.AddCookie(cookies[0])
	ts.ServeHTTP(allowed, req)
	if allowed.Code != http.StatusOK {
		t.Fatalf("expected authenticated API call to pass, got %d body=%s", allowed.Code, allowed.Body.String())
	}
}

func TestOCIReadiness(t *testing.T) {
	ts := newTestServer()
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/oci/readiness", nil)

	ts.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var body struct {
		Ready bool `json:"ready"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.Ready {
		t.Fatal("local test server must not report real OCI readiness")
	}
}

func TestListInstances(t *testing.T) {
	ts := newTestServer()
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/instances?status=Running", nil)

	ts.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var body struct {
		Items []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 running instances, got %d", len(body.Items))
	}
	for _, item := range body.Items {
		if item.Status != "Running" {
			t.Fatalf("expected Running status, got %s", item.Status)
		}
	}
}

func TestListTemplates(t *testing.T) {
	ts := newTestServer()
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)

	ts.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var body struct {
		Items []struct {
			ID      string `json:"id"`
			ImageID string `json:"imageId"`
			Shape   string `json:"shape"`
			Status  string `json:"status"`
		} `json:"items"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if len(body.Items) == 0 {
		t.Fatal("expected templates")
	}
	for _, item := range body.Items {
		if item.ID == "" || item.ImageID == "" || item.Shape == "" || item.Status != "Active" {
			t.Fatalf("unexpected template: %#v", item)
		}
	}
}

func TestLaunchOptions(t *testing.T) {
	ts := newTestServer()
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/launch-options", nil)

	ts.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var body struct {
		Templates []any `json:"templates"`
		Shapes    []any `json:"shapes"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if len(body.Templates) == 0 || len(body.Shapes) == 0 {
		t.Fatalf("expected launch options, got %#v", body)
	}
}

func TestCreateProfile(t *testing.T) {
	s := store.New()
	s.SetPersistenceSink(apiTestSink{})
	ts := NewServer(s).Handler()
	res := postJSON(t, ts, "/api/profiles", map[string]any{
		"name":           "primary",
		"tenancyOcid":    "ocid1.tenancy.oc1..example",
		"userOcid":       "ocid1.user.oc1..example",
		"fingerprint":    "11:22:33",
		"defaultRegion":  "ap-chuncheon-1",
		"privateKeyFile": "E:\\keys\\oci.pem",
	})

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", res.Code, res.Body.String())
	}
	var body map[string]any
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body["name"] != "primary" || body["privateKey"] != nil {
		t.Fatalf("unexpected profile response: %#v", body)
	}
}

func TestProfileDetailEnableDisableDelete(t *testing.T) {
	s := store.New()
	ts := NewServer(s).Handler()
	createRes := postJSON(t, ts, "/api/profiles", map[string]any{
		"name":          "DEFAULT",
		"tenancyOcid":   "ocid1.tenancy.oc1..example",
		"userOcid":      "ocid1.user.oc1..example",
		"fingerprint":   "11:22:33",
		"defaultRegion": "ap-chuncheon-1",
	})
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRes.Code, createRes.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	decodeTestJSON(t, createRes.Body.Bytes(), &created)

	detail := httptest.NewRecorder()
	ts.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/api/profiles/"+created.ID, nil))
	if detail.Code != http.StatusOK {
		t.Fatalf("expected detail 200, got %d body=%s", detail.Code, detail.Body.String())
	}

	disableRes := postJSON(t, ts, "/api/profiles/"+created.ID+"/disable", map[string]any{})
	if disableRes.Code != http.StatusOK {
		t.Fatalf("expected disable 200, got %d body=%s", disableRes.Code, disableRes.Body.String())
	}
	var disabled struct {
		Status string `json:"status"`
	}
	decodeTestJSON(t, disableRes.Body.Bytes(), &disabled)
	if disabled.Status != "Disabled" {
		t.Fatalf("expected Disabled, got %#v", disabled)
	}

	enableRes := postJSON(t, ts, "/api/profiles/"+created.ID+"/enable", map[string]any{})
	if enableRes.Code != http.StatusOK {
		t.Fatalf("expected enable 200, got %d body=%s", enableRes.Code, enableRes.Body.String())
	}

	deleteRes := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/profiles/"+created.ID, nil)
	ts.ServeHTTP(deleteRes, req)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("expected delete 204, got %d body=%s", deleteRes.Code, deleteRes.Body.String())
	}
}

func TestLaunchOptionsReturnsEmptyArraysWithoutSeedData(t *testing.T) {
	ts := NewServer(store.New()).Handler()
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/launch-options", nil)

	ts.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var body struct {
		Profiles        []any `json:"profiles"`
		Templates       []any `json:"templates"`
		Regions         []any `json:"regions"`
		Compartments    []any `json:"compartments"`
		AvailabilityADs []any `json:"availabilityAds"`
		Images          []any `json:"images"`
		Shapes          []any `json:"shapes"`
		VCNs            []any `json:"vcns"`
		Subnets         []any `json:"subnets"`
		ReservedIPs     []any `json:"reservedIps"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.Profiles == nil || body.Templates == nil || body.Regions == nil || body.Compartments == nil ||
		body.AvailabilityADs == nil || body.Images == nil || body.Shapes == nil || body.VCNs == nil ||
		body.Subnets == nil || body.ReservedIPs == nil {
		t.Fatalf("expected empty arrays instead of nulls, got %#v", body)
	}
}

func TestCreateIPTask(t *testing.T) {
	ts := newTestServer()
	payload := map[string]any{
		"mode":             "绑定保留公网 IP",
		"reservedPublicIp": "reserved-prod-01",
		"dnsLabel":         "prod-web-server-01",
		"vnicId":           "primary",
		"note":             "UI test",
		"enableIpv6":       false,
		"snapshotBefore":   true,
	}
	res := postJSON(t, ts, "/api/instances/inst-prod-web-01/ip-tasks", payload)

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		ID         string         `json:"id"`
		Type       string         `json:"type"`
		Status     string         `json:"status"`
		ResourceID string         `json:"resourceId"`
		Input      map[string]any `json:"input"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.Type != "IP 管理" || body.Status != "PENDING" {
		t.Fatalf("unexpected job response: %#v", body)
	}
	if body.ResourceID != "inst-prod-web-01" {
		t.Fatalf("expected instance resource id, got %s", body.ResourceID)
	}
	if body.Input["mode"] != "绑定保留公网 IP" {
		t.Fatalf("expected mode in input, got %#v", body.Input["mode"])
	}
}

func TestCreateInstance(t *testing.T) {
	ts := newTestServer()
	payload := map[string]any{
		"name":           "api-created-01",
		"profileId":      "profile-default",
		"region":         "ap-singapore-1",
		"compartment":    "development",
		"shape":          "VM.Standard.A1.Flex",
		"ocpus":          1,
		"memoryGb":       6,
		"bootVolumeGb":   80,
		"assignPublicIp": true,
		"subnetId":       "subnet-public-a",
		"maxRetries":     2,
	}
	res := postJSON(t, ts, "/api/instances", payload)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		Instance struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"instance"`
		Job struct {
			Type       string `json:"type"`
			Status     string `json:"status"`
			ResourceID string `json:"resourceId"`
		} `json:"job"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.Instance.ID == "" || body.Instance.Name != "api-created-01" || body.Instance.Status != "Provisioning" {
		t.Fatalf("unexpected instance response: %#v", body.Instance)
	}
	if body.Job.Type != "创建实例" || body.Job.Status != "PENDING" || body.Job.ResourceID != body.Instance.ID {
		t.Fatalf("unexpected job response: %#v", body.Job)
	}
}

func TestCreateInstanceAction(t *testing.T) {
	ts := newTestServer()
	res := postJSON(t, ts, "/api/instances/inst-prod-web-01/actions", map[string]any{
		"action":             "STOP",
		"graceful":           true,
		"preserveBootVolume": true,
		"snapshotBefore":     true,
	})

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		Type       string         `json:"type"`
		Status     string         `json:"status"`
		ResourceID string         `json:"resourceId"`
		Input      map[string]any `json:"input"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.Type != "停止实例" || body.Status != "PENDING" || body.ResourceID != "inst-prod-web-01" {
		t.Fatalf("unexpected action job: %#v", body)
	}
	if body.Input["action"] != "STOP" {
		t.Fatalf("expected action input, got %#v", body.Input)
	}
}

func TestCreateOCIInstanceActionUsesOCIDResource(t *testing.T) {
	queued := ""
	server := NewServerWithOptions(store.NewSeeded(), ServerOptions{
		ExecutionMode: "oci",
		Enqueue: func(jobID string) {
			queued = jobID
		},
		OCIReadiness: oci.ReadinessConfig{
			ExecutionMode: "oci",
			TenancyOCID:   "ocid1.tenancy.oc1..example",
			Region:        "ap-chuncheon-1",
		},
	}).Handler()
	instanceID := "ocid1.instance.oc1.ap-chuncheon-1.example"
	res := postJSON(t, server, "/api/instances/"+instanceID+"/actions", map[string]any{
		"action":             "STOP",
		"graceful":           true,
		"preserveBootVolume": true,
		"snapshotBefore":     true,
	})

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		ID         string         `json:"id"`
		ResourceID string         `json:"resourceId"`
		Input      map[string]any `json:"input"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.ResourceID != instanceID || body.Input["ociInstanceId"] != instanceID {
		t.Fatalf("unexpected OCI action job: %#v", body)
	}
	if queued != body.ID {
		t.Fatalf("expected job to be enqueued, queued=%s body=%s", queued, body.ID)
	}
}

func TestOCIModeCreateIPTaskCreatesIPv6Job(t *testing.T) {
	queued := ""
	server := NewServerWithOptions(store.NewSeeded(), ServerOptions{
		ExecutionMode: "oci",
		Enqueue: func(jobID string) {
			queued = jobID
		},
		OCIReadiness: oci.ReadinessConfig{
			ExecutionMode: "oci",
			TenancyOCID:   "ocid1.tenancy.oc1..example",
			Region:        "ap-chuncheon-1",
		},
	}).Handler()
	instanceID := "ocid1.instance.oc1.ap-chuncheon-1.example"
	res := postJSON(t, server, "/api/instances/"+instanceID+"/ip-tasks", map[string]any{
		"mode":           "enable-ipv6",
		"vnicId":         "primary",
		"enableIpv6":     true,
		"snapshotBefore": true,
	})

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		ID         string         `json:"id"`
		ResourceID string         `json:"resourceId"`
		Input      map[string]any `json:"input"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.ResourceID != instanceID || body.Input["operation"] != "ip-management" || body.Input["enableIpv6"] != true {
		t.Fatalf("unexpected OCI IP job: %#v", body)
	}
	if queued != body.ID {
		t.Fatalf("expected job to be enqueued, queued=%s body=%s", queued, body.ID)
	}
}

func TestOCIModeCreateInstanceCreatesLaunchJobWithoutPlaceholder(t *testing.T) {
	queued := ""
	server := NewServerWithOptions(store.NewSeeded(), ServerOptions{
		ExecutionMode: "oci",
		Enqueue: func(jobID string) {
			queued = jobID
		},
		OCIReadiness: oci.ReadinessConfig{
			ExecutionMode: "oci",
			TenancyOCID:   "ocid1.tenancy.oc1..example",
			Region:        "ap-chuncheon-1",
		},
	}).Handler()
	res := postJSON(t, server, "/api/instances", map[string]any{
		"name":         "must-not-create-placeholder",
		"shape":        "VM.Standard.E2.1.Micro",
		"ocpus":        1,
		"memoryGb":     1,
		"bootVolumeGb": 50,
	})

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		ID         string         `json:"id"`
		Status     string         `json:"status"`
		ResourceID string         `json:"resourceId"`
		Input      map[string]any `json:"input"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.Status != "PENDING" || body.ResourceID != "" || body.Input["operation"] != "launch" {
		t.Fatalf("unexpected launch job: %#v", body)
	}
	if queued != body.ID {
		t.Fatalf("expected launch job to be queued, queued=%s body=%s", queued, body.ID)
	}
}

func TestCreateAutomationTask(t *testing.T) {
	ts := newTestServer()
	payload := map[string]any{
		"name":              "A1 测试自动创建",
		"type":              "容量重试",
		"targetPool":        "a1-free-pool",
		"action":            "创建 1 台实例",
		"triggerInterval":   "每 5 分钟",
		"cooldown":          "30 分钟",
		"maxRetries":        10,
		"failurePolicy":     "达到上限后暂停并通知",
		"maxInstances":      4,
		"maxDailyRuns":      24,
		"regionScope":       "仅当前区域",
		"notifyChannel":     "邮件 + Webhook",
		"enableImmediately": true,
		"approvalRequired":  true,
	}
	res := postJSON(t, ts, "/api/automations/tasks", payload)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		Rule struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		} `json:"rule"`
		Job struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"job"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.Rule.ID == "" || body.Rule.Name != "A1 测试自动创建" || !body.Rule.Enabled {
		t.Fatalf("unexpected rule response: %#v", body.Rule)
	}
	if body.Job.Type != "添加自动化任务" || body.Job.Status != "PENDING" {
		t.Fatalf("unexpected job response: %#v", body.Job)
	}
}

func TestCreateAutomationTaskValidation(t *testing.T) {
	ts := newTestServer()
	res := postJSON(t, ts, "/api/automations/tasks", map[string]any{
		"type": "容量重试",
	})

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestGetJob(t *testing.T) {
	ts := newTestServer()
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/JOB-1042", nil)

	ts.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.ID != "JOB-1042" || body.Status != "SUCCESS" {
		t.Fatalf("unexpected job detail: %#v", body)
	}
}

func TestCancelJob(t *testing.T) {
	ts := newTestServer()
	createRes := postJSON(t, ts, "/api/instances/inst-prod-web-01/ip-tasks", map[string]any{
		"mode": "assign-public-ip",
	})
	var created struct {
		ID string `json:"id"`
	}
	decodeTestJSON(t, createRes.Body.Bytes(), &created)

	res := postJSON(t, ts, "/api/jobs/"+created.ID+"/cancel", map[string]any{})

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.ID != created.ID || body.Status != "CANCELLED" {
		t.Fatalf("unexpected cancel response: %#v", body)
	}
}

func TestRetryJob(t *testing.T) {
	ts := newTestServer()
	res := postJSON(t, ts, "/api/jobs/JOB-1040/retry", map[string]any{})

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		ID         string         `json:"id"`
		Status     string         `json:"status"`
		RetryCount int            `json:"retryCount"`
		Input      map[string]any `json:"input"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.ID == "JOB-1040" || body.Status != "PENDING" || body.RetryCount != 1 {
		t.Fatalf("unexpected retry response: %#v", body)
	}
	if body.Input["retryOf"] != "JOB-1040" {
		t.Fatalf("expected retryOf input, got %#v", body.Input)
	}
}

func newTestServer() http.Handler {
	return NewServer(store.NewSeeded()).Handler()
}

func postJSON(t *testing.T, handler http.Handler, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actor", "tester")
	handler.ServeHTTP(res, req)
	return res
}

func decodeTestJSON(t *testing.T, raw []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("decode json: %v body=%s", err, string(raw))
	}
}
