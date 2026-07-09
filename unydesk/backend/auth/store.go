package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already exists")
	ErrInvalidSession     = errors.New("invalid session")
)

type User struct {
	ID                  string               `json:"id"`
	Email               string               `json:"email"`
	DisplayName         string               `json:"display_name"`
	Avatar              string               `json:"avatar,omitempty"`
	PasswordHash        string               `json:"password_hash"`
	HostProvisionTokens []HostProvisionToken `json:"host_provision_tokens,omitempty"`
	CreatedAt           time.Time            `json:"created_at"`
}

type HostProvisionToken struct {
	ID         string    `json:"id"`
	Label      string    `json:"label,omitempty"`
	InstallID  string    `json:"install_id,omitempty"`
	PublicID   string    `json:"public_id,omitempty"`
	Hostname   string    `json:"hostname,omitempty"`
	TokenHash  string    `json:"token_hash"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at,omitempty"`
}

type PublicUser struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Avatar      string    `json:"avatar,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Store struct {
	mu       sync.RWMutex
	path     string
	users    map[string]User
	sessions map[string]string
}

func NewStore(path string) (*Store, error) {
	s := &Store{
		path:     path,
		users:    make(map[string]User),
		sessions: make(map[string]string),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}
	for _, user := range users {
		s.users[strings.ToLower(strings.TrimSpace(user.Email))] = user
	}
	return nil
}

func (s *Store) saveLocked() error {
	users := make([]User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o640)
}

func (s *Store) Register(email, password, displayName string) (PublicUser, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	displayName = strings.TrimSpace(displayName)
	if email == "" || password == "" {
		return PublicUser{}, "", ErrInvalidCredentials
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[email]; exists {
		return PublicUser{}, "", ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return PublicUser{}, "", err
	}

	user := User{
		ID:           newToken(12),
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}
	s.users[email] = user
	if err := s.saveLocked(); err != nil {
		delete(s.users, email)
		return PublicUser{}, "", err
	}

	token := newToken(32)
	s.sessions[token] = email
	return sanitizeUser(user), token, nil
}

func (s *Store) Login(email, password string) (PublicUser, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[email]
	if !exists {
		return PublicUser{}, "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return PublicUser{}, "", ErrInvalidCredentials
	}

	token := newToken(32)
	s.sessions[token] = email
	return sanitizeUser(user), token, nil
}

func (s *Store) ValidateCredentials(email, password string) (PublicUser, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.TrimSpace(password) == "" {
		return PublicUser{}, ErrInvalidCredentials
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[email]
	if !exists {
		return PublicUser{}, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return PublicUser{}, ErrInvalidCredentials
	}
	return sanitizeUser(user), nil
}

func (s *Store) CurrentUser(sessionToken string) (PublicUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	email, ok := s.sessions[strings.TrimSpace(sessionToken)]
	if !ok {
		return PublicUser{}, ErrInvalidSession
	}
	user, exists := s.users[email]
	if !exists {
		return PublicUser{}, ErrInvalidSession
	}
	return sanitizeUser(user), nil
}

func (s *Store) Logout(sessionToken string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, strings.TrimSpace(sessionToken))
}

func (s *Store) UpdatePassword(email, currentPassword, newPassword string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.TrimSpace(currentPassword) == "" || strings.TrimSpace(newPassword) == "" {
		return ErrInvalidCredentials
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[email]
	if !exists {
		return ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	s.users[email] = user
	return s.saveLocked()
}

func (s *Store) UpdateProfile(currentEmail, newEmail, displayName, avatar string) (PublicUser, error) {
	currentEmail = strings.ToLower(strings.TrimSpace(currentEmail))
	newEmail = strings.ToLower(strings.TrimSpace(newEmail))
	displayName = strings.TrimSpace(displayName)
	avatar = strings.ToUpper(strings.TrimSpace(avatar))
	if currentEmail == "" || newEmail == "" {
		return PublicUser{}, ErrInvalidCredentials
	}
	if displayName == "" {
		displayName = strings.Split(newEmail, "@")[0]
	}
	if len(avatar) > 2 {
		avatar = avatar[:2]
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[currentEmail]
	if !exists {
		return PublicUser{}, ErrInvalidCredentials
	}
	if currentEmail != newEmail {
		if _, taken := s.users[newEmail]; taken {
			return PublicUser{}, ErrEmailTaken
		}
		delete(s.users, currentEmail)
		for token, email := range s.sessions {
			if email == currentEmail {
				s.sessions[token] = newEmail
			}
		}
	}

	user.Email = newEmail
	user.DisplayName = displayName
	user.Avatar = avatar
	s.users[newEmail] = user
	if err := s.saveLocked(); err != nil {
		return PublicUser{}, err
	}
	return sanitizeUser(user), nil
}

func (s *Store) IssueProvisionToken(email, label, installID, publicID, hostname string) (HostProvisionToken, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return HostProvisionToken{}, "", ErrInvalidCredentials
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[email]
	if !exists {
		return HostProvisionToken{}, "", ErrInvalidCredentials
	}

	plainToken := "uph_" + newToken(24)
	hashedToken := hashProvisionToken(plainToken)
	now := time.Now().UTC()
	token := HostProvisionToken{
		ID:        newToken(12),
		Label:     strings.TrimSpace(label),
		InstallID: strings.TrimSpace(installID),
		PublicID:  strings.TrimSpace(publicID),
		Hostname:  strings.TrimSpace(hostname),
		TokenHash: hashedToken,
		CreatedAt: now,
	}

	filtered := make([]HostProvisionToken, 0, len(user.HostProvisionTokens)+1)
	for _, existing := range user.HostProvisionTokens {
		if token.InstallID != "" && strings.EqualFold(strings.TrimSpace(existing.InstallID), token.InstallID) {
			continue
		}
		if token.PublicID != "" && strings.EqualFold(strings.TrimSpace(existing.PublicID), token.PublicID) {
			continue
		}
		if token.InstallID == "" && token.PublicID == "" &&
			strings.TrimSpace(existing.InstallID) == "" &&
			strings.TrimSpace(existing.PublicID) == "" &&
			strings.EqualFold(strings.TrimSpace(existing.Label), token.Label) {
			continue
		}
		filtered = append(filtered, existing)
	}
	filtered = append(filtered, token)
	user.HostProvisionTokens = filtered
	s.users[email] = user
	if err := s.saveLocked(); err != nil {
		return HostProvisionToken{}, "", err
	}
	return token, plainToken, nil
}

func (s *Store) ValidateProvisionToken(token string) (PublicUser, HostProvisionToken, error) {
	hashedToken := hashProvisionToken(token)
	if hashedToken == "" {
		return PublicUser{}, HostProvisionToken{}, ErrInvalidCredentials
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for email, user := range s.users {
		for idx, provision := range user.HostProvisionTokens {
			if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(provision.TokenHash)), []byte(hashedToken)) != 1 {
				continue
			}
			provision.LastUsedAt = time.Now().UTC()
			user.HostProvisionTokens[idx] = provision
			s.users[email] = user
			if err := s.saveLocked(); err != nil {
				return PublicUser{}, HostProvisionToken{}, err
			}
			return sanitizeUser(user), provision, nil
		}
	}
	return PublicUser{}, HostProvisionToken{}, ErrInvalidCredentials
}

func sanitizeUser(user User) PublicUser {
	return PublicUser{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Avatar:      user.Avatar,
		CreatedAt:   user.CreatedAt,
	}
}

func hashProvisionToken(token string) string {
	value := strings.TrimSpace(token)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func newToken(size int) string {
	if size <= 0 {
		size = 32
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
