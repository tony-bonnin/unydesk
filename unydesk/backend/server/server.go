package server

import (
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/quic-go/quic-go/http3"
	"unydesk/auth"
	"unydesk/config"
	"unydesk/remote"
)

//go:embed assets/*
var embeddedAssets embed.FS

var frontendAssetRefPattern = regexp.MustCompile(`((?:href|src)=["'])(/[^"'?#]+\.(?:css|js|mjs|ico|jpg|jpeg|png|svg|webp))(?:\?[^"']*)?(["'])`)

const (
	csrfCookieName         = "unydesk_csrf"
	csrfHeaderName         = "X-CSRF-Token"
	authCookieName         = "unydesk_session"
	browserTokenCookieName = "unydesk_browser_token"
	standaloneTokenHeader  = "X-UnyDesk-Standalone-Token"
)

type Server struct {
	httpServer     *http.Server
	http3Server    *http3.Server
	redirectServer *http.Server
	addr           string
	http3Addr      string
	logger         *slog.Logger
	cfg            config.Settings
	store          *remote.MemoryStore
	authStore      *auth.Store
	publicID       string
	hostMu         sync.RWMutex
	hostConns      map[string]*hostConn
	eventMu        sync.RWMutex
	eventSubs      map[string]map[*hostConn]struct{}
	screenMu       sync.RWMutex
	screenSubs     map[string]*screenRelay
}

type hostConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

type screenRelay struct {
	host    *hostConn
	viewers map[*hostConn]struct{}
}

func (c *hostConn) writeJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

func (c *hostConn) writeMessage(messageType int, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(messageType, payload)
}

func (c *hostConn) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.Close()
}

func resolveRequestHostIPv4(r *http.Request) string {
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if idx := strings.Index(host, ","); idx >= 0 {
		host = strings.TrimSpace(host[:idx])
	}
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	}
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return ""
	}
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}
	return ""
}

func resolveRequestClientIPv4(r *http.Request) string {
	for _, headerName := range []string{"X-UnyDesk-Client-IP", "CF-Connecting-IP", "True-Client-IP", "X-Client-IP", "X-Forwarded-For", "X-Real-IP"} {
		raw := strings.TrimSpace(r.Header.Get(headerName))
		if raw == "" {
			continue
		}
		for _, part := range strings.Split(raw, ",") {
			if ip := parseIPv4(strings.TrimSpace(part)); ip != "" {
				return ip
			}
		}
	}
	if ip := parseForwardedIPv4(r.Header.Get("Forwarded")); ip != "" {
		return ip
	}
	return parseIPv4(r.RemoteAddr)
}

func parseForwardedIPv4(value string) string {
	for _, entry := range strings.Split(value, ",") {
		for _, part := range strings.Split(entry, ";") {
			key, rawValue, ok := strings.Cut(strings.TrimSpace(part), "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(key), "for") {
				continue
			}
			candidate := strings.Trim(strings.TrimSpace(rawValue), `"`)
			if strings.HasPrefix(candidate, "_") {
				continue
			}
			if strings.HasPrefix(candidate, "[") {
				if end := strings.Index(candidate, "]"); end >= 0 {
					candidate = candidate[1:end]
				}
			}
			if ip := parseIPv4(candidate); ip != "" {
				return ip
			}
		}
	}
	return ""
}

func parseIPv4(value string) string {
	if value == "" {
		return ""
	}
	if splitHost, _, err := net.SplitHostPort(value); err == nil {
		value = splitHost
	}
	value = strings.Trim(value, "[]")
	if ip := net.ParseIP(value); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}
	return ""
}

func New(cfg config.Settings, store *remote.MemoryStore, authStore *auth.Store, logger *slog.Logger) *Server {
	publicID, err := loadOrCreatePublicID(cfg.Paths.PublicIDFile)
	if err != nil {
		logger.Warn("public id bootstrap failed", "err", err)
		publicID = generatePublicID()
	}

	s := &Server{
		logger:     logger,
		cfg:        cfg,
		store:      store,
		authStore:  authStore,
		publicID:   publicID,
		hostConns:  make(map[string]*hostConn),
		eventSubs:  make(map[string]map[*hostConn]struct{}),
		screenSubs: make(map[string]*screenRelay),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/download/host/", s.handleHostDownload)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/v1/info", s.handleInfo)
	mux.HandleFunc("/api/v1/browser/identity", s.handleBrowserIdentity)
	mux.HandleFunc("/api/v1/standalone/session", s.handleStandaloneSession)
	mux.HandleFunc("/api/v1/auth/register", s.handleAuthRegister)
	mux.HandleFunc("/api/v1/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/v1/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("/api/v1/auth/session", s.handleAuthSession)
	mux.HandleFunc("/api/v1/auth/password", s.handleAuthPassword)
	mux.HandleFunc("/api/v1/profile", s.handleProfile)
	mux.HandleFunc("/api/v1/bootstrap/claim", s.handleBootstrapClaim)
	mux.HandleFunc("/api/v1/bootstrap/provision", s.handleBootstrapProvision)
	mux.HandleFunc("/api/v1/hosts", s.handleHosts)
	mux.HandleFunc("/api/v1/hosts/ws", s.handleHostsWS)
	mux.HandleFunc("/api/v1/admin/hosts", s.handleAdminHosts)
	mux.HandleFunc("/api/v1/admin/trusted-hosts", s.handleAdminTrustedHosts)
	mux.HandleFunc("/api/v1/admin/hosts/connect", s.handleAdminHostConnect)
	mux.HandleFunc("/api/v1/admin/hosts/trust", s.handleAdminHostTrust)
	mux.HandleFunc("/api/v1/admin/hosts/untrust", s.handleAdminHostUntrust)
	mux.HandleFunc("/api/v1/admin/hosts/bulk/trust", s.handleAdminHostsBulkTrust)
	mux.HandleFunc("/api/v1/sessions", s.handleSessions)
	mux.HandleFunc("/api/v1/sessions/", s.handleSessionByID)
	s.registerFrontendRoutes(mux)

	handler := s.withCORS(s.withSecurityHeaders(s.withCSRFCookie(s.withCSRFProtection(s.withLogging(mux)))))
	s.configureTransport(handler)

	return s
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"service":   s.cfg.Name,
		"version":   config.Version,
		"public_id": s.publicID,
	})
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":                   s.cfg.Name,
		"version":                config.Version,
		"listen_addr":            s.cfg.ListenAddr,
		"host_ipv4":              resolveRequestHostIPv4(r),
		"client_ipv4":            resolveRequestClientIPv4(r),
		"allow_origin":           s.cfg.Security.AllowOrigin,
		"session_ttl":            s.cfg.Remote.SessionTTLMinutes,
		"public_id":              s.publicID,
		"host_heartbeat_seconds": s.cfg.Remote.HostHeartbeatSeconds,
		"features":               s.cfg.Features,
		"ice_servers":            s.cfg.WebRTC.ICEServers,
		"webrtc": map[string]any{
			"enabled":                s.cfg.Features.WebRTC,
			"ice_servers":            s.cfg.WebRTC.ICEServers,
			"preferred_video_codecs": s.cfg.Features.PreferredVideoCodecs,
		},
		"http3": map[string]any{
			"enabled":         s.cfg.HTTP3.Enabled,
			"listen_addr":     s.addr,
			"quic_addr":       s.http3Addr,
			"port":            s.cfg.HTTP3.Port,
			"redirect_http":   s.cfg.HTTP3.RedirectHTTP,
			"cert_configured": strings.TrimSpace(s.cfg.HTTP3.CertFile) != "" && strings.TrimSpace(s.cfg.HTTP3.KeyFile) != "",
		},
		"quic": map[string]any{
			"enabled": s.cfg.Features.QUIC,
			"mode":    "monolithic",
			"status":  "reserved-monolithic",
		},
		"frontend_dir":             s.cfg.Paths.FrontendDir,
		"frontend_live_reload":     true,
		"frontend_reload_strategy": "serve-from-disk-and-refresh",
		"downloads": map[string]string{
			"linux_amd64_host":   "/download/host/linux-amd64",
			"linux_amd64_zip":    "/download/host/linux-amd64.zip",
			"linux_arm64_host":   "/download/host/linux-arm64",
			"linux_arm64_zip":    "/download/host/linux-arm64.zip",
			"windows_amd64_host": "/download/host/windows-amd64",
			"windows_amd64_zip":  "/download/host/windows-amd64.zip",
			"windows_arm64_host": "/download/host/windows-arm64",
			"windows_arm64_zip":  "/download/host/windows-arm64.zip",
			"macos_amd64_host":   "/download/host/macos-amd64",
			"macos_amd64_zip":    "/download/host/macos-amd64.zip",
			"macos_arm64_host":   "/download/host/macos-arm64",
			"macos_arm64_zip":    "/download/host/macos-arm64.zip",
			"checksums":          "/download/host/checksums",
		},
		"host_api": map[string]string{
			"register": "/api/v1/hosts",
			"list":     "/api/v1/hosts",
			"ws":       "/api/v1/hosts/ws",
		},
		"auth_api": map[string]string{
			"login":    "/api/v1/auth/login",
			"logout":   "/api/v1/auth/logout",
			"session":  "/api/v1/auth/session",
			"password": "/api/v1/auth/password",
			"profile":  "/api/v1/profile",
		},
		"admin_api": map[string]string{
			"hosts":             "/api/v1/admin/hosts",
			"trusted_hosts":     "/api/v1/admin/trusted-hosts",
			"connect_host":      "/api/v1/admin/hosts/connect",
			"trust_host":        "/api/v1/admin/hosts/trust",
			"untrust_host":      "/api/v1/admin/hosts/untrust",
			"bulk_trust_hosts":  "/api/v1/admin/hosts/bulk/trust",
			"create_session":    "/api/v1/sessions",
			"list_sessions":     "/api/v1/sessions",
			"close_session_url": "/api/v1/sessions/{id}/close",
		},
	})
}

func (s *Server) handleBrowserIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		token = strings.TrimSpace(readCookieValue(r, browserTokenCookieName))
	}
	if !isValidInstallID(token) {
		token = newCSRFToken()
	}

	http.SetCookie(w, &http.Cookie{
		Name:     browserTokenCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
		MaxAge:   60 * 60 * 24 * 365 * 10,
	})

	identityID := remote.StableID("host", token)
	identityPublicID := remote.StablePublicID("host", token)
	payload := map[string]any{
		"id":         identityID,
		"public_id":  identityPublicID,
		"token":      token,
		"role":       "client",
		"kind":       "install",
		"linked":     false,
		"install_id": token,
	}

	if host, ok := s.store.FindHostByInstallID(token); ok {
		payload["linked"] = true
		payload["hostname"] = host.Hostname
		payload["host_registered"] = true
		payload["host_role"] = host.Role
		payload["host_access_enabled"] = host.AccessEnabled
		payload["host_online"] = strings.EqualFold(strings.TrimSpace(host.Status), "online")
		payload["host_id"] = host.ID
	}

	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleAuthRegister(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusForbidden, "self-registration is disabled; manage users through users.json")
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	user, sessionToken, err := s.authStore.Login(req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	s.setAuthCookie(w, r, sessionToken)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if token := readCookieValue(r, authCookieName); token != "" {
		s.authStore.Logout(token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAuthSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	token := readCookieValue(r, authCookieName)
	if token == "" {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	user, err := s.authStore.CurrentUser(token)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user":          user,
	})
}

func (s *Server) handleAuthPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := s.authStore.UpdatePassword(user.Email, req.CurrentPassword, req.NewPassword); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, user)
	case http.MethodPost:
		var req struct {
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
			Avatar      string `json:"avatar"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		updated, err := s.authStore.UpdateProfile(user.Email, req.Email, req.DisplayName, req.Avatar)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, auth.ErrEmailTaken) {
				status = http.StatusConflict
			}
			writeError(w, status, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"user": updated,
		})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentUserFromRequest(r); !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"sessions": s.store.List()})
	case http.MethodPost:
		var req remote.CreateSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if strings.TrimSpace(req.Target) == "" || strings.TrimSpace(req.Viewer) == "" {
			writeError(w, http.StatusBadRequest, "target and viewer are required")
			return
		}
		timeout := time.Duration(s.cfg.Remote.HostHeartbeatSeconds) * time.Second
		if host, ok := s.store.FindHostByTarget(req.Target, timeout); ok {
			writeJSON(w, http.StatusCreated, s.store.CreateRouted(req, &host))
			return
		}
		writeJSON(w, http.StatusCreated, s.store.Create(req))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleStandaloneSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Target      string `json:"target"`
		Viewer      string `json:"viewer"`
		ViewerLabel string `json:"viewer_label,omitempty"`
		Password    string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	target := strings.TrimSpace(req.Target)
	viewer := strings.TrimSpace(req.Viewer)
	password := strings.TrimSpace(req.Password)
	if target == "" || viewer == "" || password == "" {
		writeError(w, http.StatusBadRequest, "target, viewer and password are required")
		return
	}

	createReq := remote.CreateSessionRequest{
		Target:         target,
		Viewer:         viewer,
		ViewerLabel:    strings.TrimSpace(req.ViewerLabel),
		ViewerAuthMode: "standalone",
	}

	timeout := time.Duration(s.cfg.Remote.HostHeartbeatSeconds) * time.Second
	host, ok := s.store.ValidateHostAccessPassword(createReq.Target, password, timeout)
	if !ok {
		if _, available := s.store.FindHostByTarget(createReq.Target, timeout); available {
			writeError(w, http.StatusForbidden, "invalid host password")
			return
		}
		writeError(w, http.StatusNotFound, "target host is offline or unavailable")
		return
	}
	session := s.store.CreateRouted(createReq, &host)

	viewerToken, err := newStandaloneViewerToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unable to provision viewer session token")
		return
	}
	session, err = s.store.SetStandaloneViewerToken(session.ID, viewerToken)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           session.ID,
		"session":      session,
		"viewer_token": viewerToken,
	})
}

func (s *Server) handleHosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		user, ok := s.currentUserFromRequest(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		timeout := time.Duration(s.cfg.Remote.HostHeartbeatSeconds) * time.Second
		writeJSON(w, http.StatusOK, map[string]any{
			"hosts":                     s.hostViewsForUser(user, timeout),
			"heartbeat_timeout_seconds": s.cfg.Remote.HostHeartbeatSeconds,
		})
	case http.MethodPost:
		if _, ok := s.provisioningUserFromRequest(r); !ok {
			writeError(w, http.StatusUnauthorized, "provisioning credentials required")
			return
		}
		var req remote.RegisterHostRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.OS) == "" || strings.TrimSpace(req.Arch) == "" {
			writeError(w, http.StatusBadRequest, "name, os and arch are required")
			return
		}
		writeJSON(w, http.StatusCreated, s.store.RegisterHost(req))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type hostAccountView struct {
	remote.Host
	Trusted           bool       `json:"trusted"`
	TrustedAt         *time.Time `json:"trusted_at,omitempty"`
	TrustedLastUsedAt *time.Time `json:"trusted_last_used_at,omitempty"`
}

type trustedHostAccountView struct {
	remote.TrustedHost
	Host *remote.Host `json:"host,omitempty"`
}

func (s *Server) hostViewsForUser(user auth.PublicUser, timeout time.Duration) []hostAccountView {
	hosts := s.store.ListHostsWithTimeout(timeout)
	trustedHosts := s.store.ListTrustedHostsForUser(user.ID)
	trustedByHostID := make(map[string]remote.TrustedHost, len(trustedHosts))
	for _, trusted := range trustedHosts {
		trustedByHostID[strings.TrimSpace(trusted.HostID)] = trusted
	}

	out := make([]hostAccountView, 0, len(hosts))
	for _, host := range hosts {
		view := hostAccountView{Host: host}
		if trusted, ok := trustedByHostID[strings.TrimSpace(host.ID)]; ok {
			view.Trusted = true
			trustedAt := trusted.TrustedAt
			view.TrustedAt = &trustedAt
			if !trusted.LastUsedAt.IsZero() {
				lastUsedAt := trusted.LastUsedAt
				view.TrustedLastUsedAt = &lastUsedAt
			}
		}
		out = append(out, view)
	}
	return out
}

func (s *Server) trustedHostViewsForUser(user auth.PublicUser, timeout time.Duration) []trustedHostAccountView {
	trustedHosts := s.store.ListTrustedHostsForUser(user.ID)
	hosts := s.store.ListHostsWithTimeout(timeout)
	hostByID := make(map[string]remote.Host, len(hosts))
	for _, host := range hosts {
		hostByID[strings.TrimSpace(host.ID)] = host
	}

	out := make([]trustedHostAccountView, 0, len(trustedHosts))
	for _, trusted := range trustedHosts {
		view := trustedHostAccountView{TrustedHost: trusted}
		if host, ok := hostByID[strings.TrimSpace(trusted.HostID)]; ok {
			hostCopy := host
			view.Host = &hostCopy
		}
		out = append(out, view)
	}
	return out
}

func (s *Server) browserViewerPublicIDFromRequest(r *http.Request) string {
	token := strings.TrimSpace(readCookieValue(r, browserTokenCookieName))
	if !isValidInstallID(token) {
		return ""
	}
	return remote.StablePublicID("host", token)
}

func (s *Server) handleAdminHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	timeout := time.Duration(s.cfg.Remote.HostHeartbeatSeconds) * time.Second
	writeJSON(w, http.StatusOK, map[string]any{
		"hosts":                     s.hostViewsForUser(user, timeout),
		"trusted_hosts":             s.trustedHostViewsForUser(user, timeout),
		"heartbeat_timeout_seconds": s.cfg.Remote.HostHeartbeatSeconds,
	})
}

func (s *Server) handleAdminTrustedHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	timeout := time.Duration(s.cfg.Remote.HostHeartbeatSeconds) * time.Second
	writeJSON(w, http.StatusOK, map[string]any{
		"trusted_hosts": s.trustedHostViewsForUser(user, timeout),
	})
}

func (s *Server) handleAdminHostTrust(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req struct {
		Target   string `json:"target"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	target := strings.TrimSpace(req.Target)
	password := strings.TrimSpace(req.Password)
	if target == "" || password == "" {
		writeError(w, http.StatusBadRequest, "target and password are required")
		return
	}

	timeout := time.Duration(s.cfg.Remote.HostHeartbeatSeconds) * time.Second
	host, ok := s.store.ValidateHostAccessPassword(target, password, timeout)
	if !ok {
		if _, available := s.store.FindHostByTarget(target, timeout); available {
			writeError(w, http.StatusForbidden, "invalid host password")
			return
		}
		writeError(w, http.StatusNotFound, "target host is offline or unavailable")
		return
	}

	trusted, err := s.store.TrustHostForUser(user.ID, user.Email, host)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unable to trust host")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":           true,
		"host":         host,
		"trusted_host": trusted,
	})
}

func (s *Server) handleAdminHostsBulkTrust(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req struct {
		Items []struct {
			Target   string `json:"target"`
			Password string `json:"password"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	timeout := time.Duration(s.cfg.Remote.HostHeartbeatSeconds) * time.Second
	results := make([]map[string]any, 0, len(req.Items))
	for _, item := range req.Items {
		target := strings.TrimSpace(item.Target)
		password := strings.TrimSpace(item.Password)
		result := map[string]any{"target": target}
		if target == "" || password == "" {
			result["ok"] = false
			result["error"] = "target and password are required"
			results = append(results, result)
			continue
		}
		host, matched := s.store.ValidateHostAccessPassword(target, password, timeout)
		if !matched {
			if _, available := s.store.FindHostByTarget(target, timeout); available {
				result["ok"] = false
				result["error"] = "invalid host password"
			} else {
				result["ok"] = false
				result["error"] = "target host is offline or unavailable"
			}
			results = append(results, result)
			continue
		}
		trusted, err := s.store.TrustHostForUser(user.ID, user.Email, host)
		if err != nil {
			result["ok"] = false
			result["error"] = "unable to trust host"
			results = append(results, result)
			continue
		}
		result["ok"] = true
		result["host"] = host
		result["trusted_host"] = trusted
		results = append(results, result)
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleAdminHostUntrust(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req struct {
		Target string `json:"target"`
		HostID string `json:"host_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	target := strings.TrimSpace(req.HostID)
	if target == "" {
		target = strings.TrimSpace(req.Target)
	}
	if target == "" {
		writeError(w, http.StatusBadRequest, "target or host_id is required")
		return
	}
	if !s.store.UntrustHostForUser(user.ID, target) {
		writeError(w, http.StatusNotFound, "trusted host not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAdminHostConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req struct {
		Target      string `json:"target"`
		Viewer      string `json:"viewer"`
		ViewerLabel string `json:"viewer_label,omitempty"`
		Password    string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	target := strings.TrimSpace(req.Target)
	viewer := strings.TrimSpace(req.Viewer)
	password := strings.TrimSpace(req.Password)
	if viewer == "" {
		viewer = s.browserViewerPublicIDFromRequest(r)
	}
	if target == "" || viewer == "" {
		writeError(w, http.StatusBadRequest, "target and viewer are required")
		return
	}

	timeout := time.Duration(s.cfg.Remote.HostHeartbeatSeconds) * time.Second
	var (
		host              remote.Host
		trusted           remote.TrustedHost
		usedTrustedAccess bool
	)
	if password != "" {
		matchedHost, matched := s.store.ValidateHostAccessPassword(target, password, timeout)
		if !matched {
			if _, available := s.store.FindHostByTarget(target, timeout); available {
				writeError(w, http.StatusForbidden, "invalid host password")
				return
			}
			writeError(w, http.StatusNotFound, "target host is offline or unavailable")
			return
		}
		host = matchedHost
		trusted, _ = s.store.TrustHostForUser(user.ID, user.Email, host)
	} else {
		matchedHost, matchedTrusted, matched := s.store.FindTrustedHostByTarget(user.ID, target, timeout)
		if !matched {
			if _, available := s.store.FindHostByTarget(target, timeout); available {
				writeError(w, http.StatusForbidden, "host password required for the first trusted connection")
				return
			}
			writeError(w, http.StatusNotFound, "target host is offline or unavailable")
			return
		}
		host = matchedHost
		trusted = matchedTrusted
		usedTrustedAccess = true
	}

	createReq := remote.CreateSessionRequest{
		Target:         target,
		Viewer:         viewer,
		ViewerLabel:    strings.TrimSpace(req.ViewerLabel),
		ViewerAuthMode: "account",
	}
	session := s.store.CreateRouted(createReq, &host)
	s.store.TouchTrustedHostUsage(user.ID, host.ID)
	if refreshedTrusted, exists := s.store.TrustedHostForUser(user.ID, host.ID); exists {
		trusted = refreshedTrusted
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":                  session.ID,
		"session":             session,
		"trusted_host":        trusted,
		"used_trusted_access": usedTrustedAccess,
	})
}

func (s *Server) handleHostsWS(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.provisioningUserFromRequest(r); !ok {
		writeError(w, http.StatusUnauthorized, "provisioning credentials required")
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("host ws upgrade failed", "err", err)
		return
	}
	defer conn.Close()

	var hello remote.HostWireMessage
	if err := conn.ReadJSON(&hello); err != nil {
		_ = conn.WriteJSON(remote.HostWireMessage{Type: "error", Error: "invalid hello message"})
		return
	}
	if hello.Type != "register" || hello.Host == nil {
		_ = conn.WriteJSON(remote.HostWireMessage{Type: "error", Error: "register message required"})
		return
	}
	req := hello.Host
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.OS) == "" || strings.TrimSpace(req.Arch) == "" {
		_ = conn.WriteJSON(remote.HostWireMessage{Type: "error", Error: "name, os and arch are required"})
		return
	}

	host := s.store.RegisterHost(*req)
	liveConn := &hostConn{conn: conn}
	s.setHostConn(host.ID, liveConn)
	defer s.clearHostConn(host.ID, liveConn)
	s.logger.Info("host registered over ws", "host_id", host.ID, "public_id", host.PublicID, "hostname", host.Hostname, "version", host.Version, "os", host.OS, "arch", host.Arch)
	if err := liveConn.writeJSON(remote.HostWireMessage{
		Type:     "registered",
		HostID:   host.ID,
		PublicID: host.PublicID,
	}); err != nil {
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(45 * time.Second))
	conn.SetPongHandler(func(string) error {
		_, err := s.store.TouchHostState(host.ID, "", nil, "")
		if err == nil {
			_ = conn.SetReadDeadline(time.Now().Add(45 * time.Second))
		}
		return nil
	})

	for {
		var msg remote.HostWireMessage
		if err := conn.ReadJSON(&msg); err != nil {
			s.logger.Info("host ws disconnected", "host_id", host.ID, "err", err)
			return
		}
		switch msg.Type {
		case "heartbeat":
			var heartbeat remote.HostHeartbeatState
			if len(msg.Payload) > 0 {
				if err := json.Unmarshal(msg.Payload, &heartbeat); err != nil {
					_ = liveConn.writeJSON(remote.HostWireMessage{Type: "error", Error: "invalid heartbeat payload"})
					return
				}
			}
			host, err = s.store.TouchHostState(host.ID, heartbeat.Role, heartbeat.AccessEnabled, heartbeat.AccessPassword)
			if err != nil {
				_ = liveConn.writeJSON(remote.HostWireMessage{Type: "error", Error: "unknown host"})
				return
			}
			queuedSessions := s.store.ListQueuedSessionsForHost(host.ID)
			dispatches := make([]remote.HostSessionDispatch, 0, len(queuedSessions))
			dispatchedIDs := make([]string, 0, len(queuedSessions))
			for _, session := range queuedSessions {
				dispatches = append(dispatches, sessionDispatchFromSession(session))
				dispatchedIDs = append(dispatchedIDs, session.ID)
			}
			_ = conn.SetReadDeadline(time.Now().Add(45 * time.Second))
			if err := liveConn.writeJSON(remote.HostWireMessage{
				Type:     "heartbeat_ack",
				HostID:   host.ID,
				PublicID: host.PublicID,
				Sessions: dispatches,
			}); err != nil {
				return
			}
			s.store.MarkSessionsDispatched(dispatchedIDs)
		case "session_ack":
			if strings.TrimSpace(msg.SessionID) == "" {
				_ = liveConn.writeJSON(remote.HostWireMessage{Type: "error", Error: "session_id is required"})
				return
			}
			session, err := s.store.AcknowledgeSessionForHost(host.ID, msg.SessionID, msg.Action)
			if err != nil {
				_ = liveConn.writeJSON(remote.HostWireMessage{Type: "error", Error: "unknown session"})
				return
			}
			if err := liveConn.writeJSON(remote.HostWireMessage{
				Type:      "session_acknowledged",
				SessionID: session.ID,
				Action:    session.DispatchState,
				HostID:    host.ID,
				PublicID:  host.PublicID,
			}); err != nil {
				return
			}
		case "session_event":
			if strings.TrimSpace(msg.SessionID) == "" || len(msg.Payload) == 0 {
				_ = liveConn.writeJSON(remote.HostWireMessage{Type: "error", Error: "session_id and payload are required"})
				return
			}
			s.logHostSessionEvent(host, msg.SessionID, msg.Payload)
			if err := s.broadcastSessionEvent(msg.SessionID, msg.Payload); err != nil {
				_ = liveConn.writeJSON(remote.HostWireMessage{Type: "error", Error: err.Error()})
				return
			}
		default:
			if err := liveConn.writeJSON(remote.HostWireMessage{Type: "error", Error: "unsupported message type"}); err != nil {
				return
			}
		}
	}
}

func (s *Server) logHostSessionEvent(host remote.Host, sessionID string, payload json.RawMessage) {
	var event map[string]any
	if err := json.Unmarshal(payload, &event); err != nil {
		return
	}
	eventType := strings.TrimSpace(fmt.Sprint(event["type"]))
	if eventType != "screen_status" && eventType != "screen_error" {
		return
	}
	s.logger.Info(
		"host session diagnostic",
		"host_id", host.ID,
		"hostname", host.Hostname,
		"session_id", sessionID,
		"type", eventType,
		"transport", strings.TrimSpace(fmt.Sprint(event["transport"])),
		"action", strings.TrimSpace(fmt.Sprint(event["action"])),
		"trigger", strings.TrimSpace(fmt.Sprint(event["trigger"])),
		"error", strings.TrimSpace(fmt.Sprint(event["error"])),
	)
}

func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/")
	if strings.Trim(path, "/") == "" {
		s.handleSessions(w, r)
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	id := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		session, err := s.store.Get(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeJSON(w, http.StatusOK, session)
		return
	}

	switch parts[1] {
	case "reprompt":
		s.handleSessionReprompt(w, r, id)
	case "offer":
		s.handleOffer(w, r, id)
	case "answer":
		s.handleAnswer(w, r, id)
	case "candidates":
		s.handleCandidate(w, r, id)
	case "ws":
		s.handleSessionWS(w, r, id)
	case "screen":
		if len(parts) > 2 && parts[2] == "ws" {
			s.handleScreenWS(w, r, id)
			return
		}
		s.handleScreenFrame(w, r, id)
	case "close":
		s.handleClose(w, r, id)
	default:
		writeError(w, http.StatusNotFound, "session action not found")
	}
}

func (s *Server) handleOffer(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req remote.SDPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.SDP) == "" {
		writeError(w, http.StatusBadRequest, "sdp is required")
		return
	}
	session, err := s.store.SetOffer(id, req.SDP)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	s.logger.Info(
		"session offer received",
		"session_id", session.ID,
		"target", session.Target,
		"viewer", session.Viewer,
		"status", session.Status,
		"dispatch_state", session.DispatchState,
		"routed_host_id", session.RoutedHostID,
		"routed_hostname", session.RoutedHostname,
	)
	if strings.TrimSpace(session.RoutedHostID) != "" {
		s.dispatchSessionToConnectedHost(session, "offer")
	} else {
		s.logger.Warn("session offer has no routed host", "session_id", session.ID, "target", session.Target)
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleAnswer(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req remote.SDPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.SDP) == "" {
		writeError(w, http.StatusBadRequest, "sdp is required")
		return
	}
	session, err := s.store.SetAnswer(id, req.SDP)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleCandidate(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req remote.CandidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Candidate) == "" {
		writeError(w, http.StatusBadRequest, "candidate is required")
		return
	}
	session, err := s.store.AddCandidate(id, req.Candidate, req.Source)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleScreenFrame(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := s.currentUserFromRequest(r); !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		data, contentType, ok := s.store.GetScreenFrame(id)
		if !ok || len(data) == 0 {
			http.NotFound(w, r)
			return
		}
		if strings.TrimSpace(contentType) == "" {
			contentType = "image/jpeg"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		_, _ = w.Write(data)
	case http.MethodPost:
		contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
		if strings.HasPrefix(contentType, "image/") {
			data, err := io.ReadAll(r.Body)
			if err != nil || len(data) == 0 {
				writeError(w, http.StatusBadRequest, "screen frame body is required")
				return
			}
			width := intHeaderValue(r, "X-UnyDesk-Screen-Width")
			height := intHeaderValue(r, "X-UnyDesk-Screen-Height")
			session, err := s.store.SetScreenFrameBinary(id, data, contentType, "", width, height, "")
			if err != nil {
				writeError(w, http.StatusNotFound, "session not found")
				return
			}
			writeJSON(w, http.StatusOK, session)
			return
		}

		var req remote.ScreenFrameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if strings.TrimSpace(req.DataURL) == "" && strings.TrimSpace(req.Error) == "" {
			writeError(w, http.StatusBadRequest, "screen frame payload is required")
			return
		}
		session, err := s.store.SetScreenFrame(id, req.DataURL, req.Width, req.Height, req.Error)
		if err != nil {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeJSON(w, http.StatusOK, session)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleSessionReprompt(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorizeSessionViewer(r, id) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	session, err := s.store.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if strings.TrimSpace(session.RoutedHostID) == "" {
		writeError(w, http.StatusConflict, "session has no routed host")
		return
	}
	if session.Status == remote.StatusClosed || strings.EqualFold(strings.TrimSpace(session.DispatchState), "rejected") {
		writeError(w, http.StatusConflict, "session is no longer waiting for approval")
		return
	}
	if err := s.repromptSessionOnHost(session); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"session": session,
	})
}

func (s *Server) handleScreenWS(w http.ResponseWriter, r *http.Request, id string) {
	role := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("role")))
	switch role {
	case "host":
		s.handleHostScreenWS(w, r, id)
	case "viewer":
		s.handleViewerScreenWS(w, r, id)
	default:
		writeError(w, http.StatusBadRequest, "screen websocket role is required")
	}
}

func (s *Server) handleHostScreenWS(w http.ResponseWriter, r *http.Request, id string) {
	session, err := s.store.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	hostID := strings.TrimSpace(r.URL.Query().Get("host_id"))
	if hostID == "" || hostID != strings.TrimSpace(session.RoutedHostID) {
		writeError(w, http.StatusForbidden, "host is not allowed to stream this session")
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin:       func(_ *http.Request) bool { return true },
		EnableCompression: false,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("host screen ws upgrade failed", "session_id", id, "err", err)
		return
	}
	defer conn.Close()

	screenConn := &hostConn{conn: conn}
	s.setScreenHost(id, screenConn)
	defer s.clearScreenHost(id, screenConn)

	for {
		messageType, payload, readErr := conn.ReadMessage()
		if readErr != nil {
			return
		}
		switch messageType {
		case websocket.BinaryMessage, websocket.TextMessage:
			s.broadcastScreenFrame(id, messageType, payload)
		case websocket.PingMessage:
			if err := screenConn.writeMessage(websocket.PongMessage, payload); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleViewerScreenWS(w http.ResponseWriter, r *http.Request, id string) {
	if !s.authorizeSessionViewer(r, id) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if _, err := s.store.Get(id); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin:       func(_ *http.Request) bool { return true },
		EnableCompression: false,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("viewer screen ws upgrade failed", "session_id", id, "err", err)
		return
	}
	defer conn.Close()

	screenConn := &hostConn{conn: conn}
	s.addScreenViewer(id, screenConn)
	defer s.removeScreenViewer(id, screenConn)
	_ = screenConn.writeJSON(map[string]any{"type": "screen_stream", "state": "connected"})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, readErr := conn.ReadMessage(); readErr != nil {
				return
			}
		}
	}()

	pingTicker := time.NewTicker(20 * time.Second)
	defer pingTicker.Stop()
	for {
		select {
		case <-done:
			return
		case <-pingTicker.C:
			if err := screenConn.writeMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) setScreenHost(sessionID string, conn *hostConn) {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	relay := s.ensureScreenRelayLocked(sessionID)
	if relay.host != nil && relay.host != conn {
		relay.host.close()
	}
	relay.host = conn
}

func (s *Server) clearScreenHost(sessionID string, conn *hostConn) {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	relay, ok := s.screenSubs[sessionID]
	if !ok {
		return
	}
	if relay.host == conn {
		relay.host = nil
	}
	if relay.host == nil && len(relay.viewers) == 0 {
		delete(s.screenSubs, sessionID)
	}
}

func (s *Server) addScreenViewer(sessionID string, conn *hostConn) {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	relay := s.ensureScreenRelayLocked(sessionID)
	relay.viewers[conn] = struct{}{}
}

func (s *Server) removeScreenViewer(sessionID string, conn *hostConn) {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	relay, ok := s.screenSubs[sessionID]
	if !ok {
		return
	}
	delete(relay.viewers, conn)
	if relay.host == nil && len(relay.viewers) == 0 {
		delete(s.screenSubs, sessionID)
	}
}

func (s *Server) broadcastScreenFrame(sessionID string, messageType int, payload []byte) {
	s.screenMu.RLock()
	relay := s.screenSubs[sessionID]
	if relay == nil || len(relay.viewers) == 0 {
		s.screenMu.RUnlock()
		return
	}
	targets := make([]*hostConn, 0, len(relay.viewers))
	for viewer := range relay.viewers {
		targets = append(targets, viewer)
	}
	s.screenMu.RUnlock()

	for _, viewer := range targets {
		if err := viewer.writeMessage(messageType, payload); err != nil {
			viewer.close()
			s.removeScreenViewer(sessionID, viewer)
		}
	}
}

func (s *Server) ensureScreenRelayLocked(sessionID string) *screenRelay {
	relay, ok := s.screenSubs[sessionID]
	if !ok {
		relay = &screenRelay{viewers: make(map[*hostConn]struct{})}
		s.screenSubs[sessionID] = relay
	}
	if relay.viewers == nil {
		relay.viewers = make(map[*hostConn]struct{})
	}
	return relay
}

func intHeaderValue(r *http.Request, name string) int {
	raw := strings.TrimSpace(r.Header.Get(name))
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func (s *Server) handleClose(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorizeSessionViewer(r, id) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	session, err := s.store.Close(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleSessionWS(w http.ResponseWriter, r *http.Request, id string) {
	if !s.authorizeSessionViewer(r, id) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	session, err := s.store.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin:       func(_ *http.Request) bool { return true },
		EnableCompression: false,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("session ws upgrade failed", "session_id", id, "err", err)
		return
	}
	defer conn.Close()
	viewerConn := &hostConn{conn: conn}

	updates, cancel := s.store.SubscribeSession(id)
	defer cancel()
	s.addSessionEventViewer(id, viewerConn)
	defer s.removeSessionEventViewer(id, viewerConn)

	if err := viewerConn.writeJSON(map[string]any{
		"type":    "session",
		"session": session,
	}); err != nil {
		return
	}

	type viewerControlMessage struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}

	done := make(chan struct{})
	controls := make(chan json.RawMessage, 8)
	go func() {
		defer close(done)
		for {
			var msg viewerControlMessage
			if readErr := conn.ReadJSON(&msg); readErr != nil {
				return
			}
			if msg.Type != "control" || len(msg.Payload) == 0 {
				continue
			}
			select {
			case controls <- msg.Payload:
			default:
			}
		}
	}()

	pingTicker := time.NewTicker(20 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-done:
			return
		case <-pingTicker.C:
			if err := viewerConn.writeMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case payload := <-controls:
			if err := s.forwardControlToHost(id, payload); err != nil {
				_ = viewerConn.writeJSON(map[string]any{
					"type":  "error",
					"error": err.Error(),
				})
			}
		case session := <-updates:
			if err := viewerConn.writeJSON(map[string]any{
				"type":    "session",
				"session": session,
			}); err != nil {
				return
			}
		}
	}
}

func (s *Server) setHostConn(hostID string, conn *hostConn) {
	s.hostMu.Lock()
	defer s.hostMu.Unlock()
	s.hostConns[hostID] = conn
}

func (s *Server) clearHostConn(hostID string, conn *hostConn) {
	s.hostMu.Lock()
	defer s.hostMu.Unlock()
	current, ok := s.hostConns[hostID]
	if !ok || current != conn {
		return
	}
	delete(s.hostConns, hostID)
}

func sessionDispatchFromSession(session remote.Session) remote.HostSessionDispatch {
	return remote.HostSessionDispatch{
		ID:                 session.ID,
		Target:             session.Target,
		Viewer:             session.Viewer,
		ViewerLabel:        session.ViewerLabel,
		Status:             string(session.Status),
		CreatedAt:          session.CreatedAt,
		UpdatedAt:          session.UpdatedAt,
		OfferDigest:        sessionOfferDigest(session),
		RoutedHostID:       session.RoutedHostID,
		RoutedHostPublicID: session.RoutedHostPublicID,
		RoutedHostname:     session.RoutedHostname,
	}
}

func sessionOfferDigest(session remote.Session) string {
	offer := strings.TrimSpace(session.OfferSDP)
	if offer == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(offer))
	return fmt.Sprintf("%x", sum[:8])
}

func (s *Server) dispatchSessionToConnectedHost(session remote.Session, reason string) {
	hostID := strings.TrimSpace(session.RoutedHostID)
	if hostID == "" {
		return
	}

	s.hostMu.RLock()
	hostConn := s.hostConns[hostID]
	s.hostMu.RUnlock()
	if hostConn == nil {
		s.logger.Warn(
			"session dispatch waiting for host connection",
			"session_id", session.ID,
			"host_id", hostID,
			"reason", reason,
			"dispatch_state", session.DispatchState,
		)
		return
	}

	if err := hostConn.writeJSON(remote.HostWireMessage{
		Type:     "heartbeat_ack",
		HostID:   hostID,
		PublicID: session.RoutedHostPublicID,
		Sessions: []remote.HostSessionDispatch{sessionDispatchFromSession(session)},
	}); err != nil {
		s.logger.Warn("session dispatch to host failed", "session_id", session.ID, "host_id", hostID, "reason", reason, "err", err)
		hostConn.close()
		return
	}

	s.store.MarkSessionsDispatched([]string{session.ID})
	s.logger.Info(
		"session dispatched to host",
		"session_id", session.ID,
		"host_id", hostID,
		"reason", reason,
		"dispatch_state", session.DispatchState,
	)
}

func (s *Server) repromptSessionOnHost(session remote.Session) error {
	hostID := strings.TrimSpace(session.RoutedHostID)
	if hostID == "" {
		return fmt.Errorf("session has no routed host")
	}

	s.hostMu.RLock()
	hostConn := s.hostConns[hostID]
	s.hostMu.RUnlock()
	if hostConn == nil {
		return fmt.Errorf("target host is offline or unavailable")
	}

	if err := hostConn.writeJSON(remote.HostWireMessage{
		Type:     "approval_prompt",
		HostID:   hostID,
		PublicID: session.RoutedHostPublicID,
		Sessions: []remote.HostSessionDispatch{sessionDispatchFromSession(session)},
	}); err != nil {
		hostConn.close()
		return fmt.Errorf("unable to notify the host right now")
	}

	s.store.MarkSessionsDispatched([]string{session.ID})
	s.logger.Info(
		"session approval prompt re-sent to host",
		"session_id", session.ID,
		"host_id", hostID,
	)
	return nil
}

func (s *Server) addSessionEventViewer(sessionID string, conn *hostConn) {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	targets := s.eventSubs[sessionID]
	if targets == nil {
		targets = make(map[*hostConn]struct{})
		s.eventSubs[sessionID] = targets
	}
	targets[conn] = struct{}{}
}

func (s *Server) removeSessionEventViewer(sessionID string, conn *hostConn) {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	targets := s.eventSubs[sessionID]
	if targets == nil {
		return
	}
	delete(targets, conn)
	if len(targets) == 0 {
		delete(s.eventSubs, sessionID)
	}
}

func (s *Server) broadcastSessionEvent(sessionID string, payload json.RawMessage) error {
	if _, err := s.store.Get(sessionID); err != nil {
		return fmt.Errorf("unknown session")
	}

	s.eventMu.RLock()
	targetSet := s.eventSubs[sessionID]
	if len(targetSet) == 0 {
		s.eventMu.RUnlock()
		return nil
	}
	targets := make([]*hostConn, 0, len(targetSet))
	for viewer := range targetSet {
		targets = append(targets, viewer)
	}
	s.eventMu.RUnlock()

	for _, viewer := range targets {
		if err := viewer.writeJSON(map[string]any{
			"type":  "event",
			"event": json.RawMessage(payload),
		}); err != nil {
			viewer.close()
			s.removeSessionEventViewer(sessionID, viewer)
		}
	}
	return nil
}

func (s *Server) forwardControlToHost(sessionID string, payload json.RawMessage) error {
	session, err := s.store.Get(sessionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(session.RoutedHostID) == "" {
		return fmt.Errorf("session has no routed host")
	}

	s.hostMu.RLock()
	hostConn := s.hostConns[session.RoutedHostID]
	s.hostMu.RUnlock()
	if hostConn == nil {
		return fmt.Errorf("host is not connected")
	}

	return hostConn.writeJSON(remote.HostWireMessage{
		Type:      "control",
		SessionID: sessionID,
		Payload:   payload,
	})
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldLogRequest(r) {
			s.logger.Info("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		}
		next.ServeHTTP(w, r)
	})
}

func shouldLogRequest(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return true
	}
	path := strings.TrimSpace(r.URL.Path)
	if isNoisySessionRead(path) || isStaticFrontendRead(path) {
		return false
	}
	return true
}

func isNoisySessionRead(path string) bool {
	trimmed := strings.Trim(path, "/")
	if !strings.HasPrefix(trimmed, "api/v1/sessions/") {
		return false
	}
	rest := strings.TrimPrefix(trimmed, "api/v1/sessions/")
	if rest == "" {
		return false
	}
	if !strings.Contains(rest, "/") {
		return true
	}
	return strings.HasSuffix(rest, "/ws")
}

func isStaticFrontendRead(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".css", ".js", ".ico", ".jpg", ".jpeg", ".png", ".svg", ".webp", ".woff", ".woff2":
		return true
	default:
		return false
	}
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.cfg.Security.AllowOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token, X-UnyDesk-Standalone-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Expose-Headers", csrfHeaderName)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", strings.Join([]string{
			"default-src 'self'",
			"base-uri 'self'",
			"object-src 'none'",
			"frame-ancestors 'none'",
			"img-src 'self' data: blob:",
			"font-src 'self' data:",
			"style-src 'self'",
			"script-src 'self'",
			"connect-src 'self' ws: wss: http://127.0.0.1:39091 http://localhost:39091",
			"form-action 'self'",
		}, "; "))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withCSRFCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := readCSRFCookie(r)
		if token == "" {
			token = newCSRFToken()
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookieName,
				Value:    token,
				Path:     "/",
				HttpOnly: false,
				SameSite: http.SameSiteLaxMode,
				Secure:   requestIsHTTPS(r),
			})
		}
		w.Header().Set(csrfHeaderName, token)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withCSRFProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		if !isBrowserLikeRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		cookieToken := readCSRFCookie(r)
		headerToken := strings.TrimSpace(r.Header.Get(csrfHeaderName))
		if cookieToken == "" || headerToken == "" || cookieToken != headerToken {
			writeError(w, http.StatusForbidden, "csrf validation failed")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) registerFrontendRoutes(mux *http.ServeMux) {
	frontendDir := strings.TrimSpace(s.cfg.Paths.FrontendDir)
	if frontendDir != "" {
		if info, err := os.Stat(frontendDir); err == nil && info.IsDir() {
			s.logger.Info("frontend served from disk", "dir", frontendDir)
			s.registerDiskFrontend(mux, frontendDir)
			return
		}
		s.logger.Warn("frontend dir unavailable, falling back to embedded assets", "dir", frontendDir)
	}

	assetFS, err := fs.Sub(embeddedAssets, "assets")
	if err == nil {
		mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetFS))))
		mux.Handle("/favicon.ico", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFileFS(w, r, assetFS, "favicon.ico")
		}))
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/download/") || r.URL.Path == "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("UnyDesk frontend not available. Set UNYDESK_ASSETS or restore frontend/public."))
	})
}

func (s *Server) registerDiskFrontend(mux *http.ServeMux, frontendDir string) {
	mux.Handle("/account", s.requireAuthPage(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveFrontendHTML(w, r, frontendDir, filepath.Join(frontendDir, "account", "index.html"))
	})))
	mux.Handle("/account/", s.requireAuthPage(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		target := filepath.Join(frontendDir, filepath.Clean(path))
		if info, err := os.Stat(target); err == nil {
			if info.IsDir() {
				indexPath := filepath.Join(target, "index.html")
				if indexInfo, indexErr := os.Stat(indexPath); indexErr == nil && !indexInfo.IsDir() {
					serveFrontendHTML(w, r, frontendDir, indexPath)
					return
				}
			}
			if !info.IsDir() {
				if strings.EqualFold(filepath.Ext(target), ".html") {
					serveFrontendHTML(w, r, frontendDir, target)
					return
				}
				serveFrontendFile(w, r, target)
				return
			}
		}
		serveFrontendHTML(w, r, frontendDir, filepath.Join(frontendDir, "account", "index.html"))
	})))

	for _, dir := range []string{"assets", "vendor", "fonts", "webfonts", "static", "media", "css", "app"} {
		prefix := "/" + dir + "/"
		dirPath := filepath.Join(frontendDir, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			mux.Handle(prefix, http.StripPrefix(prefix, cachedFrontendFileServer(dirPath)))
		}
	}

	for _, name := range []string{"app.js", "styles.css", "favicon.ico", "robots.txt", "manifest.json"} {
		filePath := filepath.Join(frontendDir, name)
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			path := "/" + name
			mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				serveFrontendFile(w, r, filePath)
			}))
		}
	}

	mux.Handle("/", spaFallbackDir(frontendDir))
}

func cachedFrontendFileServer(root string) http.Handler {
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setFrontendCacheHeaders(w, r, r.URL.Path)
		files.ServeHTTP(w, r)
	})
}

func serveFrontendFile(w http.ResponseWriter, r *http.Request, path string) {
	setFrontendCacheHeaders(w, r, path)
	http.ServeFile(w, r, path)
}

func serveFrontendHTML(w http.ResponseWriter, r *http.Request, frontendDir, path string) {
	setFrontendCacheHeaders(w, r, path)

	content, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	content = injectFrontendAssetVersions(content, frontendDir)
	_, _ = w.Write(content)
}

func injectFrontendAssetVersions(content []byte, frontendDir string) []byte {
	return frontendAssetRefPattern.ReplaceAllFunc(content, func(match []byte) []byte {
		parts := frontendAssetRefPattern.FindSubmatch(match)
		if len(parts) != 4 {
			return match
		}

		assetPath := string(parts[2])
		version := frontendAssetVersion(frontendDir, assetPath)
		if version == "" {
			return match
		}

		return []byte(string(parts[1]) + assetPath + "?v=" + version + string(parts[3]))
	})
}

func frontendAssetVersion(frontendDir, assetPath string) string {
	relPath := filepath.Clean(strings.TrimPrefix(assetPath, "/"))
	fullPath := filepath.Join(frontendDir, filepath.FromSlash(relPath))
	relative, err := filepath.Rel(frontendDir, fullPath)
	if err != nil || strings.HasPrefix(relative, "..") {
		return ""
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		return ""
	}

	return fmt.Sprintf("%x-%x", info.ModTime().UTC().Unix(), info.Size())
}

func setFrontendCacheHeaders(w http.ResponseWriter, r *http.Request, path string) {
	ext := strings.ToLower(filepath.Ext(path))
	if r.URL.RawQuery != "" && ext != "" && ext != ".html" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}

	switch ext {
	case ".html", "":
		w.Header().Set("Cache-Control", "no-cache")
	case ".js", ".css":
		w.Header().Set("Cache-Control", "public, max-age=300")
	case ".woff", ".woff2", ".ttf", ".otf":
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	default:
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}
}

func spaFallbackDir(frontendDir string) http.Handler {
	deny := []string{"/api/", "/download/", "/healthz", "/assets/", "/vendor/", "/fonts/", "/webfonts/", "/static/", "/media/", "/css/", "/app/"}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		for _, p := range deny {
			if strings.HasPrefix(r.URL.Path, p) {
				http.NotFound(w, r)
				return
			}
		}

		candidate := filepath.Join(frontendDir, filepath.Clean(strings.TrimPrefix(r.URL.Path, "/")))
		if r.URL.Path != "/" {
			if info, err := os.Stat(candidate); err == nil {
				if info.IsDir() {
					indexPath := filepath.Join(candidate, "index.html")
					if indexInfo, indexErr := os.Stat(indexPath); indexErr == nil && !indexInfo.IsDir() {
						serveFrontendHTML(w, r, frontendDir, indexPath)
						return
					}
				}
				if !info.IsDir() {
					if strings.EqualFold(filepath.Ext(candidate), ".html") {
						serveFrontendHTML(w, r, frontendDir, candidate)
						return
					}
					serveFrontendFile(w, r, candidate)
					return
				}
			}
		}

		serveFrontendHTML(w, r, frontendDir, filepath.Join(frontendDir, "index.html"))
	})
}

func (s *Server) requireAuthPage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := readCookieValue(r, authCookieName)
		if token == "" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		if _, err := s.authStore.CurrentUser(token); err != nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) setAuthCookie(w http.ResponseWriter, r *http.Request, sessionToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
		MaxAge:   60 * 60 * 24 * 30,
	})
}

func (s *Server) currentUserFromRequest(r *http.Request) (auth.PublicUser, bool) {
	token := readCookieValue(r, authCookieName)
	if token == "" {
		return auth.PublicUser{}, false
	}
	user, err := s.authStore.CurrentUser(token)
	if err != nil {
		return auth.PublicUser{}, false
	}
	return user, true
}

func (s *Server) accountOrBasicUserFromRequest(r *http.Request) (auth.PublicUser, bool) {
	if user, ok := s.currentUserFromRequest(r); ok {
		return user, true
	}

	email, password, ok := r.BasicAuth()
	if !ok {
		return auth.PublicUser{}, false
	}
	user, err := s.authStore.ValidateCredentials(email, password)
	if err != nil {
		return auth.PublicUser{}, false
	}
	return user, true
}

func (s *Server) provisioningUserFromRequest(r *http.Request) (auth.PublicUser, bool) {
	if user, ok := s.accountOrBasicUserFromRequest(r); ok {
		return user, true
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return auth.PublicUser{}, false
	}
	token := strings.TrimSpace(authHeader[len("Bearer "):])
	if token == "" {
		return auth.PublicUser{}, false
	}
	user, _, err := s.authStore.ValidateProvisionToken(token)
	if err != nil {
		return auth.PublicUser{}, false
	}
	return user, true
}

func (s *Server) authorizeSessionViewer(r *http.Request, sessionID string) bool {
	if _, ok := s.currentUserFromRequest(r); ok {
		return true
	}
	token := readStandaloneViewerToken(r)
	if token == "" {
		return false
	}
	return s.store.ValidateStandaloneViewerToken(sessionID, token)
}

func readCSRFCookie(r *http.Request) string {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func readCookieValue(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func readStandaloneViewerToken(r *http.Request) string {
	if header := strings.TrimSpace(r.Header.Get(standaloneTokenHeader)); header != "" {
		return header
	}
	return strings.TrimSpace(r.URL.Query().Get("standalone_token"))
}

func newStandaloneViewerToken() (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func isValidInstallID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 16 || len(value) > 128 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

func newCSRFToken() string {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return generatePublicID()
	}
	return base64.RawURLEncoding.EncodeToString(raw[:])
}

func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if forwardedProto != "" {
		for _, candidate := range strings.Split(forwardedProto, ",") {
			if strings.EqualFold(strings.TrimSpace(candidate), "https") {
				return true
			}
		}
	}
	for _, entry := range strings.Split(r.Header.Get("Forwarded"), ",") {
		for _, part := range strings.Split(entry, ";") {
			key, rawValue, ok := strings.Cut(strings.TrimSpace(part), "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(key), "proto") {
				continue
			}
			if strings.EqualFold(strings.Trim(strings.TrimSpace(rawValue), `"`), "https") {
				return true
			}
		}
	}
	return false
}

func isBrowserLikeRequest(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("Origin")) != "" || strings.TrimSpace(r.Header.Get("Referer")) != ""
}

func loadOrCreatePublicID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}

	id := generatePublicID()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(id+"\n"), 0o640); err != nil {
		return "", err
	}
	return id, nil
}

func generatePublicID() string {
	var raw [9]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "000 000 000"
	}
	for i := range raw {
		raw[i] = '0' + (raw[i] % 10)
	}
	return string(raw[0:3]) + " " + string(raw[3:6]) + " " + string(raw[6:9])
}
