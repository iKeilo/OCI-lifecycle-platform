package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	defaultCookieName = "a_series_oracle_session"
	defaultSessionTTL = 12 * time.Hour
)

type Config struct {
	PasswordHash  string
	PlainPassword string
	SessionSecret string
	CookieName    string
	AuthDisabled  bool
}

type Manager struct {
	enabled      bool
	passwordHash []byte
	sessionKey   []byte
	cookieName   string
	ttl          time.Duration
}

func New(cfg Config) (*Manager, error) {
	if cfg.AuthDisabled {
		return &Manager{enabled: false, cookieName: defaultCookieName, ttl: defaultSessionTTL}, nil
	}

	passwordHash := strings.TrimSpace(cfg.PasswordHash)
	if passwordHash == "" && strings.TrimSpace(cfg.PlainPassword) != "" {
		hash, err := HashPassword(cfg.PlainPassword)
		if err != nil {
			return nil, err
		}
		passwordHash = hash
	}
	if passwordHash == "" {
		return &Manager{enabled: false, cookieName: defaultCookieName, ttl: defaultSessionTTL}, nil
	}

	sessionSecret := strings.TrimSpace(cfg.SessionSecret)
	if sessionSecret == "" {
		generated, err := randomSecret()
		if err != nil {
			return nil, err
		}
		sessionSecret = generated
	}

	cookieName := strings.TrimSpace(cfg.CookieName)
	if cookieName == "" {
		cookieName = defaultCookieName
	}

	return &Manager{
		enabled:      true,
		passwordHash: []byte(passwordHash),
		sessionKey:   []byte(sessionSecret),
		cookieName:   cookieName,
		ttl:          defaultSessionTTL,
	}, nil
}

func HashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if len(password) < 8 {
		return "", errors.New("panel password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (m *Manager) Enabled() bool {
	return m != nil && m.enabled
}

func (m *Manager) VerifyPassword(password string) bool {
	if !m.Enabled() {
		return true
	}
	return bcrypt.CompareHashAndPassword(m.passwordHash, []byte(password)) == nil
}

func (m *Manager) IssueSession(w http.ResponseWriter) {
	if !m.Enabled() {
		return
	}
	expires := time.Now().UTC().Add(m.ttl).Unix()
	value := fmt.Sprintf("%d.%s", expires, m.sign(strconv.FormatInt(expires, 10)))
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(m.ttl.Seconds()),
		Expires:  time.Unix(expires, 0).UTC(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *Manager) ClearSession(w http.ResponseWriter) {
	cookieName := defaultCookieName
	if m != nil && m.cookieName != "" {
		cookieName = m.cookieName
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *Manager) IsAuthenticated(r *http.Request) bool {
	if !m.Enabled() {
		return true
	}
	cookie, err := r.Cookie(m.cookieName)
	if err != nil {
		return false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().UTC().Unix() > expires {
		return false
	}
	expected := m.sign(parts[0])
	return hmac.Equal([]byte(expected), []byte(parts[1]))
}

func (m *Manager) sign(value string) string {
	mac := hmac.New(sha256.New, m.sessionKey)
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func randomSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
