package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"a-series-oracle/backend/internal/domain"
)

type WebhookPayload struct {
	Event        string              `json:"event"`
	Title        string              `json:"title"`
	Message      string              `json:"message"`
	Severity     string              `json:"severity"`
	Notification domain.Notification `json:"notification,omitempty"`
	CreatedAt    time.Time           `json:"createdAt"`
}

func SendWebhook(ctx context.Context, settings domain.WebhookSettings, payload WebhookPayload) error {
	if !settings.Enabled {
		return nil
	}
	if strings.TrimSpace(settings.URL) == "" {
		return fmt.Errorf("webhook URL is not configured")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, settings.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "OCI-Lifecycle-Platform/1.0")
	for key, value := range settings.Headers {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	if strings.TrimSpace(settings.Secret) != "" {
		req.Header.Set("X-OCI-Lifecycle-Signature", hmacSignature(settings.Secret, body))
	}

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = resp.Status
		}
		return fmt.Errorf("webhook returned %s: %s", resp.Status, message)
	}
	return nil
}

func hmacSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
