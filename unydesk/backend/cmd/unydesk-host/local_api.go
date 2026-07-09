package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

var startLocalHostUIOnce sync.Once

const localHostUILoopbackAddr = "127.0.0.1:39091"

func startLocalHostUI(ctx context.Context, autoOpen bool) {
	startLocalHostUIOnce.Do(func() {
		listener, err := net.Listen("tcp", localHostUILoopbackAddr)
		if err != nil {
			return
		}

		baseURL := "http://" + listener.Addr().String()
		localHostUI.setLocalUIURL(baseURL)

		mux := http.NewServeMux()
		mux.HandleFunc("/", handleLocalHostUIPage)
		mux.HandleFunc("/api/discovery", handleLocalHostUIDiscovery)
		mux.HandleFunc("/api/bootstrap", handleLocalHostUIBootstrap)
		mux.HandleFunc("/api/status", handleLocalHostUIStatus)
		mux.HandleFunc("/api/access", handleLocalHostUIAccess)
		mux.HandleFunc("/api/password", handleLocalHostUIPassword)
		mux.HandleFunc("/api/provision", handleLocalHostUIProvision)

		server := &http.Server{Handler: mux}
		go func() {
			<-ctx.Done()
			_ = server.Shutdown(context.Background())
		}()
		go func() {
			_ = server.Serve(listener)
		}()

		if autoOpen {
			go func() {
				_ = openLocalHostUI(baseURL)
			}()
		}
	})
}

func handleLocalHostUIDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeLocalHostUIPublicCORS(w, r, "GET, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeLocalHostUIPublicCORS(w, r, "GET, OPTIONS")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(localHostPublicDiscoverySnapshot(localHostUI.snapshot()))
}

func handleLocalHostUIStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeLocalHostUICORS(w, r, "GET, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeLocalHostUICORS(w, r, "GET, OPTIONS")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(localHostUI.snapshot())
}

func handleLocalHostUIBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeLocalHostUIPublicCORS(w, r, "POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Domain         string `json:"domain"`
		ServerURL      string `json:"server_url"`
		InstallID      string `json:"install_id"`
		PublicID       string `json:"public_id"`
		Credential     string `json:"credential"`
		CredentialType string `json:"credential_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	serverURL := strings.TrimSpace(payload.ServerURL)
	if serverURL == "" {
		http.Error(w, "server_url is required", http.StatusBadRequest)
		return
	}
	if !looksLikeServerAddress(serverURL) {
		http.Error(w, "invalid server_url", http.StatusBadRequest)
		return
	}

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	hasCredential := strings.TrimSpace(payload.Credential) != ""
	if hasCredential {
		if origin != "" && !localHostUIPublicOrigin(origin) {
			http.Error(w, "origin is not allowed for bootstrap", http.StatusForbidden)
			return
		}
		writeLocalHostUIPublicCORS(w, r, "POST, OPTIONS")
	} else {
		if origin == "" || !localHostUIAllowedOrigin(origin) {
			http.Error(w, "origin is not allowed for bootstrap routing", http.StatusForbidden)
			return
		}
		writeLocalHostUICORS(w, r, "POST, OPTIONS")
	}

	if hasCredential {
		if err := updateBootstrapProvisioning(
			serverURL,
			strings.TrimSpace(payload.Domain),
			strings.TrimSpace(payload.InstallID),
			strings.TrimSpace(payload.PublicID),
			strings.TrimSpace(payload.Credential),
			firstNonEmpty(strings.TrimSpace(payload.CredentialType), "bearer"),
		); err != nil {
			http.Error(w, "unable to persist bootstrap configuration", http.StatusInternalServerError)
			return
		}
		localHostUI.applyBootstrapClaim(serverURL, strings.TrimSpace(payload.InstallID), strings.TrimSpace(payload.PublicID), true)
	} else {
		if err := updateBootstrapRouting(
			serverURL,
			strings.TrimSpace(payload.Domain),
			strings.TrimSpace(payload.InstallID),
			strings.TrimSpace(payload.PublicID),
		); err != nil {
			http.Error(w, "unable to persist bootstrap route", http.StatusInternalServerError)
			return
		}
		localHostUI.applyBootstrapClaim(serverURL, strings.TrimSpace(payload.InstallID), strings.TrimSpace(payload.PublicID), false)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(localHostUI.snapshot())
}

func handleLocalHostUIAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeLocalHostUICORS(w, r, "POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeLocalHostUICORS(w, r, "POST, OPTIONS")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	setHostAccessEnabled(payload.Enabled)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(localHostUI.snapshot())
}

func handleLocalHostUIPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeLocalHostUICORS(w, r, "POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeLocalHostUICORS(w, r, "POST, OPTIONS")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if _, err := rotatePersistentHostAccessPassword(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(localHostUI.snapshot())
}

func handleLocalHostUIProvision(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeLocalHostUICORS(w, r, "POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeLocalHostUICORS(w, r, "POST, OPTIONS")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(payload.Email)
	password := strings.TrimSpace(payload.Password)
	status := localHostUI.snapshot()
	if email == "" || password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(status.ServerURL) == "" {
		http.Error(w, "bootstrap server URL is missing", http.StatusBadRequest)
		return
	}

	requestBody := map[string]string{
		"install_id": status.InstallID,
		"public_id":  status.PublicID,
		"hostname":   status.Hostname,
		"name":       status.Name,
		"version":    status.Version,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		http.Error(w, "unable to build provisioning request", http.StatusInternalServerError)
		return
	}

	endpoint := strings.TrimRight(status.ServerURL, "/") + "/api/v1/bootstrap/provision"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "unable to prepare provisioning request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(email, password)

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		http.Error(w, "unable to reach bootstrap server", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	var provisioning struct {
		OK             bool   `json:"ok"`
		Domain         string `json:"domain"`
		ServerURL      string `json:"server_url"`
		Credential     string `json:"credential"`
		CredentialType string `json:"credential_type"`
		PublicID       string `json:"public_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&provisioning); err != nil {
		http.Error(w, "invalid provisioning response", http.StatusBadGateway)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		http.Error(w, "provisioning rejected by server", http.StatusUnauthorized)
		return
	}
	if strings.TrimSpace(provisioning.Credential) == "" {
		http.Error(w, "provisioning response returned no credential", http.StatusBadGateway)
		return
	}

	if err := updateBootstrapProvisioning(
		firstNonEmpty(strings.TrimSpace(provisioning.ServerURL), strings.TrimSpace(status.ServerURL)),
		strings.TrimSpace(provisioning.Domain),
		strings.TrimSpace(status.InstallID),
		firstNonEmpty(strings.TrimSpace(provisioning.PublicID), strings.TrimSpace(status.PublicID)),
		strings.TrimSpace(provisioning.Credential),
		firstNonEmpty(strings.TrimSpace(provisioning.CredentialType), "bearer"),
	); err != nil {
		http.Error(w, "unable to store provisioning token locally", http.StatusInternalServerError)
		return
	}

	localHostUI.applyBootstrapClaim(
		firstNonEmpty(strings.TrimSpace(provisioning.ServerURL), strings.TrimSpace(status.ServerURL)),
		strings.TrimSpace(status.InstallID),
		firstNonEmpty(strings.TrimSpace(provisioning.PublicID), strings.TrimSpace(status.PublicID)),
		true,
	)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(localHostUI.snapshot())
}

func localHostUIAllowedOrigin(origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}

	allowed := make(map[string]struct{}, 4)
	for _, candidate := range []string{
		strings.TrimSpace(localHostUI.snapshot().ServerURL),
		strings.TrimSpace(defaultServerURL),
	} {
		if candidate == "" {
			continue
		}
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}
		baseOrigin := parsed.Scheme + "://" + parsed.Host
		allowed[strings.ToLower(baseOrigin)] = struct{}{}
		hostName := parsed.Hostname()
		switch {
		case strings.HasPrefix(strings.ToLower(hostName), "www."):
			altHost := strings.TrimPrefix(hostName, "www.")
			allowed[strings.ToLower(parsed.Scheme+"://"+altHost)] = struct{}{}
		case hostName != "":
			allowed[strings.ToLower(parsed.Scheme+"://www."+hostName)] = struct{}{}
		}
	}
	_, ok := allowed[strings.ToLower(origin)]
	return ok
}

func localHostUIPublicOrigin(origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return strings.TrimSpace(parsed.Host) != ""
}

func localHostPublicDiscoverySnapshot(snapshot hostUIStatus) map[string]any {
	return map[string]any{
		"name":             strings.TrimSpace(snapshot.Name),
		"version":          strings.TrimSpace(snapshot.Version),
		"hostname":         strings.TrimSpace(snapshot.Hostname),
		"server_url":       strings.TrimSpace(snapshot.ServerURL),
		"install_id":       strings.TrimSpace(snapshot.InstallID),
		"public_id":        strings.TrimSpace(snapshot.PublicID),
		"access_password":  "",
		"local_ui_url":     strings.TrimSpace(snapshot.LocalUIURL),
		"connected":        snapshot.Connected,
		"provisioned":      snapshot.Provisioned,
		"connection_state": strings.TrimSpace(snapshot.ConnectionState),
		"connection_note":  strings.TrimSpace(snapshot.ConnectionNote),
	}
}

func writeLocalHostUICORS(w http.ResponseWriter, r *http.Request, methods string) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}
	snapshot := localHostUI.snapshot()
	serverURLKnown := strings.TrimSpace(snapshot.ServerURL) != "" || strings.TrimSpace(defaultServerURL) != ""
	if serverURLKnown && !localHostUIAllowedOrigin(origin) {
		return false
	}
	if !serverURLKnown && !localHostUIPublicOrigin(origin) {
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", methods)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Access-Control-Request-Private-Network")), "true") {
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
	}
	w.Header().Set("Access-Control-Max-Age", "600")
	w.Header().Set("Vary", "Origin")
	return true
}

func writeLocalHostUIPublicCORS(w http.ResponseWriter, r *http.Request, methods string) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}
	if !localHostUIPublicOrigin(origin) {
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", methods)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Access-Control-Request-Private-Network")), "true") {
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
	}
	w.Header().Set("Access-Control-Max-Age", "600")
	w.Header().Set("Vary", "Origin")
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
