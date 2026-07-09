package server

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"unydesk/remote"
)

func (s *Server) handleBootstrapClaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	s.handleBootstrapCredentialIssue(w, r, user.Email)
}

func (s *Server) handleBootstrapProvision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.accountOrBasicUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	s.handleBootstrapCredentialIssue(w, r, user.Email)
}

func (s *Server) handleBootstrapCredentialIssue(w http.ResponseWriter, r *http.Request, email string) {
	var req struct {
		InstallID string `json:"install_id"`
		PublicID  string `json:"public_id"`
		Hostname  string `json:"hostname"`
		Name      string `json:"name"`
		Version   string `json:"version"`
		OS        string `json:"os"`
		Arch      string `json:"arch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	req.InstallID = strings.TrimSpace(req.InstallID)
	req.PublicID = strings.TrimSpace(req.PublicID)
	req.Hostname = strings.TrimSpace(req.Hostname)
	req.Name = strings.TrimSpace(req.Name)
	req.Version = strings.TrimSpace(req.Version)
	req.OS = strings.TrimSpace(req.OS)
	req.Arch = strings.TrimSpace(req.Arch)
	if req.InstallID == "" && req.PublicID == "" {
		writeError(w, http.StatusBadRequest, "install_id or public_id is required")
		return
	}

	resolvedPublicID := req.PublicID
	if resolvedPublicID == "" && req.InstallID != "" {
		resolvedPublicID = remote.StablePublicID("host", req.InstallID)
	}

	labelParts := make([]string, 0, 6)
	if req.Hostname != "" {
		labelParts = append(labelParts, req.Hostname)
	}
	if resolvedPublicID != "" {
		labelParts = append(labelParts, resolvedPublicID)
	}
	if req.Version != "" {
		labelParts = append(labelParts, req.Version)
	}
	if req.OS != "" {
		labelParts = append(labelParts, req.OS)
	}
	if req.Arch != "" {
		labelParts = append(labelParts, req.Arch)
	}
	if len(labelParts) == 0 {
		labelParts = append(labelParts, "Host claim")
	}

	token, plainToken, err := s.authStore.IssueProvisionToken(email, strings.Join(labelParts, " · "), req.InstallID, resolvedPublicID, req.Hostname)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unable to issue provisioning token")
		return
	}

	resolvedServerURL := s.bootstrapServerURL(r)
	resolvedDomain := bootstrapDomain(r)

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"domain":          resolvedDomain,
		"server_url":      resolvedServerURL,
		"install_id":      req.InstallID,
		"public_id":       resolvedPublicID,
		"credential":      plainToken,
		"credential_type": "bearer",
		"token_id":        token.ID,
	})
}

func (s *Server) bootstrapServerURL(r *http.Request) string {
	if value := strings.TrimSpace(s.cfg.ServerURL); value != "" {
		return strings.TrimRight(value, "/")
	}
	return requestBaseServerURL(r)
}

func requestBaseServerURL(r *http.Request) string {
	scheme := "http"
	if requestIsHTTPS(r) {
		scheme = "https"
	}
	return scheme + "://" + requestHost(r)
}

func bootstrapDomain(r *http.Request) string {
	host := requestHost(r)
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	}
	return strings.Trim(host, "[]")
}

func requestHost(r *http.Request) string {
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if idx := strings.Index(host, ","); idx >= 0 {
		host = strings.TrimSpace(host[:idx])
	}
	return host
}
