package auth

import (
	"crypto/rand"
	"encoding/base64"
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
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	Avatar       string    `json:"avatar,omitempty"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
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

func sanitizeUser(user User) PublicUser {
	return PublicUser{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Avatar:      user.Avatar,
		CreatedAt:   user.CreatedAt,
	}
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
