package remote

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("session not found")

type MemoryStore struct {
	mu                 sync.RWMutex
	sessions           map[string]Session
	hosts              map[string]Host
	screenFrames       map[string]screenFrame
	sessionSubscribers map[string]map[chan Session]struct{}
}

type screenFrame struct {
	contentType string
	data        []byte
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions:           make(map[string]Session),
		hosts:              make(map[string]Host),
		screenFrames:       make(map[string]screenFrame),
		sessionSubscribers: make(map[string]map[chan Session]struct{}),
	}
}

func normalizeSessionViewerAuthMode(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "standalone") {
		return "standalone"
	}
	return "account"
}

func hashStandaloneViewerToken(token string) string {
	value := strings.TrimSpace(token)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
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
	var updated Session

	s.mu.Lock()
	if existingID, ok := s.findMatchingSessionLocked(req); ok {
		session := s.sessions[existingID]
		session = resetSession(session, req, now)
		s.sessions[existingID] = session
		delete(s.screenFrames, existingID)
		updated = session
	} else {
		session := Session{
			ID:             newID(),
			Target:         strings.TrimSpace(req.Target),
			Viewer:         strings.TrimSpace(req.Viewer),
			ViewerAuthMode: normalizeSessionViewerAuthMode(req.ViewerAuthMode),
			Status:         StatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		s.sessions[session.ID] = session
		updated = session
	}
	s.mu.Unlock()

	s.notifySessionUpdated(updated)
	return updated
}

func (s *MemoryStore) CreateRouted(req CreateSessionRequest, host *Host) Session {
	now := time.Now().UTC()
	var session Session
	s.mu.Lock()
	if existingID, ok := s.findMatchingSessionLocked(req); ok {
		session = s.sessions[existingID]
		if session.Status == StatusClosed {
			session = resetSession(session, req, now)
		} else {
			session.Target = strings.TrimSpace(req.Target)
			session.Viewer = strings.TrimSpace(req.Viewer)
			session.ViewerAuthMode = normalizeSessionViewerAuthMode(req.ViewerAuthMode)
			session.UpdatedAt = now
		}
	} else {
		session = newSession(req, now)
	}

	if host != nil {
		hostChanged := strings.TrimSpace(session.RoutedHostID) != "" && session.RoutedHostID != host.ID
		session.RoutedHostID = host.ID
		session.RoutedHostPublicID = host.PublicID
		session.RoutedHostname = host.Hostname
		if hostChanged {
			session.Status = StatusPending
			session.OfferSDP = ""
			session.AnswerSDP = ""
			session.ViewerICECandidates = nil
			session.HostICECandidates = nil
			session.LastHostAckAt = nil
		}
		if session.DispatchState != "accepted" {
			session.DispatchState = "queued"
		}
	}
	session.UpdatedAt = now
	s.sessions[session.ID] = session
	s.mu.Unlock()

	s.notifySessionUpdated(session)
	return session
}

func newSession(req CreateSessionRequest, now time.Time) Session {
	return Session{
		ID:             newID(),
		Target:         strings.TrimSpace(req.Target),
		Viewer:         strings.TrimSpace(req.Viewer),
		ViewerAuthMode: normalizeSessionViewerAuthMode(req.ViewerAuthMode),
		Status:         StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
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

func (s *MemoryStore) SetStandaloneViewerToken(id, token string) (Session, error) {
	hashed := hashStandaloneViewerToken(token)
	if hashed == "" {
		return Session{}, ErrNotFound
	}

	s.mu.Lock()
	session, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return Session{}, ErrNotFound
	}
	session.ViewerAuthMode = "standalone"
	session.StandaloneTokenHash = hashed
	session.UpdatedAt = time.Now().UTC()
	s.sessions[id] = session
	s.mu.Unlock()

	s.notifySessionUpdated(session)
	return session, nil
}

func (s *MemoryStore) ValidateStandaloneViewerToken(id, token string) bool {
	hashed := hashStandaloneViewerToken(token)
	if hashed == "" {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return false
	}
	if normalizeSessionViewerAuthMode(session.ViewerAuthMode) != "standalone" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(session.StandaloneTokenHash), []byte(hashed)) == 1
}

func (s *MemoryStore) SetOffer(id string, sdp string) (Session, error) {
	offer := strings.TrimSpace(sdp)
	s.mu.Lock()
	session, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return Session{}, ErrNotFound
	}
	if strings.TrimSpace(session.OfferSDP) == offer {
		s.mu.Unlock()
		return session, nil
	}
	session.OfferSDP = offer
	session.AnswerSDP = ""
	session.ViewerICECandidates = nil
	session.HostICECandidates = nil
	session.Status = StatusOffered
	if strings.TrimSpace(session.RoutedHostID) != "" {
		session.DispatchState = "queued"
		session.LastHostAckAt = nil
	}
	session.UpdatedAt = time.Now().UTC()
	s.sessions[id] = session
	s.mu.Unlock()

	s.notifySessionUpdated(session)
	return session, nil
}

func (s *MemoryStore) SetAnswer(id string, sdp string) (Session, error) {
	s.mu.Lock()
	session, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return Session{}, ErrNotFound
	}
	session.AnswerSDP = sdp
	session.Status = StatusActive
	if strings.TrimSpace(session.RoutedHostID) != "" {
		session.DispatchState = "accepted"
	}
	session.UpdatedAt = time.Now().UTC()
	s.sessions[id] = session
	s.mu.Unlock()

	s.notifySessionUpdated(session)
	return session, nil
}

func (s *MemoryStore) AddCandidate(id string, candidate string, source string) (Session, error) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return Session{}, ErrNotFound
	}
	s.mu.Lock()
	session, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return Session{}, ErrNotFound
	}
	switch strings.TrimSpace(source) {
	case "host":
		if stringSliceContains(session.HostICECandidates, candidate) {
			s.mu.Unlock()
			return session, nil
		}
		session.HostICECandidates = append(session.HostICECandidates, candidate)
	default:
		if stringSliceContains(session.ViewerICECandidates, candidate) {
			s.mu.Unlock()
			return session, nil
		}
		session.ViewerICECandidates = append(session.ViewerICECandidates, candidate)
	}
	session.UpdatedAt = time.Now().UTC()
	s.sessions[id] = session
	s.mu.Unlock()

	s.notifySessionUpdated(session)
	return session, nil
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == needle {
			return true
		}
	}
	return false
}

func (s *MemoryStore) SetScreenFrame(id, dataURL string, width, height int, captureError string) (Session, error) {
	return s.SetScreenFrameBinary(id, nil, "", dataURL, width, height, captureError)
}

func (s *MemoryStore) SetScreenFrameBinary(id string, frameData []byte, contentType, dataURL string, width, height int, captureError string) (Session, error) {
	s.mu.Lock()
	session, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return Session{}, ErrNotFound
	}

	now := time.Now().UTC()
	if len(frameData) > 0 {
		s.screenFrames[id] = screenFrame{
			contentType: strings.TrimSpace(contentType),
			data:        append([]byte(nil), frameData...),
		}
		session.ScreenDataURL = ""
		session.ScreenRevision = now.UnixNano()
		session.ScreenWidth = width
		session.ScreenHeight = height
		session.ScreenCaptureError = ""
		session.ScreenUpdatedAt = &now
	} else if strings.TrimSpace(dataURL) != "" {
		delete(s.screenFrames, id)
		session.ScreenDataURL = dataURL
		session.ScreenRevision = now.UnixNano()
		session.ScreenWidth = width
		session.ScreenHeight = height
		session.ScreenCaptureError = ""
		session.ScreenUpdatedAt = &now
	} else {
		delete(s.screenFrames, id)
		session.ScreenDataURL = ""
		session.ScreenRevision = 0
		session.ScreenCaptureError = strings.TrimSpace(captureError)
		if session.ScreenUpdatedAt == nil {
			session.ScreenUpdatedAt = &now
		}
	}
	session.UpdatedAt = now
	s.sessions[id] = session
	s.mu.Unlock()

	s.notifySessionUpdated(session)
	return session, nil
}

func (s *MemoryStore) GetScreenFrame(id string) ([]byte, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	frame, ok := s.screenFrames[id]
	if !ok || len(frame.data) == 0 {
		return nil, "", false
	}
	return append([]byte(nil), frame.data...), frame.contentType, true
}

func (s *MemoryStore) Close(id string) (Session, error) {
	s.mu.Lock()
	session, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return Session{}, ErrNotFound
	}
	session.Status = StatusClosed
	session.DispatchState = "closed"
	session.ScreenDataURL = ""
	session.ScreenRevision = 0
	session.UpdatedAt = time.Now().UTC()
	s.sessions[id] = session
	delete(s.screenFrames, id)
	s.mu.Unlock()

	s.notifySessionUpdated(session)
	return session, nil
}

func normalizeHostRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "client":
		return "client"
	default:
		return "host"
	}
}

func deriveHostStatus(host Host, timeout time.Duration, now time.Time) string {
	if timeout > 0 && now.Sub(host.LastSeenAt) > timeout {
		return "offline"
	}
	if !host.AccessEnabled {
		return "paused"
	}
	return "online"
}

func (s *MemoryStore) FindHostByTarget(target string, timeout time.Duration) (Host, bool) {
	needle := strings.ToLower(strings.TrimSpace(target))
	if needle == "" {
		return Host{}, false
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
		if strings.ToLower(strings.TrimSpace(host.ID)) == needle ||
			strings.ToLower(strings.TrimSpace(host.PublicID)) == needle ||
			strings.ToLower(strings.TrimSpace(host.Hostname)) == needle ||
			strings.ToLower(strings.TrimSpace(host.Name)) == needle {
			return host, true
		}
	}
	return Host{}, false
}

func (s *MemoryStore) FindHostByInstallID(installID string) (Host, bool) {
	needle := strings.TrimSpace(installID)
	if needle == "" {
		return Host{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, host := range s.hosts {
		if strings.TrimSpace(host.InstallID) == needle {
			return host, true
		}
	}
	return Host{}, false
}

func (s *MemoryStore) FindHostByPublicID(publicID string) (Host, bool) {
	needle := strings.ToLower(strings.TrimSpace(publicID))
	if needle == "" {
		return Host{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, host := range s.hosts {
		if strings.ToLower(strings.TrimSpace(host.PublicID)) == needle {
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
	viewerAuthMode := normalizeSessionViewerAuthMode(req.ViewerAuthMode)
	var best Session
	var found bool

	for _, session := range s.sessions {
		if strings.TrimSpace(session.Target) != target {
			continue
		}
		if strings.TrimSpace(session.Viewer) != viewer {
			continue
		}
		if normalizeSessionViewerAuthMode(session.ViewerAuthMode) != viewerAuthMode {
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
	session.ViewerAuthMode = normalizeSessionViewerAuthMode(req.ViewerAuthMode)
	session.Status = StatusPending
	session.OfferSDP = ""
	session.AnswerSDP = ""
	session.ViewerICECandidates = nil
	session.HostICECandidates = nil
	session.ScreenDataURL = ""
	session.ScreenRevision = 0
	session.ScreenWidth = 0
	session.ScreenHeight = 0
	session.ScreenCaptureError = ""
	session.ScreenUpdatedAt = nil
	session.DispatchState = ""
	session.DispatchCount = 0
	session.LastDispatchAt = nil
	session.LastHostAckAt = nil
	session.RoutedHostID = ""
	session.RoutedHostPublicID = ""
	session.RoutedHostname = ""
	session.StandaloneTokenHash = ""
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
	updated := make([]Session, 0, len(ids))
	s.mu.Lock()
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
		updated = append(updated, session)
	}
	s.mu.Unlock()

	for _, session := range updated {
		s.notifySessionUpdated(session)
	}
}

func (s *MemoryStore) AcknowledgeSessionForHost(hostID, sessionID, action string) (Session, error) {
	now := time.Now().UTC()
	s.mu.Lock()
	session, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return Session{}, ErrNotFound
	}
	if session.RoutedHostID != hostID {
		s.mu.Unlock()
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
	case "deny", "reject", "decline":
		session.DispatchState = "rejected"
		session.Status = StatusClosed
	case "busy":
		session.DispatchState = "busy"
	default:
		session.DispatchState = "acknowledged"
	}

	s.sessions[sessionID] = session
	s.mu.Unlock()

	s.notifySessionUpdated(session)
	return session, nil
}

func (s *MemoryStore) SubscribeSession(id string) (<-chan Session, func()) {
	ch := make(chan Session, 8)

	s.mu.Lock()
	if _, ok := s.sessionSubscribers[id]; !ok {
		s.sessionSubscribers[id] = make(map[chan Session]struct{})
	}
	s.sessionSubscribers[id][ch] = struct{}{}
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		subscribers, ok := s.sessionSubscribers[id]
		if !ok {
			return
		}
		if _, exists := subscribers[ch]; !exists {
			return
		}
		delete(subscribers, ch)
		if len(subscribers) == 0 {
			delete(s.sessionSubscribers, id)
		}
	}

	return ch, cancel
}

func (s *MemoryStore) notifySessionUpdated(session Session) {
	s.mu.RLock()
	subscribers := s.sessionSubscribers[session.ID]
	targets := make([]chan Session, 0, len(subscribers))
	for ch := range subscribers {
		targets = append(targets, ch)
	}
	s.mu.RUnlock()

	for _, ch := range targets {
		select {
		case ch <- session:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- session:
			default:
			}
		}
	}
}

func (s *MemoryStore) ListHosts() []Host {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Host, 0, len(s.hosts))
	for _, host := range s.hosts {
		out = append(out, host)
	}
	sortHostsStable(out)
	return out
}

func (s *MemoryStore) ListHostsWithTimeout(timeout time.Duration) []Host {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()
	out := make([]Host, 0, len(s.hosts))
	for _, host := range s.hosts {
		cloned := host
		cloned.Role = normalizeHostRole(cloned.Role)
		cloned.Status = deriveHostStatus(cloned, timeout, now)
		out = append(out, cloned)
	}
	sortHostsStable(out)
	return out
}

func sortHostsStable(hosts []Host) {
	sort.SliceStable(hosts, func(i, j int) bool {
		left := hosts[i]
		right := hosts[j]

		if !left.RegisteredAt.Equal(right.RegisteredAt) {
			if left.RegisteredAt.IsZero() {
				return false
			}
			if right.RegisteredAt.IsZero() {
				return true
			}
			return left.RegisteredAt.Before(right.RegisteredAt)
		}

		leftKey := hostStableSortKey(left)
		rightKey := hostStableSortKey(right)
		if leftKey != rightKey {
			return leftKey < rightKey
		}
		return strings.TrimSpace(left.ID) < strings.TrimSpace(right.ID)
	})
}

func hostStableSortKey(host Host) string {
	parts := []string{
		host.Hostname,
		host.Name,
		host.PublicID,
		host.InstallID,
		host.ID,
	}
	for i, part := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(part))
	}
	return strings.Join(parts, "\x00")
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
			ID:            hostID,
			InstallID:     req.InstallID,
			Role:          normalizeHostRole(req.Role),
			AccessEnabled: true,
			PublicID:      publicID,
			RegisteredAt:  now,
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
	host.Role = normalizeHostRole(req.Role)
	if req.AccessEnabled != nil {
		host.AccessEnabled = *req.AccessEnabled
	} else if !exists {
		host.AccessEnabled = true
	}
	if !host.AccessEnabled {
		host.Status = "paused"
	} else {
		host.Status = "online"
	}
	host.LastSeenAt = now

	s.hosts[host.ID] = host
	return host
}

func (s *MemoryStore) findMatchingHostLocked(req RegisterHostRequest) (string, bool) {
	installID := strings.TrimSpace(req.InstallID)
	hostname := strings.ToLower(strings.TrimSpace(req.Hostname))
	osName := strings.ToLower(strings.TrimSpace(req.OS))
	arch := strings.ToLower(strings.TrimSpace(req.Arch))

	for id, host := range s.hosts {
		if installID != "" && strings.TrimSpace(host.InstallID) == installID {
			return id, true
		}
	}

	if installID != "" {
		return "", false
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

	return "", false
}

func (s *MemoryStore) TouchHost(id string) (Host, error) {
	return s.TouchHostState(id, "", nil)
}

func (s *MemoryStore) TouchHostState(id string, role string, accessEnabled *bool) (Host, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	host, ok := s.hosts[id]
	if !ok {
		return Host{}, ErrNotFound
	}
	if strings.TrimSpace(role) != "" {
		host.Role = normalizeHostRole(role)
	} else if host.Role == "" {
		host.Role = "host"
	}
	if accessEnabled != nil {
		host.AccessEnabled = *accessEnabled
	}
	host.LastSeenAt = time.Now().UTC()
	if !host.AccessEnabled {
		host.Status = "paused"
	} else {
		host.Status = "online"
	}
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
