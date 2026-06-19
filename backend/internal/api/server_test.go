package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	s := store.New()
	ts := NewServer(s).Handler()
	createRes := postJSON(t, ts, "/api/templates", map[string]any{
		"name":           "E2 Micro 预输入",
		"profileId":      "profile-default",
		"region":         "ap-chuncheon-1",
		"shape":          "VM.Standard.E2.1.Micro",
		"ocpus":          1,
		"memoryGb":       1,
		"bootVolumeGb":   50,
		"assignPublicIp": true,
		"tags": map[string]string{
			"managedBy": "oci-lifecycle-platform",
		},
	})
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected template create 201, got %d body=%s", createRes.Code, createRes.Body.String())
	}
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
		if item.ID == "" || item.Shape == "" || item.Status != "ACTIVE" {
			t.Fatalf("unexpected template: %#v", item)
		}
	}
}

func TestTemplateCanBeSavedWithoutOCIProfileAndLocallyChecked(t *testing.T) {
	ts := NewServer(store.New()).Handler()
	createRes := postJSON(t, ts, "/api/templates", map[string]any{
		"name":         "无 Profile 预输入",
		"shape":        "VM.Standard.E3.Flex",
		"ocpus":        1,
		"memoryGb":     1,
		"bootVolumeGb": 50,
	})
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected template create 201, got %d body=%s", createRes.Code, createRes.Body.String())
	}
	var created struct {
		ID                 string `json:"id"`
		ProfileID          string `json:"profileId"`
		ValidationStatus   string `json:"validationStatus"`
		ValidationErrorMsg string `json:"validationMessage"`
	}
	decodeTestJSON(t, createRes.Body.Bytes(), &created)
	if created.ID == "" || created.ProfileID != "" {
		t.Fatalf("expected template without profile to be saved, got %#v", created)
	}

	validateRes := postJSON(t, ts, "/api/templates/"+created.ID+"/validate", map[string]any{})
	if validateRes.Code != http.StatusOK {
		t.Fatalf("expected local validation 200, got %d body=%s", validateRes.Code, validateRes.Body.String())
	}
	var validation struct {
		Verified         bool     `json:"verified"`
		Status           string   `json:"status"`
		ErrorCode        string   `json:"errorCode"`
		IncompatibleKeys []string `json:"incompatibleKeys"`
	}
	decodeTestJSON(t, validateRes.Body.Bytes(), &validation)
	if validation.Verified || validation.Status != "INVALID" || validation.ErrorCode != "TEMPLATE_FIELDS_INCOMPLETE" {
		t.Fatalf("expected incomplete local validation, got %#v", validation)
	}
	if len(validation.IncompatibleKeys) == 0 {
		t.Fatalf("expected missing fields to be listed, got %#v", validation)
	}
}

func TestLaunchOptions(t *testing.T) {
	s := store.New()
	if _, err := s.CreateProfile(domain.CreateProfileRequest{
		Name:          "DEFAULT",
		TenancyOCID:   "ocid1.tenancy.oc1..example",
		UserOCID:      "ocid1.user.oc1..example",
		Fingerprint:   "11:22:33",
		DefaultRegion: "ap-chuncheon-1",
	}, "tester"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTemplate(domain.CreateTemplateRequest{
		Name:         "E2 Micro 预输入",
		ProfileID:    "profile-default",
		Region:       "ap-chuncheon-1",
		Shape:        "VM.Standard.E2.1.Micro",
		OCPUs:        1,
		MemoryGB:     1,
		BootVolumeGB: 50,
	}, "tester"); err != nil {
		t.Fatal(err)
	}
	ts := NewServer(s).Handler()
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

func TestCreateOCIInstanceReinstallCreatesJob(t *testing.T) {
	queued := ""
	st := store.NewSeeded()
	server := NewServerWithOptions(st, ServerOptions{
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
	res := postJSON(t, server, "/api/instances/"+instanceID+"/system/reinstall", map[string]any{
		"imageId":                "ocid1.image.oc1.ap-chuncheon-1.example",
		"imageName":              "Oracle-Linux-9",
		"bootVolumeSizeGb":       50,
		"bootVolumeVpusPerGb":    10,
		"preserveOldBootVolume":  true,
		"createBootVolumeBackup": false,
		"generateRootPassword":   false,
		"notifyPasswordInApp":    false,
		"notifyPasswordByEmail":  false,
		"sshAuthorizedKey":       "",
		"cloudInit":              "",
		"confirmationName":       "",
		"note":                   "api-test",
	})

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		ID         string         `json:"id"`
		Type       string         `json:"type"`
		ResourceID string         `json:"resourceId"`
		Input      map[string]any `json:"input"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.Type != "重装系统" || body.ResourceID != instanceID || body.Input["operation"] != "reinstall" {
		t.Fatalf("unexpected reinstall job: %#v", body)
	}
	if queued != body.ID {
		t.Fatalf("expected reinstall job to be enqueued, queued=%s body=%s", queued, body.ID)
	}
	notifications := st.ListNotifications(false)
	if len(notifications) != 1 {
		t.Fatalf("expected reinstall creation notification, got %#v", notifications)
	}
	notice := notifications[0]
	if notice.Title != "重装系统任务已创建: "+instanceID || notice.Category != "instance-system" || !notice.EmailRequested {
		t.Fatalf("unexpected reinstall notification: %#v", notice)
	}
	if !strings.Contains(notice.Message, "SSH 密码: 未生成 / 未变更") || !strings.Contains(notice.Message, "操作: 重装系统") {
		t.Fatalf("expected SSH and operation details in notification, got %s", notice.Message)
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
		"mode":              "enable-ipv6",
		"vnicId":            "primary",
		"enableIpv6":        true,
		"autoConfigureIpv6": true,
		"ipv6Strategy":      "replace_gateway",
		"snapshotBefore":    true,
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
	if body.ResourceID != instanceID || body.Input["operation"] != "ip-management" || body.Input["enableIpv6"] != true || body.Input["ipv6Strategy"] != "replace_gateway" || body.Input["mayReplacePublicIPv4"] != false {
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

func TestRootTenancyCreateInstanceGeneratesSensitiveNotification(t *testing.T) {
	queued := ""
	tenancyID := "ocid1.tenancy.oc1..example"
	server := NewServerWithOptions(store.NewSeeded(), ServerOptions{
		ExecutionMode: "oci",
		Enqueue: func(jobID string) {
			queued = jobID
		},
		OCIReadiness: oci.ReadinessConfig{
			ExecutionMode: "oci",
			TenancyOCID:   tenancyID,
			Region:        "ap-chuncheon-1",
		},
	}).Handler()

	res := postJSON(t, server, "/api/instances", map[string]any{
		"name":                 "root-password-test",
		"compartmentId":        tenancyID,
		"shape":                "VM.Standard.E2.1.Micro",
		"ocpus":                1,
		"memoryGb":             1,
		"bootVolumeGb":         50,
		"generateRootPassword": true,
		"notifyRootPassword":   true,
	})

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		ID    string         `json:"id"`
		Input map[string]any `json:"input"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if queued != body.ID {
		t.Fatalf("expected launch job to be queued, queued=%s body=%s", queued, body.ID)
	}
	if _, ok := body.Input["cloudInit"]; ok {
		t.Fatalf("cloudInit must be redacted from API response: %#v", body.Input)
	}
	if body.Input["cloudInitRedacted"] != true {
		t.Fatalf("expected cloudInitRedacted flag, got %#v", body.Input)
	}

	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/notifications?unread=true", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("expected notification list 200, got %d body=%s", list.Code, list.Body.String())
	}
	var notifications struct {
		UnreadCount int `json:"unreadCount"`
		Items       []struct {
			ID             string `json:"id"`
			Title          string `json:"title"`
			Category       string `json:"category"`
			Sensitive      bool   `json:"sensitive"`
			EmailRequested bool   `json:"emailRequested"`
			EmailSent      bool   `json:"emailSent"`
			EmailError     string `json:"emailError"`
		} `json:"items"`
	}
	decodeTestJSON(t, list.Body.Bytes(), &notifications)
	if notifications.UnreadCount == 0 || len(notifications.Items) == 0 {
		t.Fatalf("expected unread sensitive notification, got %#v", notifications)
	}
	notice := notifications.Items[0]
	if notice.Title != "Root password generated: root-password-test" || notice.Category != "credential" || !notice.Sensitive || !notice.EmailRequested {
		t.Fatalf("unexpected notification: %#v", notice)
	}
	if notice.EmailSent || notice.EmailError == "" {
		t.Fatalf("email should be recorded as not sent when SMTP is disabled: %#v", notice)
	}

	deleteRes := httptest.NewRecorder()
	server.ServeHTTP(deleteRes, httptest.NewRequest(http.MethodDelete, "/api/notifications/"+notice.ID, nil))
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("expected delete notification 204, got %d body=%s", deleteRes.Code, deleteRes.Body.String())
	}
	listAfterDelete := httptest.NewRecorder()
	server.ServeHTTP(listAfterDelete, httptest.NewRequest(http.MethodGet, "/api/notifications?unread=true", nil))
	if listAfterDelete.Code != http.StatusOK {
		t.Fatalf("expected notification list 200 after delete, got %d body=%s", listAfterDelete.Code, listAfterDelete.Body.String())
	}
	var afterDelete struct {
		UnreadCount int `json:"unreadCount"`
		Items       []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	decodeTestJSON(t, listAfterDelete.Body.Bytes(), &afterDelete)
	if afterDelete.UnreadCount != 0 || len(afterDelete.Items) != 0 {
		t.Fatalf("expected deleted notification to disappear, got %#v", afterDelete)
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

func TestClearCompletedJobs(t *testing.T) {
	ts := newTestServer()
	res := postJSON(t, ts, "/api/jobs/clear-completed", map[string]any{})

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		DeletedCount int `json:"deletedCount"`
		Items        []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	decodeTestJSON(t, res.Body.Bytes(), &body)
	if body.DeletedCount != 2 {
		t.Fatalf("expected two seeded completed jobs to be cleared, got %#v", body)
	}
	for _, job := range body.Items {
		if job.ID == "JOB-1042" || job.ID == "JOB-1040" {
			t.Fatalf("completed job remained after cleanup: %#v", body.Items)
		}
	}
	if len(body.Items) != 1 || body.Items[0].ID != "JOB-1041" || body.Items[0].Status != "WAITING_OCI" {
		t.Fatalf("expected waiting OCI job to remain, got %#v", body.Items)
	}
}

func TestGuardrailsBlockOversizedLaunch(t *testing.T) {
	appStore := store.NewSeeded()
	if _, err := appStore.SetSecurityGuardrailSettings(domain.SecurityGuardrailSettings{
		Enabled:                 true,
		MaxOCPUsPerInstance:     1,
		MaxMemoryGBPerInstance:  2,
		MaxBootVolumeGB:         50,
		MaxRetryAttempts:        3,
		MaxPublicIPBatchCount:   2,
		BlockBootVolumeDeletion: true,
	}, "tester"); err != nil {
		t.Fatal(err)
	}
	ts := NewServer(appStore).Handler()
	res := postJSON(t, ts, "/api/instances", map[string]any{
		"name":         "too-large",
		"shape":        "VM.Standard.E3.Flex",
		"ocpus":        4,
		"memoryGb":     16,
		"bootVolumeGb": 50,
	})
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected oversized launch to be blocked, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestGuardrailsBlockTerminateDeletingBootVolume(t *testing.T) {
	appStore := store.NewSeeded()
	if _, err := appStore.SetSecurityGuardrailSettings(domain.SecurityGuardrailSettings{
		Enabled:                       true,
		MaxOCPUsPerInstance:           8,
		MaxMemoryGBPerInstance:        32,
		MaxBootVolumeGB:               200,
		MaxRetryAttempts:              10,
		MaxPublicIPBatchCount:         10,
		RequireApprovalForTerminate:   true,
		BlockBootVolumeDeletion:       true,
		BlockRootPasswordWithoutEmail: true,
	}, "tester"); err != nil {
		t.Fatal(err)
	}
	ts := NewServer(appStore).Handler()
	res := postJSON(t, ts, "/api/instances/inst-prod-web-01/actions", map[string]any{
		"action":             "TERMINATE",
		"preserveBootVolume": false,
		"snapshotBefore":     true,
	})
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected terminate delete boot volume to be blocked, got %d body=%s", res.Code, res.Body.String())
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
