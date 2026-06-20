package server

import (
	"crypto/rand"
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"unydesk/auth"
	"unydesk/config"
	"unydesk/remote"
)

//go:embed assets/*
var embeddedAssets embed.FS

const (
	csrfCookieName = "unydesk_csrf"
	csrfHeaderName = "X-CSRF-Token"
	authCookieName = "unydesk_session"
)

type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
	cfg        config.Settings
	store      *remote.MemoryStore
	authStore  *auth.Store
	publicID   string
}

func New(cfg config.Settings, store *remote.MemoryStore, authStore *auth.Store, logger *slog.Logger) *Server {
	publicID, err := loadOrCreatePublicID(cfg.Paths.PublicIDFile)
	if err != nil {
		logger.Warn("public id bootstrap failed", "err", err)
		publicID = generatePublicID()
	}

	s := &Server{
		logger:   logger,
		cfg:      cfg,
		store:    store,
		authStore: authStore,
		publicID: publicID,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/download/host/", s.handleHostDownload)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/v1/info", s.handleInfo)
	mux.HandleFunc("/api/v1/browser/identity", s.handleBrowserIdentity)
	mux.HandleFunc("/api/v1/auth/register", s.handleAuthRegister)
	mux.HandleFunc("/api/v1/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/v1/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("/api/v1/auth/session", s.handleAuthSession)
	mux.HandleFunc("/api/v1/auth/password", s.handleAuthPassword)
	mux.HandleFunc("/api/v1/profile", s.handleProfile)
	mux.HandleFunc("/api/v1/hosts", s.handleHosts)
	mux.HandleFunc("/api/v1/hosts/ws", s.handleHostsWS)
	mux.HandleFunc("/api/v1/sessions", s.handleSessions)
	mux.HandleFunc("/api/v1/sessions/", s.handleSessionByID)
	s.registerFrontendRoutes(mux)

	s.httpServer = &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: s.withCORS(s.withSecurityHeaders(s.withCSRFCookie(s.withCSRFProtection(s.withLogging(mux))))),
	}

	return s
}

func (s *Server) ListenAndServe() error {
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"service":   s.cfg.Name,
		"version":   config.Version,
		"public_id": s.publicID,
	})
}

func (s *Server) handleInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":                     s.cfg.Name,
		"version":                  config.Version,
		"listen_addr":              s.cfg.ListenAddr,
		"allow_origin":             s.cfg.Security.AllowOrigin,
		"session_ttl":              s.cfg.Remote.SessionTTLMinutes,
		"public_id":                s.publicID,
		"host_heartbeat_seconds":   s.cfg.Remote.HostHeartbeatSeconds,
		"frontend_dir":             s.cfg.Paths.FrontendDir,
		"frontend_live_reload":     true,
		"frontend_reload_strategy": "serve-from-disk-and-refresh",
		"downloads": map[string]string{
			"linux_amd64_host":   "/download/host/linux-amd64",
			"linux_arm64_host":   "/download/host/linux-arm64",
			"windows_amd64_host": "/download/host/windows-amd64",
			"windows_arm64_host": "/download/host/windows-arm64",
			"macos_amd64_host":   "/download/host/macos-amd64",
			"macos_arm64_host":   "/download/host/macos-arm64",
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
	})
}

func (s *Server) handleBrowserIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		token = strings.TrimSpace(readCookieValue(r, "unydesk_browser_token"))
	}
	if token == "" {
		token = newCSRFToken()
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "unydesk_browser_token",
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
		MaxAge:   60 * 60 * 24 * 365 * 10,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"id":        remote.StableID("browser", token),
		"public_id": remote.StablePublicID("browser", token),
		"token":     token,
		"kind":      "browser",
	})
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

func (s *Server) handleHostDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	target := strings.TrimPrefix(r.URL.Path, "/download/host/")
	filename := ""
	switch target {
	case "linux-amd64":
		filename = "unydesk-host-linux-amd64"
	case "linux-arm64":
		filename = "unydesk-host-linux-arm64"
	case "windows-amd64":
		filename = "unydesk-host-windows-amd64.exe"
	case "windows-arm64":
		filename = "unydesk-host-windows-arm64.exe"
	case "macos-amd64":
		filename = "unydesk-host-darwin-amd64"
	case "macos-arm64":
		filename = "unydesk-host-darwin-arm64"
	case "checksums":
		filename = "SHA256SUMS"
	default:
		writeError(w, http.StatusNotFound, "host download not found")
		return
	}

	path := filepath.Join(s.cfg.Paths.HostDownloadsDir, filename)
	if _, err := os.Stat(path); err != nil {
		writeError(w, http.StatusNotFound, "host binary not available yet")
		return
	}

	if filename == "SHA256SUMS" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.ServeFile(w, r, path)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, path)
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
		if host, ok := s.store.FindHostByTarget(req.Target); ok {
			writeJSON(w, http.StatusCreated, s.store.CreateRouted(req, &host))
			return
		}
		writeJSON(w, http.StatusCreated, s.store.Create(req))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleHosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := s.currentUserFromRequest(r); !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		timeout := time.Duration(s.cfg.Remote.HostHeartbeatSeconds) * time.Second
		writeJSON(w, http.StatusOK, map[string]any{
			"hosts":                     s.store.ListHostsWithTimeout(timeout),
			"heartbeat_timeout_seconds": s.cfg.Remote.HostHeartbeatSeconds,
		})
	case http.MethodPost:
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

func (s *Server) handleHostsWS(w http.ResponseWriter, r *http.Request) {
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
	s.logger.Info("host registered over ws", "host_id", host.ID, "public_id", host.PublicID, "hostname", host.Hostname)
	if err := conn.WriteJSON(remote.HostWireMessage{
		Type:     "registered",
		HostID:   host.ID,
		PublicID: host.PublicID,
	}); err != nil {
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(45 * time.Second))
	conn.SetPongHandler(func(string) error {
		_, err := s.store.TouchHost(host.ID)
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
			host, err = s.store.TouchHost(host.ID)
			if err != nil {
				_ = conn.WriteJSON(remote.HostWireMessage{Type: "error", Error: "unknown host"})
				return
			}
			queuedSessions := s.store.ListQueuedSessionsForHost(host.ID)
			dispatches := make([]remote.HostSessionDispatch, 0, len(queuedSessions))
			dispatchedIDs := make([]string, 0, len(queuedSessions))
			for _, session := range queuedSessions {
				dispatches = append(dispatches, remote.HostSessionDispatch{
					ID:                 session.ID,
					Target:             session.Target,
					Viewer:             session.Viewer,
					Status:             string(session.Status),
					CreatedAt:          session.CreatedAt,
					RoutedHostID:       session.RoutedHostID,
					RoutedHostPublicID: session.RoutedHostPublicID,
					RoutedHostname:     session.RoutedHostname,
				})
				dispatchedIDs = append(dispatchedIDs, session.ID)
			}
			_ = conn.SetReadDeadline(time.Now().Add(45 * time.Second))
			if err := conn.WriteJSON(remote.HostWireMessage{
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
				_ = conn.WriteJSON(remote.HostWireMessage{Type: "error", Error: "session_id is required"})
				return
			}
			session, err := s.store.AcknowledgeSessionForHost(host.ID, msg.SessionID, msg.Action)
			if err != nil {
				_ = conn.WriteJSON(remote.HostWireMessage{Type: "error", Error: "unknown session"})
				return
			}
			if err := conn.WriteJSON(remote.HostWireMessage{
				Type:      "session_acknowledged",
				SessionID: session.ID,
				Action:    session.DispatchState,
				HostID:    host.ID,
				PublicID:  host.PublicID,
			}); err != nil {
				return
			}
		default:
			if err := conn.WriteJSON(remote.HostWireMessage{Type: "error", Error: "unsupported message type"}); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/")
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
	case "offer":
		s.handleOffer(w, r, id)
	case "answer":
		s.handleAnswer(w, r, id)
	case "candidates":
		s.handleCandidate(w, r, id)
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

func (s *Server) handleClose(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	session, err := s.store.Close(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.cfg.Security.AllowOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
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
			"img-src 'self' data:",
			"font-src 'self' data:",
			"style-src 'self'",
			"script-src 'self'",
			"connect-src 'self' ws: wss:",
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
		http.ServeFile(w, r, filepath.Join(frontendDir, "account", "index.html"))
	})))
	mux.Handle("/account/", s.requireAuthPage(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		target := filepath.Join(frontendDir, filepath.Clean(path))
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			http.ServeFile(w, r, target)
			return
		}
		http.ServeFile(w, r, filepath.Join(frontendDir, "account", "index.html"))
	})))

	for _, dir := range []string{"assets", "vendor", "fonts", "webfonts", "static", "media", "css", "app"} {
		prefix := "/" + dir + "/"
		dirPath := filepath.Join(frontendDir, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			mux.Handle(prefix, http.StripPrefix(prefix, http.FileServer(http.Dir(dirPath))))
		}
	}

	for _, name := range []string{"app.js", "styles.css", "favicon.ico", "robots.txt", "manifest.json"} {
		filePath := filepath.Join(frontendDir, name)
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			path := "/" + name
			mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, filePath)
			}))
		}
	}

	mux.Handle("/", spaFallbackDir(frontendDir))
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
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						http.ServeFile(w, r, indexPath)
						return
					}
				}
				if !info.IsDir() {
					http.ServeFile(w, r, candidate)
					return
				}
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, filepath.Join(frontendDir, "index.html"))
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
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
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
