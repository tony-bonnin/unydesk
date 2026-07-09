package remote

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (s *MemoryStore) ConfigurePersistence(hostsPath, trustedHostsPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hostsPath = strings.TrimSpace(hostsPath)
	s.trustedHostsPath = strings.TrimSpace(trustedHostsPath)

	if err := s.loadHostsLocked(); err != nil {
		return err
	}
	if err := s.loadTrustedHostsLocked(); err != nil {
		return err
	}
	return nil
}

func (s *MemoryStore) loadHostsLocked() error {
	if s.hostsPath == "" {
		return nil
	}
	var hosts []Host
	if err := readJSONFile(s.hostsPath, &hosts); err != nil {
		return err
	}
	for _, host := range hosts {
		host = normalizePersistedHost(host)
		if strings.TrimSpace(host.ID) == "" {
			continue
		}
		s.hosts[host.ID] = host
	}
	return nil
}

func (s *MemoryStore) loadTrustedHostsLocked() error {
	if s.trustedHostsPath == "" {
		return nil
	}
	var trustedHosts []TrustedHost
	if err := readJSONFile(s.trustedHostsPath, &trustedHosts); err != nil {
		return err
	}
	for _, trusted := range trustedHosts {
		trusted = normalizeTrustedHost(trusted)
		if strings.TrimSpace(trusted.UserID) == "" || strings.TrimSpace(trusted.HostID) == "" {
			continue
		}
		s.trustedHosts[trustedHostKey(trusted.UserID, trusted.HostID)] = trusted
	}
	return nil
}

func normalizePersistedHost(host Host) Host {
	host.ID = strings.TrimSpace(host.ID)
	host.InstallID = strings.TrimSpace(host.InstallID)
	host.Role = normalizeHostRole(host.Role)
	host.Name = strings.TrimSpace(host.Name)
	host.OS = strings.TrimSpace(host.OS)
	host.Arch = strings.TrimSpace(host.Arch)
	host.Version = strings.TrimSpace(host.Version)
	host.Hostname = strings.TrimSpace(host.Hostname)
	host.PublicID = strings.TrimSpace(host.PublicID)
	host.AccessMode = strings.TrimSpace(host.AccessMode)
	host.PasswordHash = strings.TrimSpace(host.PasswordHash)
	if host.ID == "" && host.InstallID != "" {
		host.ID = StableID("host", host.InstallID)
	}
	if host.PublicID == "" && host.InstallID != "" {
		host.PublicID = StablePublicID("host", host.InstallID)
	}
	if host.Role == "" {
		host.Role = "host"
	}
	if host.AccessMode == "" {
		host.AccessMode = "password"
	}
	return host
}

func normalizeTrustedHost(trusted TrustedHost) TrustedHost {
	trusted.ID = strings.TrimSpace(trusted.ID)
	trusted.UserID = strings.TrimSpace(trusted.UserID)
	trusted.UserEmail = strings.ToLower(strings.TrimSpace(trusted.UserEmail))
	trusted.HostID = strings.TrimSpace(trusted.HostID)
	trusted.HostPublicID = strings.TrimSpace(trusted.HostPublicID)
	trusted.HostInstallID = strings.TrimSpace(trusted.HostInstallID)
	trusted.Hostname = strings.TrimSpace(trusted.Hostname)
	trusted.Name = strings.TrimSpace(trusted.Name)
	if trusted.ID == "" && trusted.UserID != "" && trusted.HostID != "" {
		trusted.ID = StableID("trusted-host", trusted.UserID+":"+trusted.HostID)
	}
	if trusted.TrustedAt.IsZero() {
		trusted.TrustedAt = time.Now().UTC()
	}
	return trusted
}

func readJSONFile(path string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, dest)
}

func writeJSONFile(path string, value any) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *MemoryStore) saveHostsLocked() error {
	if s.hostsPath == "" {
		return nil
	}
	hosts := make([]Host, 0, len(s.hosts))
	for _, host := range s.hosts {
		hosts = append(hosts, host)
	}
	sortHostsStable(hosts)
	return writeJSONFile(s.hostsPath, hosts)
}

func (s *MemoryStore) saveTrustedHostsLocked() error {
	if s.trustedHostsPath == "" {
		return nil
	}
	trustedHosts := make([]TrustedHost, 0, len(s.trustedHosts))
	for _, trusted := range s.trustedHosts {
		trustedHosts = append(trustedHosts, trusted)
	}
	sortTrustedHostsStable(trustedHosts)
	return writeJSONFile(s.trustedHostsPath, trustedHosts)
}

func sortTrustedHostsStable(trustedHosts []TrustedHost) {
	sort.SliceStable(trustedHosts, func(i, j int) bool {
		left := trustedHosts[i]
		right := trustedHosts[j]
		if !left.TrustedAt.Equal(right.TrustedAt) {
			if left.TrustedAt.IsZero() {
				return false
			}
			if right.TrustedAt.IsZero() {
				return true
			}
			return left.TrustedAt.Before(right.TrustedAt)
		}
		return trustedHostSortKey(left) < trustedHostSortKey(right)
	})
}

func trustedHostSortKey(trusted TrustedHost) string {
	parts := []string{
		trusted.UserEmail,
		trusted.Hostname,
		trusted.Name,
		trusted.HostPublicID,
		trusted.HostInstallID,
		trusted.HostID,
	}
	for i, part := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(part))
	}
	return strings.Join(parts, "\x00")
}

func trustedHostKey(userID, hostID string) string {
	return strings.TrimSpace(userID) + "\x00" + strings.TrimSpace(hostID)
}

func (s *MemoryStore) TrustHostForUser(userID, userEmail string, host Host) (TrustedHost, error) {
	userID = strings.TrimSpace(userID)
	hostID := strings.TrimSpace(host.ID)
	if userID == "" || hostID == "" {
		return TrustedHost{}, ErrNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	key := trustedHostKey(userID, hostID)
	trusted := s.trustedHosts[key]
	if trusted.ID == "" {
		trusted = TrustedHost{
			ID:        StableID("trusted-host", userID+":"+hostID),
			UserID:    userID,
			TrustedAt: now,
		}
	}
	trusted.UserEmail = strings.ToLower(strings.TrimSpace(userEmail))
	trusted.HostID = hostID
	trusted.HostPublicID = strings.TrimSpace(host.PublicID)
	trusted.HostInstallID = strings.TrimSpace(host.InstallID)
	trusted.Hostname = strings.TrimSpace(host.Hostname)
	trusted.Name = strings.TrimSpace(host.Name)
	trusted.LastUsedAt = now

	s.trustedHosts[key] = trusted
	_ = s.saveTrustedHostsLocked()
	return trusted, nil
}

func (s *MemoryStore) UntrustHostForUser(userID, target string) bool {
	userID = strings.TrimSpace(userID)
	target = strings.ToLower(strings.TrimSpace(target))
	if userID == "" || target == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	removed := false
	for key, trusted := range s.trustedHosts {
		if strings.TrimSpace(trusted.UserID) != userID {
			continue
		}
		if trustedHostMatchesTarget(trusted, target) {
			delete(s.trustedHosts, key)
			removed = true
		}
	}
	if removed {
		_ = s.saveTrustedHostsLocked()
	}
	return removed
}

func trustedHostMatchesTarget(trusted TrustedHost, target string) bool {
	if target == "" {
		return false
	}
	for _, value := range []string{
		trusted.ID,
		trusted.HostID,
		trusted.HostPublicID,
		trusted.HostInstallID,
		trusted.Hostname,
		trusted.Name,
	} {
		if strings.ToLower(strings.TrimSpace(value)) == target {
			return true
		}
	}
	return false
}

func (s *MemoryStore) TouchTrustedHostUsage(userID, hostID string) {
	userID = strings.TrimSpace(userID)
	hostID = strings.TrimSpace(hostID)
	if userID == "" || hostID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := trustedHostKey(userID, hostID)
	trusted, ok := s.trustedHosts[key]
	if !ok {
		return
	}
	trusted.LastUsedAt = time.Now().UTC()
	if host, exists := s.hosts[hostID]; exists {
		trusted.HostPublicID = strings.TrimSpace(host.PublicID)
		trusted.HostInstallID = strings.TrimSpace(host.InstallID)
		trusted.Hostname = strings.TrimSpace(host.Hostname)
		trusted.Name = strings.TrimSpace(host.Name)
	}
	s.trustedHosts[key] = trusted
	_ = s.saveTrustedHostsLocked()
}

func (s *MemoryStore) ListTrustedHostsForUser(userID string) []TrustedHost {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]TrustedHost, 0)
	for _, trusted := range s.trustedHosts {
		if strings.TrimSpace(trusted.UserID) != userID {
			continue
		}
		out = append(out, trusted)
	}
	sortTrustedHostsStable(out)
	return out
}

func (s *MemoryStore) TrustedHostForUser(userID, hostID string) (TrustedHost, bool) {
	userID = strings.TrimSpace(userID)
	hostID = strings.TrimSpace(hostID)
	if userID == "" || hostID == "" {
		return TrustedHost{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	trusted, ok := s.trustedHosts[trustedHostKey(userID, hostID)]
	return trusted, ok
}

func (s *MemoryStore) FindTrustedHostByTarget(userID, target string, timeout time.Duration) (Host, TrustedHost, bool) {
	userID = strings.TrimSpace(userID)
	needle := strings.ToLower(strings.TrimSpace(target))
	if userID == "" || needle == "" {
		return Host{}, TrustedHost{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()
	hosts := make([]Host, 0, len(s.hosts))
	for _, host := range s.hosts {
		hosts = append(hosts, host)
	}
	sortHostsStable(hosts)

	for _, host := range hosts {
		host.Role = normalizeHostRole(host.Role)
		host.Status = deriveHostStatus(host, timeout, now)
		if host.Status != "online" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(host.ID)) != needle &&
			strings.ToLower(strings.TrimSpace(host.PublicID)) != needle &&
			strings.ToLower(strings.TrimSpace(host.Hostname)) != needle &&
			strings.ToLower(strings.TrimSpace(host.Name)) != needle {
			continue
		}
		trusted, ok := s.trustedHosts[trustedHostKey(userID, host.ID)]
		if ok {
			return host, trusted, true
		}
	}
	return Host{}, TrustedHost{}, false
}

func (s *MemoryStore) syncTrustedHostSnapshotsLocked(host Host) {
	changed := false
	for key, trusted := range s.trustedHosts {
		if strings.TrimSpace(trusted.HostID) != strings.TrimSpace(host.ID) {
			continue
		}
		next := trusted
		next.HostPublicID = strings.TrimSpace(host.PublicID)
		next.HostInstallID = strings.TrimSpace(host.InstallID)
		next.Hostname = strings.TrimSpace(host.Hostname)
		next.Name = strings.TrimSpace(host.Name)
		if next != trusted {
			s.trustedHosts[key] = next
			changed = true
		}
	}
	if changed {
		_ = s.saveTrustedHostsLocked()
	}
}
