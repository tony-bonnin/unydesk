package main

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	hostPasswordMu     sync.RWMutex
	hostAccessPassword string
)

func currentHostAccessPassword() string {
	hostPasswordMu.RLock()
	defer hostPasswordMu.RUnlock()
	return hostAccessPassword
}

func setHostAccessPassword(value string) {
	hostPasswordMu.Lock()
	hostAccessPassword = strings.TrimSpace(value)
	hostPasswordMu.Unlock()
	localHostUI.setAccessPassword(hostAccessPassword)
}

func rotatePersistentHostAccessPassword() (string, error) {
	value, err := generateHostAccessPassword()
	if err != nil {
		return "", err
	}
	syncHostAccessPassword(hostAccessPasswordPaths(), value)
	setHostAccessPassword(value)
	return value, nil
}

func generateHostAccessPassword() (string, error) {
	// Uppercase-only alphanumeric password for easier phone support dictation
	// while still being longer than the previous short code.
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const size = 16

	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	var out strings.Builder
	out.Grow(size)
	for _, value := range buf {
		out.WriteByte(alphabet[int(value)%len(alphabet)])
	}
	return out.String(), nil
}

func hostAccessPasswordPaths() []string {
	paths := make([]string, 0, 3)
	if configDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(configDir) != "" {
		paths = append(paths, filepath.Join(configDir, "UnyDesk", "host-access-password"))
	}
	if homeDir, err := os.UserHomeDir(); err == nil && strings.TrimSpace(homeDir) != "" {
		paths = append(paths, filepath.Join(homeDir, ".unydesk", "host-access-password"))
	}
	switch runtime.GOOS {
	case "windows":
		if programData := strings.TrimSpace(os.Getenv("ProgramData")); programData != "" {
			paths = append(paths, filepath.Join(programData, "UnyDesk", "host-access-password"))
		}
	}

	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	if len(out) == 0 {
		out = append(out, filepath.Join(".", "host-access-password"))
	}
	return out
}

func syncHostAccessPassword(paths []string, value string) {
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			continue
		}
		_ = os.WriteFile(path, []byte(strings.TrimSpace(value)+"\n"), 0o600)
	}
}
