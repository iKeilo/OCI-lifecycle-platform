package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, nil
	}
	conn, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(30 * time.Minute)
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func Migrate(ctx context.Context, conn *sql.DB) error {
	if conn == nil {
		return nil
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, schemaSQL); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, seedMigrationSQL); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

func IsConfigured(conn *sql.DB) bool {
	return conn != nil
}

func ErrNotConfigured() error {
	return errors.New("database is not configured; set DATABASE_URL")
}

func Health(ctx context.Context, conn *sql.DB) error {
	if conn == nil {
		return ErrNotConfigured()
	}
	if err := conn.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS profiles (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  tenancy_ocid TEXT NOT NULL,
  user_ocid TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  default_region TEXT NOT NULL,
  status TEXT NOT NULL,
  private_key_ciphertext BYTEA,
  private_key_file TEXT,
  last_checked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS instances (
  id TEXT PRIMARY KEY,
  oci_instance_id TEXT UNIQUE,
  profile_id TEXT NOT NULL,
  name TEXT NOT NULL,
  shape TEXT NOT NULL,
  region TEXT NOT NULL,
  compartment TEXT NOT NULL,
  compartment_id TEXT NOT NULL,
  primary_ip TEXT NOT NULL DEFAULT '',
  private_ip TEXT NOT NULL DEFAULT '',
  primary_ipv6 TEXT NOT NULL DEFAULT '',
  ipv6_addresses JSONB NOT NULL DEFAULT '[]'::jsonb,
  ocpus INTEGER NOT NULL DEFAULT 0,
  memory_gb INTEGER NOT NULL DEFAULT 0,
  boot_volume_gb INTEGER NOT NULL DEFAULT 0,
  boot_volume_vpus_per_gb INTEGER NOT NULL DEFAULT 10,
  status TEXT NOT NULL,
  protected BOOLEAN NOT NULL DEFAULT false,
  reserved_ip_name TEXT,
  created_label TEXT NOT NULL DEFAULT '',
  last_synced_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  status TEXT NOT NULL,
  profile_id TEXT NOT NULL,
  region TEXT NOT NULL,
  compartment_id TEXT NOT NULL,
  resource_type TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  oci_request_id TEXT NOT NULL DEFAULT '',
  oci_work_request_id TEXT NOT NULL DEFAULT '',
  input JSONB NOT NULL DEFAULT '{}'::jsonb,
  result JSONB,
  error_code TEXT,
  error_message TEXT,
  retry_count INTEGER NOT NULL DEFAULT 0,
  max_retries INTEGER NOT NULL DEFAULT 0,
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  actor TEXT NOT NULL,
  action TEXT NOT NULL,
  resource_type TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  profile_id TEXT NOT NULL DEFAULT '',
  region TEXT NOT NULL DEFAULT '',
  compartment_id TEXT NOT NULL DEFAULT '',
  oci_request_id TEXT NOT NULL DEFAULT '',
  oci_work_request_id TEXT NOT NULL DEFAULT '',
  request_payload JSONB,
  result_payload JSONB,
  error_code TEXT,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS instance_templates (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  version TEXT NOT NULL,
  profile_id TEXT NOT NULL,
  region TEXT NOT NULL,
  compartment TEXT NOT NULL,
  compartment_id TEXT NOT NULL DEFAULT '',
  availability_ad TEXT NOT NULL DEFAULT '',
  image_id TEXT NOT NULL,
  image_name TEXT NOT NULL,
  shape TEXT NOT NULL,
  ocpus INTEGER NOT NULL,
  memory_gb INTEGER NOT NULL,
  boot_volume_gb INTEGER NOT NULL,
  boot_volume_vpus_per_gb INTEGER NOT NULL DEFAULT 10,
  vcn_id TEXT NOT NULL,
  subnet_id TEXT NOT NULL,
  assign_public_ip BOOLEAN NOT NULL DEFAULT false,
  enable_ipv6 BOOLEAN NOT NULL DEFAULT false,
  reserved_public_ip TEXT NOT NULL DEFAULT '',
  ssh_key TEXT NOT NULL DEFAULT '',
  cloud_init TEXT NOT NULL DEFAULT '',
  cloud_init_set BOOLEAN NOT NULL DEFAULT false,
  tags JSONB NOT NULL DEFAULT '{}'::jsonb,
  config_format TEXT NOT NULL DEFAULT 'json',
  config_text TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  validation_status TEXT NOT NULL DEFAULT 'UNVERIFIED',
  validation_error_code TEXT NOT NULL DEFAULT '',
  validation_message TEXT NOT NULL DEFAULT '',
  last_validated_at TIMESTAMPTZ,
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS automations (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  target_pool TEXT NOT NULL,
  action TEXT NOT NULL,
  trigger_interval TEXT NOT NULL,
  cooldown TEXT NOT NULL,
  max_retries INTEGER NOT NULL,
  failure_policy TEXT NOT NULL,
  max_instances INTEGER NOT NULL,
  max_daily_runs INTEGER NOT NULL,
  region_scope TEXT NOT NULL,
  notify_channel TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT false,
  approval_required BOOLEAN NOT NULL DEFAULT false,
  last_run_at TIMESTAMPTZ,
  next_run_at TIMESTAMPTZ,
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS app_settings (
  key TEXT PRIMARY KEY,
  value JSONB NOT NULL DEFAULT '{}'::jsonb,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_jobs_status_created_at ON jobs(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_instances_status ON instances(status);

ALTER TABLE instances ADD COLUMN IF NOT EXISTS primary_ipv6 TEXT NOT NULL DEFAULT '';
ALTER TABLE instances ADD COLUMN IF NOT EXISTS ipv6_addresses JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE instances ADD COLUMN IF NOT EXISTS boot_volume_vpus_per_gb INTEGER NOT NULL DEFAULT 10;

ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS compartment_id TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS availability_ad TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS boot_volume_vpus_per_gb INTEGER NOT NULL DEFAULT 10;
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS enable_ipv6 BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS reserved_public_ip TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS ssh_key TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS cloud_init TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS cloud_init_set BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS config_format TEXT NOT NULL DEFAULT 'json';
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS config_text TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS validation_status TEXT NOT NULL DEFAULT 'UNVERIFIED';
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS validation_error_code TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS validation_message TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS last_validated_at TIMESTAMPTZ;
ALTER TABLE instance_templates ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
`

const seedMigrationSQL = `
INSERT INTO schema_migrations(version)
VALUES ('20260608_0001_initial_control_plane')
ON CONFLICT (version) DO NOTHING;
`
