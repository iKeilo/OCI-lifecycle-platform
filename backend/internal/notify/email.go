package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"a-series-oracle/backend/internal/domain"
)

func SendEmail(ctx context.Context, settings domain.EmailSettings, subject string, body string) error {
	if !settings.Enabled {
		return nil
	}
	if strings.TrimSpace(settings.Host) == "" {
		return fmt.Errorf("SMTP host is not configured")
	}
	if strings.TrimSpace(settings.From) == "" {
		return fmt.Errorf("SMTP from address is not configured")
	}
	recipients := cleanRecipients(settings.To)
	if len(recipients) == 0 {
		return fmt.Errorf("SMTP recipient is not configured")
	}
	port := settings.Port
	if port == 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", settings.Host, port)
	message := buildMessage(settings.From, recipients, subject, body)

	if settings.UseTLS {
		return sendWithTLS(ctx, addr, settings, recipients, message)
	}
	return sendPlainOrStartTLS(ctx, addr, settings, recipients, message)
}

func sendPlainOrStartTLS(ctx context.Context, addr string, settings domain.EmailSettings, recipients []string, message []byte) error {
	dialer := net.Dialer{Timeout: 15 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, settings.Host)
	if err != nil {
		return err
	}
	defer client.Close()

	if settings.StartTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: settings.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return err
			}
		}
	}
	if auth := smtpAuth(settings); auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return err
			}
		}
	}
	return sendMailData(client, settings.From, recipients, message)
}

func sendWithTLS(ctx context.Context, addr string, settings domain.EmailSettings, recipients []string, message []byte) error {
	dialer := tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 15 * time.Second},
		Config:    &tls.Config{ServerName: settings.Host, MinVersion: tls.VersionTLS12},
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, settings.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if auth := smtpAuth(settings); auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	return sendMailData(client, settings.From, recipients, message)
}

func sendMailData(client *smtp.Client, from string, recipients []string, message []byte) error {
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func smtpAuth(settings domain.EmailSettings) smtp.Auth {
	if strings.TrimSpace(settings.Username) == "" {
		return nil
	}
	return smtp.PlainAuth("", settings.Username, settings.Password, settings.Host)
}

func buildMessage(from string, recipients []string, subject string, body string) []byte {
	headers := []string{
		"From: " + from,
		"To: " + strings.Join(recipients, ", "),
		"Subject: " + encodeHeader(defaultString(subject, "OCI Lifecycle Platform Notification")),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body + "\r\n")
}

func encodeHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func cleanRecipients(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			key := strings.ToLower(part)
			if part == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, part)
		}
	}
	return out
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
