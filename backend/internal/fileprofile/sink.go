package fileprofile

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"a-series-oracle/backend/internal/domain"
	"a-series-oracle/backend/internal/store"
)

type Sink struct {
	mu        sync.Mutex
	path      string
	encryptor *profileEncryptor
	data      profileStoreFile
}

type profileStoreFile struct {
	Version   int                                `json:"version"`
	Profiles  map[string]profileRow              `json:"profiles"`
	Templates map[string]domain.InstanceTemplate `json:"templates,omitempty"`
	Settings  map[string]json.RawMessage         `json:"settings,omitempty"`
}

type profileRow struct {
	Profile              domain.Profile `json:"profile"`
	PrivateKeyCiphertext string         `json:"privateKeyCiphertext,omitempty"`
	PrivateKeyFile       string         `json:"privateKeyFile,omitempty"`
}

func New(path string) (*Sink, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("profile store file path is required")
	}
	s := &Sink{
		path: path,
		data: profileStoreFile{
			Version:   1,
			Profiles:  map[string]profileRow{},
			Templates: map[string]domain.InstanceTemplate{},
			Settings:  map[string]json.RawMessage{},
		},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Sink) SetProfileKeyEncryptionKey(secret string) error {
	encryptor, err := newProfileEncryptor(secret)
	if err != nil {
		return err
	}
	s.encryptor = encryptor
	return nil
}

func (s *Sink) SaveProfile(profile domain.Profile, secret domain.ProfileSecret) error {
	if s == nil {
		return store.ErrNotFound
	}
	if strings.TrimSpace(profile.ID) == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	row := s.data.Profiles[profile.ID]
	row.Profile = profile
	if strings.TrimSpace(secret.PrivateKey) != "" {
		if s.encryptor == nil {
			return errors.New("PROFILE_KEY_ENCRYPTION_KEY is required to store an inline OCI private key")
		}
		ciphertext, err := s.encryptor.encrypt([]byte(secret.PrivateKey))
		if err != nil {
			return err
		}
		row.PrivateKeyCiphertext = base64.StdEncoding.EncodeToString(ciphertext)
	}
	if strings.TrimSpace(secret.PrivateKeyFile) != "" {
		row.PrivateKeyFile = strings.TrimSpace(secret.PrivateKeyFile)
	}
	s.data.Profiles[profile.ID] = row
	return s.flushLocked()
}

func (s *Sink) ListProfiles() ([]domain.Profile, error) {
	if s == nil {
		return nil, store.ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]domain.Profile, 0, len(s.data.Profiles))
	for _, row := range s.data.Profiles {
		if strings.TrimSpace(row.Profile.ID) == "" {
			continue
		}
		out = append(out, row.Profile)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Sink) GetProfileSecret(profileID string) (domain.ProfileSecret, error) {
	if s == nil {
		return domain.ProfileSecret{}, store.ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	row, ok := s.data.Profiles[profileID]
	if !ok {
		return domain.ProfileSecret{}, store.ErrNotFound
	}
	secret := domain.ProfileSecret{PrivateKeyFile: row.PrivateKeyFile}
	if strings.TrimSpace(row.PrivateKeyCiphertext) != "" {
		if s.encryptor == nil {
			return domain.ProfileSecret{}, errors.New("PROFILE_KEY_ENCRYPTION_KEY is required to decrypt an OCI private key")
		}
		raw, err := base64.StdEncoding.DecodeString(row.PrivateKeyCiphertext)
		if err != nil {
			return domain.ProfileSecret{}, err
		}
		plaintext, err := s.encryptor.decrypt(raw)
		if err != nil {
			return domain.ProfileSecret{}, err
		}
		secret.PrivateKey = string(plaintext)
	}
	return secret, nil
}

func (s *Sink) DeleteProfile(profileID string) error {
	if s == nil {
		return store.ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data.Profiles, profileID)
	return s.flushLocked()
}

func (s *Sink) SaveTemplate(template domain.InstanceTemplate) error {
	if s == nil {
		return store.ErrNotFound
	}
	if strings.TrimSpace(template.ID) == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Templates == nil {
		s.data.Templates = map[string]domain.InstanceTemplate{}
	}
	s.data.Templates[template.ID] = template
	return s.flushLocked()
}

func (s *Sink) ListTemplates() ([]domain.InstanceTemplate, error) {
	if s == nil {
		return nil, store.ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.InstanceTemplate, 0, len(s.data.Templates))
	for _, template := range s.data.Templates {
		if strings.TrimSpace(template.ID) == "" {
			continue
		}
		out = append(out, template)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Region == out[j].Region {
			return out[i].Name < out[j].Name
		}
		return out[i].Region < out[j].Region
	})
	return out, nil
}

func (s *Sink) DeleteTemplate(templateID string) error {
	if s == nil {
		return store.ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Templates, templateID)
	return s.flushLocked()
}

func (s *Sink) SaveJob(domain.Job) error {
	return nil
}

func (s *Sink) SaveInstance(domain.Instance) error {
	return nil
}

func (s *Sink) RecordAudit(domain.AuditLog) error {
	return nil
}

func (s *Sink) SaveEmailSettings(settings domain.EmailSettings) error {
	return s.saveSetting("email", settings)
}

func (s *Sink) GetEmailSettings() (domain.EmailSettings, error) {
	var settings domain.EmailSettings
	if err := s.getSetting("email", &settings); err != nil {
		return domain.EmailSettings{}, err
	}
	if settings.Password != "" {
		settings.PasswordSet = true
	}
	return settings, nil
}

func (s *Sink) SaveWebhookSettings(settings domain.WebhookSettings) error {
	return s.saveSetting("webhook", settings)
}

func (s *Sink) GetWebhookSettings() (domain.WebhookSettings, error) {
	var settings domain.WebhookSettings
	if err := s.getSetting("webhook", &settings); err != nil {
		return domain.WebhookSettings{}, err
	}
	if settings.Secret != "" {
		settings.SecretSet = true
	}
	return settings, nil
}

func (s *Sink) SaveAccountSettings(settings domain.AccountSettings) error {
	return s.saveSetting("account", settings)
}

func (s *Sink) GetAccountSettings() (domain.AccountSettings, error) {
	var settings domain.AccountSettings
	if err := s.getSetting("account", &settings); err != nil {
		return domain.AccountSettings{}, err
	}
	if settings.PasswordHash != "" {
		settings.PasswordSet = true
	}
	return settings, nil
}

func (s *Sink) SaveAppearanceSettings(settings domain.AppearanceSettings) error {
	return s.saveSetting("appearance", settings)
}

func (s *Sink) GetAppearanceSettings() (domain.AppearanceSettings, error) {
	var settings domain.AppearanceSettings
	if err := s.getSetting("appearance", &settings); err != nil {
		return domain.AppearanceSettings{}, err
	}
	return settings, nil
}

func (s *Sink) SaveBudgetSettings(settings domain.BudgetSettings) error {
	return s.saveSetting("budget", settings)
}

func (s *Sink) GetBudgetSettings() (domain.BudgetSettings, error) {
	var settings domain.BudgetSettings
	if err := s.getSetting("budget", &settings); err != nil {
		return domain.BudgetSettings{}, err
	}
	return settings, nil
}

func (s *Sink) SaveAccessControlSettings(settings domain.AccessControlSettings) error {
	return s.saveSetting("access", settings)
}

func (s *Sink) GetAccessControlSettings() (domain.AccessControlSettings, error) {
	var settings domain.AccessControlSettings
	if err := s.getSetting("access", &settings); err != nil {
		return domain.AccessControlSettings{}, err
	}
	return settings, nil
}

func (s *Sink) SaveSecurityGuardrailSettings(settings domain.SecurityGuardrailSettings) error {
	return s.saveSetting("guardrails", settings)
}

func (s *Sink) GetSecurityGuardrailSettings() (domain.SecurityGuardrailSettings, error) {
	var settings domain.SecurityGuardrailSettings
	if err := s.getSetting("guardrails", &settings); err != nil {
		return domain.SecurityGuardrailSettings{}, err
	}
	return settings, nil
}

func (s *Sink) saveSetting(key string, value any) error {
	if s == nil {
		return store.ErrNotFound
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Settings == nil {
		s.data.Settings = map[string]json.RawMessage{}
	}
	s.data.Settings[key] = raw
	return s.flushLocked()
}

func (s *Sink) getSetting(key string, out any) error {
	if s == nil {
		return store.ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.Settings) == 0 {
		return nil
	}
	raw := s.data.Settings[strings.TrimSpace(key)]
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (s *Sink) load() error {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return err
	}
	if s.data.Version == 0 {
		s.data.Version = 1
	}
	if s.data.Profiles == nil {
		s.data.Profiles = map[string]profileRow{}
	}
	if s.data.Templates == nil {
		s.data.Templates = map[string]domain.InstanceTemplate{}
	}
	if s.data.Settings == nil {
		s.data.Settings = map[string]json.RawMessage{}
	}
	return nil
}

func (s *Sink) flushLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
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
