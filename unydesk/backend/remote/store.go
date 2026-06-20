package remote

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("session not found")

type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
	hosts    map[string]Host
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]Session),
		hosts:    make(map[string]Host),
	}
}

func (s *MemoryStore) List() []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		out = append(out, session)
	}
	return out
}

func (s *MemoryStore) Create(req CreateSessionRequest) Session {
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	if existingID, ok := s.findMatchingSessionLocked(req); ok {
		session := s.sessions[existingID]
		session = resetSession(session, req, now)
		s.sessions[existingID] = session
		return session
	}

	session := Session{
		ID:        newID(),
		Target:    strings.TrimSpace(req.Target),
		Viewer:    strings.TrimSpace(req.Viewer),
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.sessions[session.ID] = session
	return session
}

func (s *MemoryStore) CreateRouted(req CreateSessionRequest, host *Host) Session {
	session := s.Create(req)
	if host == nil {
		return session
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session = s.sessions[session.ID]
	session.RoutedHostID = host.ID
	session.RoutedHostPublicID = host.PublicID
	session.RoutedHostname = host.Hostname
	session.DispatchState = "queued"
	session.UpdatedAt = time.Now().UTC()
	s.sessions[session.ID] = session
	return session
}

func (s *MemoryStore) Get(id string) (Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	return session, nil
}

func (s *MemoryStore) SetOffer(id string, sdp string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	session.OfferSDP = sdp
	session.Status = StatusOffered
	session.UpdatedAt = time.Now().UTC()
	s.sessions[id] = session
	return session, nil
}

func (s *MemoryStore) SetAnswer(id string, sdp string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	session.AnswerSDP = sdp
	session.Status = StatusActive
	session.UpdatedAt = time.Now().UTC()
	s.sessions[id] = session
	return session, nil
}

func (s *MemoryStore) AddCandidate(id string, candidate string, source string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	switch strings.TrimSpace(source) {
	case "host":
		session.HostICECandidates = append(session.HostICECandidates, candidate)
	default:
		session.ViewerICECandidates = append(session.ViewerICECandidates, candidate)
	}
	session.UpdatedAt = time.Now().UTC()
	s.sessions[id] = session
	return session, nil
}

func (s *MemoryStore) Close(id string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	session.Status = StatusClosed
	session.DispatchState = "closed"
	session.UpdatedAt = time.Now().UTC()
	s.sessions[id] = session
	return session, nil
}

func (s *MemoryStore) FindHostByTarget(target string) (Host, bool) {
	needle := strings.ToLower(strings.TrimSpace(target))
	if needle == "" {
		return Host{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, host := range s.hosts {
		if strings.ToLower(strings.TrimSpace(host.ID)) == needle ||
			strings.ToLower(strings.TrimSpace(host.PublicID)) == needle ||
			strings.ToLower(strings.TrimSpace(host.Hostname)) == needle ||
			strings.ToLower(strings.TrimSpace(host.Name)) == needle {
			return host, true
		}
	}
	return Host{}, false
}

func (s *MemoryStore) FindReusableSession(req CreateSessionRequest) (Session, bool) {
	target := strings.TrimSpace(req.Target)
	viewer := strings.TrimSpace(req.Viewer)
	if target == "" || viewer == "" {
		return Session{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if existingID, ok := s.findMatchingSessionLocked(req); ok {
		return s.sessions[existingID], true
	}

	return Session{}, false
}

func (s *MemoryStore) findMatchingSessionLocked(req CreateSessionRequest) (string, bool) {
	target := strings.TrimSpace(req.Target)
	viewer := strings.TrimSpace(req.Viewer)
	var best Session
	var found bool

	for _, session := range s.sessions {
		if strings.TrimSpace(session.Target) != target {
			continue
		}
		if strings.TrimSpace(session.Viewer) != viewer {
			continue
		}
		if !found || session.UpdatedAt.After(best.UpdatedAt) {
			best = session
			found = true
		}
	}

	if !found {
		return "", false
	}
	return best.ID, true
}

func resetSession(session Session, req CreateSessionRequest, now time.Time) Session {
	session.Target = strings.TrimSpace(req.Target)
	session.Viewer = strings.TrimSpace(req.Viewer)
	session.Status = StatusPending
	session.OfferSDP = ""
	session.AnswerSDP = ""
	session.ViewerICECandidates = nil
	session.HostICECandidates = nil
	session.DispatchState = ""
	session.DispatchCount = 0
	session.LastDispatchAt = nil
	session.LastHostAckAt = nil
	session.RoutedHostID = ""
	session.RoutedHostPublicID = ""
	session.RoutedHostname = ""
	session.UpdatedAt = now
	return session
}

func (s *MemoryStore) ListQueuedSessionsForHost(hostID string) []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Session, 0)
	for _, session := range s.sessions {
		if session.RoutedHostID != hostID {
			continue
		}
		if session.Status == StatusClosed {
			continue
		}
		if session.DispatchState != "queued" {
			continue
		}
		out = append(out, session)
	}
	return out
}

func (s *MemoryStore) MarkSessionsDispatched(ids []string) {
	if len(ids) == 0 {
		return
	}

	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		session, ok := s.sessions[id]
		if !ok {
			continue
		}
		session.DispatchState = "delivered"
		session.DispatchCount++
		session.LastDispatchAt = &now
		session.UpdatedAt = now
		s.sessions[id] = session
	}
}

func (s *MemoryStore) AcknowledgeSessionForHost(hostID, sessionID, action string) (Session, error) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return Session{}, ErrNotFound
	}
	if session.RoutedHostID != hostID {
		return Session{}, ErrNotFound
	}

	session.LastHostAckAt = &now
	session.UpdatedAt = now

	switch strings.TrimSpace(action) {
	case "accept":
		session.DispatchState = "accepted"
		if session.Status == StatusPending {
			session.Status = StatusOffered
		}
	case "busy":
		session.DispatchState = "busy"
	default:
		session.DispatchState = "acknowledged"
	}

	s.sessions[sessionID] = session
	return session, nil
}

func (s *MemoryStore) ListHosts() []Host {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Host, 0, len(s.hosts))
	for _, host := range s.hosts {
		out = append(out, host)
	}
	return out
}

func (s *MemoryStore) ListHostsWithTimeout(timeout time.Duration) []Host {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()
	out := make([]Host, 0, len(s.hosts))
	for _, host := range s.hosts {
		cloned := host
		if timeout > 0 && now.Sub(cloned.LastSeenAt) > timeout {
			cloned.Status = "offline"
		} else {
			cloned.Status = "online"
		}
		out = append(out, cloned)
	}
	return out
}

func (s *MemoryStore) RegisterHost(req RegisterHostRequest) Host {
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	hostID := newID()
	publicID := newPublicID()
	if req.InstallID != "" {
		hostID = StableID("host", req.InstallID)
		publicID = StablePublicID("host", req.InstallID)
	}

	host, exists := s.hosts[hostID]
	if !exists {
		if existingID, ok := s.findMatchingHostLocked(req); ok {
			host = s.hosts[existingID]
			hostID = host.ID
			if req.InstallID != "" {
				hostID = StableID("host", req.InstallID)
				publicID = StablePublicID("host", req.InstallID)
				if existingID != hostID {
					delete(s.hosts, existingID)
				}
			}
			exists = true
		}
	}
	if !exists {
		host = Host{
			ID:           hostID,
			InstallID:    req.InstallID,
			PublicID:     publicID,
			RegisteredAt: now,
		}
	}

	host.ID = hostID
	host.PublicID = publicID
	host.Name = req.Name
	host.OS = req.OS
	host.Arch = req.Arch
	host.Version = req.Version
	host.Hostname = req.Hostname
	host.InstallID = req.InstallID
	host.Status = "online"
	host.LastSeenAt = now

	s.hosts[host.ID] = host
	return host
}

func (s *MemoryStore) findMatchingHostLocked(req RegisterHostRequest) (string, bool) {
	installID := strings.TrimSpace(req.InstallID)
	hostname := strings.ToLower(strings.TrimSpace(req.Hostname))
	name := strings.ToLower(strings.TrimSpace(req.Name))
	osName := strings.ToLower(strings.TrimSpace(req.OS))
	arch := strings.ToLower(strings.TrimSpace(req.Arch))

	for id, host := range s.hosts {
		if installID != "" && strings.TrimSpace(host.InstallID) == installID {
			return id, true
		}
	}

	for id, host := range s.hosts {
		if hostname == "" || osName == "" || arch == "" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(host.Hostname)) == hostname &&
			strings.ToLower(strings.TrimSpace(host.OS)) == osName &&
			strings.ToLower(strings.TrimSpace(host.Arch)) == arch {
			return id, true
		}
	}

	for id, host := range s.hosts {
		if name == "" || osName == "" || arch == "" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(host.Name)) == name &&
			strings.ToLower(strings.TrimSpace(host.OS)) == osName &&
			strings.ToLower(strings.TrimSpace(host.Arch)) == arch {
			return id, true
		}
	}

	return "", false
}

func (s *MemoryStore) TouchHost(id string) (Host, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	host, ok := s.hosts[id]
	if !ok {
		return Host{}, ErrNotFound
	}
	host.LastSeenAt = time.Now().UTC()
	host.Status = "online"
	s.hosts[id] = host
	return host, nil
}

func newPublicID() string {
	var raw [9]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "000 000 000"
	}
	for i := range raw {
		raw[i] = '0' + (raw[i] % 10)
	}
	return string(raw[0:3]) + " " + string(raw[3:6]) + " " + string(raw[6:9])
}

func newID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		now := time.Now().UTC().UnixNano()
		return hex.EncodeToString([]byte(time.Unix(0, now).UTC().Format("150405.000000000")))
	}
	return hex.EncodeToString(raw[:])
}
