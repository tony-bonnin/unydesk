package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	bootstrapRuntimeMu     sync.RWMutex
	bootstrapRuntimeConfig bootstrapConfig
	bootstrapRuntimePath   string
	bootstrapRuntimeWake   = make(chan struct{}, 1)
)

func setBootstrapRuntime(cfg bootstrapConfig, path string) {
	bootstrapRuntimeMu.Lock()
	bootstrapRuntimeConfig = normalizeBootstrapConfig(cfg)
	bootstrapRuntimePath = strings.TrimSpace(path)
	bootstrapRuntimeMu.Unlock()
}

func currentBootstrapRuntime() bootstrapConfig {
	bootstrapRuntimeMu.RLock()
	defer bootstrapRuntimeMu.RUnlock()
	return bootstrapRuntimeConfig
}

func currentBootstrapRuntimePath() string {
	bootstrapRuntimeMu.RLock()
	defer bootstrapRuntimeMu.RUnlock()
	return bootstrapRuntimePath
}

func currentRuntimeServerCredential() string {
	return resolveServerCredential(currentBootstrapRuntime())
}

func waitForProvisioningCredential(ctx context.Context) string {
	for {
		if credential := strings.TrimSpace(currentRuntimeServerCredential()); credential != "" {
			return credential
		}
		select {
		case <-ctx.Done():
			return ""
		case <-bootstrapRuntimeWake:
		}
	}
}

func waitForBootstrapUpdate(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-bootstrapRuntimeWake:
		return true
	}
}

func triggerBootstrapWake() {
	select {
	case bootstrapRuntimeWake <- struct{}{}:
	default:
	}
}

func updateBootstrapProvisioning(serverURL, domain, installID, publicID, credential, credentialType string) error {
	cfg := currentBootstrapRuntime()
	if strings.TrimSpace(serverURL) != "" {
		cfg.Server = strings.TrimSpace(serverURL)
	}
	if strings.TrimSpace(domain) != "" {
		cfg.Domain = strings.TrimSpace(domain)
	}
	if strings.TrimSpace(installID) != "" {
		cfg.InstallID = strings.TrimSpace(installID)
	}
	if strings.TrimSpace(publicID) != "" {
		cfg.PublicID = strings.TrimSpace(publicID)
	}
	cfg.Credential = strings.TrimSpace(credential)
	cfg.CredentialType = strings.TrimSpace(credentialType)
	cfg = normalizeBootstrapConfig(cfg)

	path := currentBootstrapRuntimePath()
	if path == "" {
		path = defaultBootstrapRuntimePath()
	}
	resolvedPath, err := persistBootstrapConfig(path, cfg)
	if err != nil {
		return err
	}

	setBootstrapRuntime(cfg, resolvedPath)
	triggerBootstrapWake()
	return nil
}

func updateBootstrapRouting(serverURL, domain, installID, publicID string) error {
	cfg := currentBootstrapRuntime()
	if strings.TrimSpace(serverURL) != "" {
		cfg.Server = strings.TrimSpace(serverURL)
	}
	if strings.TrimSpace(domain) != "" {
		cfg.Domain = strings.TrimSpace(domain)
	}
	if strings.TrimSpace(installID) != "" {
		cfg.InstallID = strings.TrimSpace(installID)
	}
	if strings.TrimSpace(publicID) != "" {
		cfg.PublicID = strings.TrimSpace(publicID)
	}
	cfg = normalizeBootstrapConfig(cfg)

	path := currentBootstrapRuntimePath()
	if path == "" {
		path = defaultBootstrapRuntimePath()
	}
	resolvedPath, err := persistBootstrapConfig(path, cfg)
	if err != nil {
		return err
	}

	setBootstrapRuntime(cfg, resolvedPath)
	triggerBootstrapWake()
	return nil
}

func normalizeBootstrapConfig(cfg bootstrapConfig) bootstrapConfig {
	cfg.Server = strings.TrimSpace(cfg.Server)
	cfg.Domain = strings.TrimSpace(cfg.Domain)
	cfg.InstallID = strings.TrimSpace(cfg.InstallID)
	cfg.PublicID = strings.TrimSpace(cfg.PublicID)
	cfg.Credential = strings.TrimSpace(cfg.Credential)
	cfg.CredentialType = strings.TrimSpace(cfg.CredentialType)
	if cfg.Credential == "" {
		cfg.CredentialType = ""
	}
	return cfg
}

func bootstrapConfigCandidates() []string {
	executablePath, err := os.Executable()
	executableDir := "."
	if err == nil {
		executableDir = filepath.Dir(executablePath)
	}
	workingDir, wdErr := os.Getwd()
	if wdErr != nil || strings.TrimSpace(workingDir) == "" {
		workingDir = "."
	}

	paths := make([]string, 0, 8)
	if configDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(configDir) != "" {
		paths = append(paths, filepath.Join(configDir, "UnyDesk", "unydesk-host.json"))
	}
	paths = append(paths,
		filepath.Join(workingDir, "unydesk-host.json"),
		filepath.Join(workingDir, "unydesk-host.txt"),
		filepath.Join(workingDir, "server.txt"),
		filepath.Join(executableDir, "unydesk-host.json"),
		filepath.Join(executableDir, "unydesk-host.txt"),
		filepath.Join(executableDir, "server.txt"),
	)
	return uniqueNonEmptyPaths(paths, filepath.Join(".", "unydesk-host.json"))
}

func defaultBootstrapRuntimePath() string {
	candidates := bootstrapConfigCandidates()
	if len(candidates) > 0 {
		return candidates[0]
	}
	return filepath.Join(".", "unydesk-host.json")
}

func saveBootstrapConfigFile(path string, cfg bootstrapConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(normalizeBootstrapConfig(cfg), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func persistBootstrapConfig(path string, cfg bootstrapConfig) (string, error) {
	primaryPath := strings.TrimSpace(path)
	if primaryPath == "" {
		primaryPath = defaultBootstrapRuntimePath()
	}
	if err := saveBootstrapConfigFile(primaryPath, cfg); err == nil {
		return primaryPath, nil
	}

	fallbackPath := defaultBootstrapRuntimePath()
	if fallbackPath == "" || sameBootstrapPath(primaryPath, fallbackPath) {
		return "", saveBootstrapConfigFile(primaryPath, cfg)
	}
	if err := saveBootstrapConfigFile(fallbackPath, cfg); err != nil {
		if primaryErr := saveBootstrapConfigFile(primaryPath, cfg); primaryErr != nil {
			return "", primaryErr
		}
		return fallbackPath, nil
	}
	return fallbackPath, nil
}

func sameBootstrapPath(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return left == right
	}
	if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func uniqueNonEmptyPaths(paths []string, fallback string) []string {
	seen := make(map[string]struct{}, len(paths)+1)
	out := make([]string, 0, len(paths)+1)
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	if len(out) == 0 && strings.TrimSpace(fallback) != "" {
		out = append(out, fallback)
	}
	return out
}
