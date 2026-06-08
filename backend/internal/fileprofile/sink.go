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
	Version  int                   `json:"version"`
	Profiles map[string]profileRow `json:"profiles"`
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
			Version:  1,
			Profiles: map[string]profileRow{},
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

func (s *Sink) SaveJob(domain.Job) error {
	return nil
}

func (s *Sink) SaveInstance(domain.Instance) error {
	return nil
}

func (s *Sink) RecordAudit(domain.AuditLog) error {
	return nil
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
