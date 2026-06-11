package db

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"a-series-oracle/backend/internal/domain"
)

type PostgresSink struct {
	conn      *sql.DB
	encryptor *profileEncryptor
}

func NewPostgresSink(conn *sql.DB) *PostgresSink {
	return &PostgresSink{conn: conn}
}

func (s *PostgresSink) SetProfileKeyEncryptionKey(secret string) error {
	encryptor, err := newProfileEncryptor(secret)
	if err != nil {
		return err
	}
	s.encryptor = encryptor
	return nil
}

func (s *PostgresSink) SaveProfile(profile domain.Profile, secret domain.ProfileSecret) error {
	if s == nil || s.conn == nil {
		return ErrNotConfigured()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var encryptedPrivateKey any
	if strings.TrimSpace(secret.PrivateKey) != "" {
		if s.encryptor == nil {
			return errors.New("PROFILE_KEY_ENCRYPTION_KEY is required to store an inline OCI private key")
		}
		ciphertext, err := s.encryptor.encrypt([]byte(secret.PrivateKey))
		if err != nil {
			return err
		}
		encryptedPrivateKey = ciphertext
	}

	_, err := s.conn.ExecContext(ctx, `
INSERT INTO profiles (
  id, name, tenancy_ocid, user_ocid, fingerprint, default_region, status,
  private_key_ciphertext, private_key_file, last_checked_at, updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7,
  $8, $9, $10, now()
)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  tenancy_ocid = EXCLUDED.tenancy_ocid,
  user_ocid = EXCLUDED.user_ocid,
  fingerprint = EXCLUDED.fingerprint,
  default_region = EXCLUDED.default_region,
  status = EXCLUDED.status,
  private_key_ciphertext = COALESCE(EXCLUDED.private_key_ciphertext, profiles.private_key_ciphertext),
  private_key_file = COALESCE(EXCLUDED.private_key_file, profiles.private_key_file),
  last_checked_at = EXCLUDED.last_checked_at,
  updated_at = now()`,
		profile.ID,
		profile.Name,
		profile.TenancyOCID,
		profile.UserOCID,
		profile.Fingerprint,
		profile.DefaultRegion,
		profile.Status,
		encryptedPrivateKey,
		nullableString(secret.PrivateKeyFile),
		nullableTime(profile.LastCheckedAt),
	)
	return err
}

func (s *PostgresSink) ListProfiles() ([]domain.Profile, error) {
	if s == nil || s.conn == nil {
		return nil, ErrNotConfigured()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := s.conn.QueryContext(ctx, `
SELECT id, name, tenancy_ocid, user_ocid, fingerprint, default_region, status, last_checked_at
FROM profiles
ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Profile
	for rows.Next() {
		var profile domain.Profile
		var lastCheckedAt sql.NullTime
		if err := rows.Scan(
			&profile.ID,
			&profile.Name,
			&profile.TenancyOCID,
			&profile.UserOCID,
			&profile.Fingerprint,
			&profile.DefaultRegion,
			&profile.Status,
			&lastCheckedAt,
		); err != nil {
			return nil, err
		}
		if lastCheckedAt.Valid {
			profile.LastCheckedAt = lastCheckedAt.Time
		}
		out = append(out, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresSink) GetProfileSecret(profileID string) (domain.ProfileSecret, error) {
	if s == nil || s.conn == nil {
		return domain.ProfileSecret{}, ErrNotConfigured()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var ciphertext []byte
	var privateKeyFile sql.NullString
	err := s.conn.QueryRowContext(ctx, `
SELECT private_key_ciphertext, private_key_file
FROM profiles
WHERE id = $1`, profileID).Scan(&ciphertext, &privateKeyFile)
	if err != nil {
		return domain.ProfileSecret{}, err
	}

	secret := domain.ProfileSecret{}
	if len(ciphertext) > 0 {
		if s.encryptor == nil {
			return domain.ProfileSecret{}, errors.New("PROFILE_KEY_ENCRYPTION_KEY is required to decrypt an OCI private key")
		}
		plaintext, err := s.encryptor.decrypt(ciphertext)
		if err != nil {
			return domain.ProfileSecret{}, err
		}
		secret.PrivateKey = string(plaintext)
	}
	if privateKeyFile.Valid {
		secret.PrivateKeyFile = privateKeyFile.String
	}
	return secret, nil
}

func (s *PostgresSink) DeleteProfile(profileID string) error {
	if s == nil || s.conn == nil {
		return ErrNotConfigured()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.conn.ExecContext(ctx, `DELETE FROM profiles WHERE id = $1`, profileID)
	return err
}

func (s *PostgresSink) SaveInstance(instance domain.Instance) error {
	if s == nil || s.conn == nil {
		return ErrNotConfigured()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.conn.ExecContext(ctx, `
INSERT INTO instances (
  id, oci_instance_id, profile_id, name, shape, region, compartment, compartment_id,
  primary_ip, private_ip, ocpus, memory_gb, boot_volume_gb, status, protected,
  reserved_ip_name, created_label, last_synced_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8,
  $9, $10, $11, $12, $13, $14, $15,
  $16, $17, $18
)
ON CONFLICT (id) DO UPDATE SET
  oci_instance_id = EXCLUDED.oci_instance_id,
  profile_id = EXCLUDED.profile_id,
  name = EXCLUDED.name,
  shape = EXCLUDED.shape,
  region = EXCLUDED.region,
  compartment = EXCLUDED.compartment,
  compartment_id = EXCLUDED.compartment_id,
  primary_ip = EXCLUDED.primary_ip,
  private_ip = EXCLUDED.private_ip,
  ocpus = EXCLUDED.ocpus,
  memory_gb = EXCLUDED.memory_gb,
  boot_volume_gb = EXCLUDED.boot_volume_gb,
  status = EXCLUDED.status,
  protected = EXCLUDED.protected,
  reserved_ip_name = EXCLUDED.reserved_ip_name,
  created_label = EXCLUDED.created_label,
  last_synced_at = EXCLUDED.last_synced_at,
  updated_at = now()`,
		instance.ID,
		nullableString(instance.OCIInstanceID),
		defaultString(instance.ProfileID, "DEFAULT"),
		instance.Name,
		instance.Shape,
		instance.Region,
		instance.Compartment,
		instance.CompartmentID,
		instance.PrimaryIP,
		instance.PrivateIP,
		instance.OCPUs,
		instance.MemoryGB,
		instance.BootVolumeGB,
		string(instance.Status),
		instance.Protected,
		nullableString(instance.ReservedIPName),
		instance.Created,
		nullableTime(instance.LastSyncedAt),
	)
	return err
}

func (s *PostgresSink) ListInstances(status string) ([]domain.Instance, error) {
	if s == nil || s.conn == nil {
		return nil, ErrNotConfigured()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
SELECT id, oci_instance_id, profile_id, name, shape, region, compartment, compartment_id,
       primary_ip, private_ip, ocpus, memory_gb, boot_volume_gb, status, protected,
       reserved_ip_name, created_label, last_synced_at
FROM instances`
	args := []any{}
	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
	}
	query += " ORDER BY name ASC"

	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Instance
	for rows.Next() {
		var instance domain.Instance
		var ociInstanceID, reservedIPName sql.NullString
		var lastSyncedAt sql.NullTime
		var statusValue string
		if err := rows.Scan(
			&instance.ID,
			&ociInstanceID,
			&instance.ProfileID,
			&instance.Name,
			&instance.Shape,
			&instance.Region,
			&instance.Compartment,
			&instance.CompartmentID,
			&instance.PrimaryIP,
			&instance.PrivateIP,
			&instance.OCPUs,
			&instance.MemoryGB,
			&instance.BootVolumeGB,
			&statusValue,
			&instance.Protected,
			&reservedIPName,
			&instance.Created,
			&lastSyncedAt,
		); err != nil {
			return nil, err
		}
		instance.Status = domain.InstanceStatus(statusValue)
		if ociInstanceID.Valid {
			instance.OCIInstanceID = ociInstanceID.String
		}
		if reservedIPName.Valid {
			instance.ReservedIPName = reservedIPName.String
		}
		if lastSyncedAt.Valid {
			instance.LastSyncedAt = lastSyncedAt.Time
		}
		out = append(out, instance)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresSink) ListJobs() ([]domain.Job, error) {
	if s == nil || s.conn == nil {
		return nil, ErrNotConfigured()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := s.conn.QueryContext(ctx, `
SELECT id, type, status, profile_id, region, compartment_id, resource_type, resource_id,
       oci_request_id, oci_work_request_id, input, result, error_code, error_message,
       retry_count, max_retries, created_by, created_at, started_at, finished_at
FROM jobs
ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Job
	for rows.Next() {
		var job domain.Job
		var statusValue string
		var inputRaw, resultRaw []byte
		var errorCode, errorMessage sql.NullString
		var startedAt, finishedAt sql.NullTime
		if err := rows.Scan(
			&job.ID,
			&job.Type,
			&statusValue,
			&job.ProfileID,
			&job.Region,
			&job.CompartmentID,
			&job.ResourceType,
			&job.ResourceID,
			&job.OCIRequestID,
			&job.OCIWorkRequestID,
			&inputRaw,
			&resultRaw,
			&errorCode,
			&errorMessage,
			&job.RetryCount,
			&job.MaxRetries,
			&job.CreatedBy,
			&job.CreatedAt,
			&startedAt,
			&finishedAt,
		); err != nil {
			return nil, err
		}
		job.Status = domain.JobStatus(statusValue)
		input, err := jsonMap(inputRaw)
		if err != nil {
			return nil, err
		}
		job.Input = input
		result, err := jsonMap(resultRaw)
		if err != nil {
			return nil, err
		}
		job.Result = result
		if errorCode.Valid {
			job.ErrorCode = errorCode.String
		}
		if errorMessage.Valid {
			job.ErrorMessage = errorMessage.String
		}
		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			job.FinishedAt = &finishedAt.Time
		}
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresSink) SaveJob(job domain.Job) error {
	if s == nil || s.conn == nil {
		return ErrNotConfigured()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	input, err := jsonPayload(job.Input, false)
	if err != nil {
		return err
	}
	result, err := jsonPayload(job.Result, true)
	if err != nil {
		return err
	}

	_, err = s.conn.ExecContext(ctx, `
INSERT INTO jobs (
  id, type, status, profile_id, region, compartment_id, resource_type, resource_id,
  oci_request_id, oci_work_request_id, input, result, error_code, error_message,
  retry_count, max_retries, created_by, created_at, started_at, finished_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8,
  $9, $10, $11, $12, $13, $14,
  $15, $16, $17, $18, $19, $20
)
ON CONFLICT (id) DO UPDATE SET
  type = EXCLUDED.type,
  status = EXCLUDED.status,
  profile_id = EXCLUDED.profile_id,
  region = EXCLUDED.region,
  compartment_id = EXCLUDED.compartment_id,
  resource_type = EXCLUDED.resource_type,
  resource_id = EXCLUDED.resource_id,
  oci_request_id = EXCLUDED.oci_request_id,
  oci_work_request_id = EXCLUDED.oci_work_request_id,
  input = EXCLUDED.input,
  result = EXCLUDED.result,
  error_code = EXCLUDED.error_code,
  error_message = EXCLUDED.error_message,
  retry_count = EXCLUDED.retry_count,
  max_retries = EXCLUDED.max_retries,
  created_by = EXCLUDED.created_by,
  created_at = EXCLUDED.created_at,
  started_at = EXCLUDED.started_at,
  finished_at = EXCLUDED.finished_at`,
		job.ID,
		job.Type,
		string(job.Status),
		job.ProfileID,
		job.Region,
		job.CompartmentID,
		job.ResourceType,
		job.ResourceID,
		job.OCIRequestID,
		job.OCIWorkRequestID,
		input,
		result,
		nullableString(job.ErrorCode),
		nullableString(job.ErrorMessage),
		job.RetryCount,
		job.MaxRetries,
		job.CreatedBy,
		job.CreatedAt,
		job.StartedAt,
		job.FinishedAt,
	)
	return err
}

func (s *PostgresSink) RecordAudit(entry domain.AuditLog) error {
	if s == nil || s.conn == nil {
		return ErrNotConfigured()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	requestPayload, err := jsonPayload(entry.RequestPayload, true)
	if err != nil {
		return err
	}
	resultPayload, err := jsonPayload(entry.ResultPayload, true)
	if err != nil {
		return err
	}
	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	_, err = s.conn.ExecContext(ctx, `
INSERT INTO audit_logs (
  actor, action, resource_type, resource_id, profile_id, region, compartment_id,
  oci_request_id, oci_work_request_id, request_payload, result_payload,
  error_code, error_message, created_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7,
  $8, $9, $10, $11,
  $12, $13, $14
)`,
		entry.Actor,
		entry.Action,
		entry.ResourceType,
		entry.ResourceID,
		entry.ProfileID,
		entry.Region,
		entry.CompartmentID,
		entry.OCIRequestID,
		entry.OCIWorkRequestID,
		requestPayload,
		resultPayload,
		nullableString(entry.ErrorCode),
		nullableString(entry.ErrorMessage),
		createdAt,
	)
	return err
}

func (s *PostgresSink) ListAuditLogs(filter domain.AuditLogFilter) ([]domain.AuditLog, error) {
	if s == nil || s.conn == nil {
		return nil, ErrNotConfigured()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT id, actor, action, resource_type, resource_id, profile_id, region, compartment_id,
       oci_request_id, oci_work_request_id, request_payload, result_payload,
       error_code, error_message, created_at
FROM audit_logs`
	var where []string
	var args []any
	addLike := func(column string, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		args = append(args, "%"+value+"%")
		where = append(where, fmt.Sprintf("%s ILIKE $%d", column, len(args)))
	}
	addLike("actor", filter.Actor)
	addLike("action", filter.Action)
	addLike("resource_type", filter.ResourceType)
	addLike("resource_id", filter.ResourceID)
	addLike("profile_id", filter.ProfileID)
	switch strings.ToLower(strings.TrimSpace(filter.Status)) {
	case "failed":
		where = append(where, "(COALESCE(error_code, '') <> '' OR COALESCE(error_message, '') <> '')")
	case "success":
		where = append(where, "(COALESCE(error_code, '') = '' AND COALESCE(error_message, '') = '')")
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", len(args))

	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AuditLog
	for rows.Next() {
		var entry domain.AuditLog
		var requestRaw, resultRaw []byte
		var errorCode, errorMessage sql.NullString
		if err := rows.Scan(
			&entry.ID,
			&entry.Actor,
			&entry.Action,
			&entry.ResourceType,
			&entry.ResourceID,
			&entry.ProfileID,
			&entry.Region,
			&entry.CompartmentID,
			&entry.OCIRequestID,
			&entry.OCIWorkRequestID,
			&requestRaw,
			&resultRaw,
			&errorCode,
			&errorMessage,
			&entry.CreatedAt,
		); err != nil {
			return nil, err
		}
		requestPayload, err := jsonMap(requestRaw)
		if err != nil {
			return nil, err
		}
		resultPayload, err := jsonMap(resultRaw)
		if err != nil {
			return nil, err
		}
		entry.RequestPayload = requestPayload
		entry.ResultPayload = resultPayload
		if errorCode.Valid {
			entry.ErrorCode = errorCode.String
		}
		if errorMessage.Valid {
			entry.ErrorMessage = errorMessage.String
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresSink) SaveEmailSettings(settings domain.EmailSettings) error {
	return s.saveSetting("email", settings)
}

func (s *PostgresSink) GetEmailSettings() (domain.EmailSettings, error) {
	var settings domain.EmailSettings
	if err := s.getSetting("email", &settings); err != nil {
		return domain.EmailSettings{}, err
	}
	if settings.Password != "" {
		settings.PasswordSet = true
	}
	return settings, nil
}

func (s *PostgresSink) SaveWebhookSettings(settings domain.WebhookSettings) error {
	return s.saveSetting("webhook", settings)
}

func (s *PostgresSink) GetWebhookSettings() (domain.WebhookSettings, error) {
	var settings domain.WebhookSettings
	if err := s.getSetting("webhook", &settings); err != nil {
		return domain.WebhookSettings{}, err
	}
	if settings.Secret != "" {
		settings.SecretSet = true
	}
	return settings, nil
}

func (s *PostgresSink) SaveAccountSettings(settings domain.AccountSettings) error {
	return s.saveSetting("account", settings)
}

func (s *PostgresSink) GetAccountSettings() (domain.AccountSettings, error) {
	var settings domain.AccountSettings
	if err := s.getSetting("account", &settings); err != nil {
		return domain.AccountSettings{}, err
	}
	if settings.PasswordHash != "" {
		settings.PasswordSet = true
	}
	return settings, nil
}

func (s *PostgresSink) SaveAppearanceSettings(settings domain.AppearanceSettings) error {
	return s.saveSetting("appearance", settings)
}

func (s *PostgresSink) GetAppearanceSettings() (domain.AppearanceSettings, error) {
	var settings domain.AppearanceSettings
	if err := s.getSetting("appearance", &settings); err != nil {
		return domain.AppearanceSettings{}, err
	}
	return settings, nil
}

func (s *PostgresSink) saveSetting(key string, value any) error {
	if s == nil || s.conn == nil {
		return ErrNotConfigured()
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.conn.ExecContext(ctx, `
INSERT INTO app_settings (key, value, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, key, payload)
	return err
}

func (s *PostgresSink) getSetting(key string, out any) error {
	if s == nil || s.conn == nil {
		return ErrNotConfigured()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var payload []byte
	err := s.conn.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = $1`, key).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	return json.Unmarshal(payload, out)
}

func jsonPayload(payload map[string]any, nullable bool) (any, error) {
	if len(payload) == 0 {
		if nullable {
			return nil, nil
		}
		payload = map[string]any{}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func jsonMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

type profileEncryptor struct {
	aead cipher.AEAD
}

func newProfileEncryptor(secret string) (*profileEncryptor, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, nil
	}
	key, err := profileEncryptionKey(secret)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &profileEncryptor{aead: aead}, nil
}

func profileEncryptionKey(secret string) ([]byte, error) {
	for _, decoder := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
		raw, err := decoder.DecodeString(secret)
		if err == nil && len(raw) == 32 {
			return raw, nil
		}
	}
	if len([]byte(secret)) == 32 {
		return []byte(secret), nil
	}
	return nil, fmt.Errorf("PROFILE_KEY_ENCRYPTION_KEY must be exactly 32 bytes or a base64 encoded 32-byte key")
}

func (e *profileEncryptor) encrypt(plaintext []byte) ([]byte, error) {
	if e == nil {
		return nil, errors.New("profile encryptor is not configured")
	}
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := e.aead.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(nonce)+len(ciphertext))
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

func (e *profileEncryptor) decrypt(ciphertext []byte) ([]byte, error) {
	if e == nil {
		return nil, errors.New("profile encryptor is not configured")
	}
	nonceSize := e.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("encrypted profile key is malformed")
	}
	nonce := ciphertext[:nonceSize]
	payload := ciphertext[nonceSize:]
	return e.aead.Open(nil, nonce, payload, nil)
}
