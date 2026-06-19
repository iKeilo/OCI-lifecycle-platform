package lifecyclenotify

import (
	"context"
	"fmt"
	"strings"

	"a-series-oracle/backend/internal/domain"
	"a-series-oracle/backend/internal/notify"
	"a-series-oracle/backend/internal/store"
)

type ReinstallEvent string

const (
	ReinstallRequested ReinstallEvent = "requested"
	ReinstallSucceeded ReinstallEvent = "succeeded"
	ReinstallFailed    ReinstallEvent = "failed"
)

func SendReinstallNotification(ctx context.Context, store *store.Store, event ReinstallEvent, job domain.Job, result map[string]any) domain.Notification {
	if store == nil {
		return domain.Notification{}
	}
	req := BuildReinstallNotification(store, event, job, result)
	notification, err := store.CreateNotification(req, defaultActor(job.CreatedBy))
	if err != nil {
		return domain.Notification{}
	}
	notification = deliverEmail(ctx, store, notification)
	notification = deliverWebhook(ctx, store, notification)
	return notification
}

func BuildReinstallNotification(store *store.Store, event ReinstallEvent, job domain.Job, result map[string]any) domain.NotificationRequest {
	instance, _ := store.GetInstance(job.ResourceID)
	title, severity := reinstallNotificationTitle(event, instanceName(instance, job))
	return domain.NotificationRequest{
		Title:          title,
		Message:        reinstallNotificationMessage(event, job, instance, result),
		Severity:       severity,
		Category:       "instance-system",
		ResourceType:   job.ResourceType,
		ResourceID:     job.ResourceID,
		ProfileID:      job.ProfileID,
		Region:         job.Region,
		CompartmentID:  job.CompartmentID,
		Sensitive:      true,
		EmailRequested: true,
	}
}

func reinstallNotificationTitle(event ReinstallEvent, name string) (string, domain.NotificationSeverity) {
	switch event {
	case ReinstallSucceeded:
		return "重装系统成功: " + name, domain.NotificationSuccess
	case ReinstallFailed:
		return "重装系统失败: " + name, domain.NotificationError
	default:
		return "重装系统任务已创建: " + name, domain.NotificationWarning
	}
}

func reinstallNotificationMessage(event ReinstallEvent, job domain.Job, instance domain.Instance, result map[string]any) string {
	statusLine := "任务已创建，正在等待 OCI 执行。"
	switch event {
	case ReinstallSucceeded:
		statusLine = "OCI 已完成重装系统任务，平台已完成结果验证。"
	case ReinstallFailed:
		statusLine = "OCI 重装系统任务失败，请查看错误信息后再决定是否重试。"
	}

	lines := []string{
		statusLine,
		"",
		"操作信息:",
		"- 操作: 重装系统",
		"- 任务 ID: " + job.ID,
		"- 操作人: " + defaultActor(job.CreatedBy),
		"- 实例名称: " + instanceName(instance, job),
		"- 实例 OCID: " + defaultString(job.ResourceID, "-"),
		"- Profile: " + defaultString(job.ProfileID, "-"),
		"- Region: " + defaultString(job.Region, "-"),
		"- Compartment: " + defaultString(job.CompartmentID, "-"),
		"",
		"服务器 SSH 信息:",
		"- SSH 用户: root 或镜像默认用户，请以镜像系统策略为准",
		"- SSH 密码: 未生成 / 未变更。OCI 重装路径不支持注入新的 root 密码或 cloud-init。",
		"- 公网 IPv4: " + defaultString(instance.PrimaryIP, "-"),
		"- 公网 IPv6: " + defaultString(instance.PrimaryIPv6, "-"),
		"- 私网 IP: " + defaultString(instance.PrivateIP, "-"),
		"",
		"重装参数:",
		"- 镜像: " + defaultString(stringFromMap(job.Input, "imageName"), stringFromMap(job.Input, "imageId")),
		"- 启动盘大小: " + intLabel(job.Input, result, "bootVolumeSizeGb", "targetBootVolumeGb", "GB"),
		"- 启动盘性能: " + intLabel(job.Input, result, "bootVolumeVpusPerGb", "targetBootVolumeVpusPerGb", "VPUs/GB"),
		"- 保留旧启动盘: " + boolLabel(boolFromMap(job.Input, "preserveOldBootVolume")),
	}
	if note := strings.TrimSpace(stringFromMap(job.Input, "note")); note != "" {
		lines = append(lines, "- 备注: "+note)
	}
	if job.OCIRequestID != "" || job.OCIWorkRequestID != "" {
		lines = append(lines, "", "OCI 引用:")
		if job.OCIRequestID != "" {
			lines = append(lines, "- Request ID: "+job.OCIRequestID)
		}
		if job.OCIWorkRequestID != "" {
			lines = append(lines, "- Work Request ID: "+job.OCIWorkRequestID)
		}
	}
	if event == ReinstallSucceeded {
		lines = append(lines, "", "结果:")
		lines = append(lines, "- 最终状态: "+defaultString(stringFromMap(result, "finalState"), "-"))
		lines = append(lines, "- 已验证: "+boolLabel(boolFromMap(result, "verified")))
	}
	if event == ReinstallFailed {
		lines = append(lines, "", "错误:")
		lines = append(lines, "- 错误码: "+defaultString(job.ErrorCode, stringFromMap(result, "errorCode")))
		lines = append(lines, "- 错误信息: "+defaultString(job.ErrorMessage, stringFromMap(result, "errorMessage")))
	}
	return strings.Join(lines, "\n")
}

func deliverEmail(ctx context.Context, store *store.Store, notification domain.Notification) domain.Notification {
	if !notification.EmailRequested {
		return notification
	}
	settings := store.GetEmailSettingsForSend()
	if !settings.Enabled {
		updated, _ := store.UpdateNotificationEmailStatus(notification.ID, false, "email delivery is disabled")
		return updated
	}
	if len(settings.To) == 0 {
		account := store.GetAccountSettings()
		if strings.TrimSpace(account.Email) != "" {
			settings.To = []string{strings.TrimSpace(account.Email)}
		}
	}
	if err := notify.SendEmail(ctx, settings, notification.Title, notification.Message); err != nil {
		updated, _ := store.UpdateNotificationEmailStatus(notification.ID, false, err.Error())
		return updated
	}
	updated, _ := store.UpdateNotificationEmailStatus(notification.ID, true, "")
	return updated
}

func deliverWebhook(ctx context.Context, store *store.Store, notification domain.Notification) domain.Notification {
	settings := store.GetWebhookSettingsForSend()
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
		updated, _ := store.UpdateNotificationWebhookStatus(notification.ID, false, err.Error())
		return updated
	}
	updated, _ := store.UpdateNotificationWebhookStatus(notification.ID, true, "")
	return updated
}

func instanceName(instance domain.Instance, job domain.Job) string {
	if strings.TrimSpace(instance.Name) != "" {
		return strings.TrimSpace(instance.Name)
	}
	if name := strings.TrimSpace(stringFromMap(job.Input, "confirmationName")); name != "" {
		return name
	}
	return defaultString(job.ResourceID, "unknown-instance")
}

func intLabel(input map[string]any, result map[string]any, inputKey string, resultKey string, unit string) string {
	value := intFromMap(result, resultKey)
	if value == 0 {
		value = intFromMap(input, inputKey)
	}
	if value == 0 {
		return "-"
	}
	return fmt.Sprintf("%d %s", value, unit)
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	if typed, ok := values[key].(string); ok {
		return strings.TrimSpace(typed)
	}
	return ""
}

func boolFromMap(values map[string]any, key string) bool {
	if values == nil {
		return false
	}
	if typed, ok := values[key].(bool); ok {
		return typed
	}
	return false
}

func intFromMap(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	switch typed := values[key].(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func boolLabel(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func defaultActor(value string) string {
	return defaultString(value, "system")
}

func defaultString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "-"
}
