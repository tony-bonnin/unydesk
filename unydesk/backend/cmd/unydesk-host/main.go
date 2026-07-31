package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"unydesk/remote"
)

var defaultServerURL = ""
var defaultInstallID = ""

const hostBuildID = "20260711-route-wake"

type hostInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Status  string `json:"status"`
	Next    string `json:"next"`
}

type registerResponse struct {
	Type      string                `json:"type"`
	ID        string                `json:"host_id"`
	PublicID  string                `json:"public_id"`
	SessionID string                `json:"session_id"`
	Action    string                `json:"action"`
	Payload   json.RawMessage       `json:"payload"`
	Sessions  []hostSessionDispatch `json:"sessions"`
	Error     string                `json:"error"`
}

type hostSessionDispatch struct {
	ID                 string    `json:"id"`
	Target             string    `json:"target"`
	Viewer             string    `json:"viewer"`
	ViewerLabel        string    `json:"viewer_label"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	OfferDigest        string    `json:"offer_digest"`
	RoutedHostID       string    `json:"routed_host_id"`
	RoutedHostPublicID string    `json:"routed_host_public_id"`
	RoutedHostname     string    `json:"routed_hostname"`
}

type hostIdentity struct {
	InstallID      string
	HostID         string
	PublicID       string
	AccessPassword string
}

type bootstrapConfig struct {
	Server         string `json:"server"`
	Domain         string `json:"domain,omitempty"`
	InstallID      string `json:"install_id"`
	PublicID       string `json:"public_id,omitempty"`
	Credential     string `json:"credential,omitempty"`
	CredentialType string `json:"credential_type,omitempty"`
}

type sessionSnapshot struct {
	ID                  string   `json:"id"`
	Target              string   `json:"target"`
	Viewer              string   `json:"viewer"`
	Status              string   `json:"status"`
	OfferSDP            string   `json:"offer_sdp"`
	AnswerSDP           string   `json:"answer_sdp"`
	ViewerICECandidates []string `json:"viewer_ice_candidates"`
	HostICECandidates   []string `json:"host_ice_candidates"`
	ScreenDataURL       string   `json:"screen_data_url"`
	ScreenCaptureError  string   `json:"screen_capture_error"`
}

type incomingFileTransfer struct {
	ID                string
	Name              string
	DeclaredSize      int64
	ReceivedBytes     int64
	NextChunkIndex    int
	TempPath          string
	DestinationDir    string
	DestinationPath   string
	File              *os.File
	LastProgressAt    time.Time
	LastProgressBytes int64
}

var (
	fileTransferMu    sync.Mutex
	incomingTransfers = make(map[string]*incomingFileTransfer)
	hostAccessMu      sync.RWMutex
	hostAccessEnabled = true
)

func currentHostAccessEnabled() bool {
	hostAccessMu.RLock()
	defer hostAccessMu.RUnlock()
	return hostAccessEnabled
}

func setHostAccessEnabled(enabled bool) {
	hostAccessMu.Lock()
	hostAccessEnabled = enabled
	hostAccessMu.Unlock()
	localHostUI.setAccessEnabled(enabled)
}

func main() {
	jsonMode := flag.Bool("json", false, "print machine-readable output")
	diagnoseMode := flag.Bool("diagnose", false, "run host diagnostics and exit")
	elevateMode := flag.Bool("elevate", false, "on Windows, relaunch as administrator when needed")
	noPause := flag.Bool("no-pause", false, "do not wait for Enter before exit")
	consoleMode := flag.Bool("console", false, "show the legacy console window on Windows instead of tray mode")
	serverURL := flag.String("server", "", "register this host against an UnyDesk server, for example http://127.0.0.1:8890")
	installIDFlag := flag.String("install-id", "", "force the install identity used by this host")
	flag.Parse()

	info := hostInfo{
		Name:    "UnyDesk Host",
		Version: "0.1.0-alpha+" + hostBuildID,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Status:  "bootstrap",
		Next:    "Remote fabric with adaptive H.264 WebRTC and transport fallbacks is active.",
	}

	if *jsonMode {
		printJSON(info)
		return
	}

	if *elevateMode && runtime.GOOS == "windows" && !processHasAdminRights() {
		if err := relaunchProcessElevated(); err != nil {
			fmt.Printf("Elevation failed: %v\n", err)
			if *consoleMode && !*noPause {
				waitForEnter()
			}
		}
		return
	}

	windowsTrayMode := runtime.GOOS == "windows" && !*consoleMode
	if windowsTrayMode {
		if logFile, logPath, err := startHostFileLogging(); err == nil {
			defer logFile.Close()
			fmt.Printf("\n--- UnyDesk Host start %s ---\n", time.Now().Format(time.RFC3339))
			fmt.Printf("Log file: %s\n", logPath)
		}
		hideConsoleWindow()
	}

	sidecar := loadBootstrapConfigFromSidecar()
	resolvedServerURL, serverSource := resolveServerURL(*serverURL, sidecar)
	if *diagnoseMode {
		if !windowsTrayMode {
			printBanner(info)
		}
		runHostDiagnostics(resolvedServerURL, serverSource, resolveInstallIDOverride(*installIDFlag, sidecar), info)
		if runtime.GOOS == "windows" && *consoleMode && !*noPause {
			waitForEnter()
		}
		return
	}

	prewarmRealtimeEncodersAsync()
	if !windowsTrayMode {
		printBanner(info)
	}
	if resolvedServerURL != "" || windowsTrayMode {
		if serverSource != "" {
			fmt.Printf("Server source : %s\n", serverSource)
			fmt.Printf("Server target : %s\n", resolvedServerURL)
			fmt.Println()
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		autoOpenUI := runtime.GOOS == "windows" && !windowsTrayMode && !*noPause && strings.TrimSpace(currentRuntimeServerCredential()) == ""
		if err := runPersistentHost(ctx, stop, resolvedServerURL, info, resolveInstallIDOverride(*installIDFlag, sidecar), strings.TrimSpace(sidecar.PublicID), autoOpenUI, windowsTrayMode); err != nil && ctx.Err() == nil {
			fmt.Println()
			fmt.Printf("Registration error: %v\n", err)
		}
	}

	if runtime.GOOS == "windows" && *consoleMode && !*noPause {
		waitForEnter()
	}
}

func prewarmRealtimeEncodersAsync() {
	go func() {
		started := time.Now()
		ffmpegPath, ok := lookupFFmpegBinary()
		if !ok {
			fmt.Println("FFmpeg warmup: skipped (encoder not found)")
			return
		}
		h265, h265OK := lookupH265Encoder(ffmpegPath)
		av1, av1OK := lookupAV1Encoder(ffmpegPath)
		h265Status := "unavailable"
		if h265OK {
			h265Status = h265
		}
		av1Status := "unavailable"
		if av1OK {
			av1Status = av1
		}
		fmt.Printf("FFmpeg warmup: ready in %s, h265=%s, av1=%s, path=%s\n", time.Since(started).Round(time.Millisecond), h265Status, av1Status, ffmpegPath)
	}()
}

func printJSON(info hostInfo) {
	fmt.Printf("{\n")
	fmt.Printf("  \"name\": %q,\n", info.Name)
	fmt.Printf("  \"version\": %q,\n", info.Version)
	fmt.Printf("  \"os\": %q,\n", info.OS)
	fmt.Printf("  \"arch\": %q,\n", info.Arch)
	fmt.Printf("  \"status\": %q,\n", info.Status)
	fmt.Printf("  \"next\": %q\n", info.Next)
	fmt.Printf("}\n")
}

func printBanner(info hostInfo) {
	fmt.Println("UnyDesk Host")
	fmt.Println("============")
	fmt.Println()
	fmt.Printf("Version : %s\n", info.Version)
	fmt.Printf("System  : %s/%s\n", info.OS, info.Arch)
	fmt.Printf("Status  : %s\n", info.Status)
	fmt.Println()
	fmt.Println("This binary now uses an outbound control tunnel plus a")
	fmt.Println("remote fabric for screen capture, video encoding, and")
	fmt.Println("transport fallback selection.")
	fmt.Println()
	fmt.Println("Available options:")
	fmt.Println("- --json     print machine-readable output")
	fmt.Println("- --diagnose run host diagnostics and exit")
	fmt.Println("- --elevate  relaunch as administrator on Windows")
	fmt.Println("- --server   connect this host to an UnyDesk server over WebSocket")
	fmt.Println("- --console  keep the legacy Windows console window instead of tray mode")
	fmt.Println("- --no-pause exit immediately on Windows")
}

func resolveServerURL(flagValue string, sidecar bootstrapConfig) (string, string) {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value, "command line"
	}
	if value := strings.TrimSpace(os.Getenv("UNYDESK_SERVER")); value != "" {
		return value, "environment"
	}
	if value := strings.TrimSpace(defaultServerURL); value != "" {
		return value, "embedded default"
	}
	if value := strings.TrimSpace(sidecar.Server); value != "" {
		return value, "sidecar config"
	}
	return "", ""
}

func loadBootstrapConfigFromSidecar() bootstrapConfig {
	for _, path := range bootstrapConfigCandidates() {
		if value, ok := readBootstrapConfigFile(path); ok {
			fmt.Printf("Bootstrap: loaded sidecar from %s\n", path)
			setBootstrapRuntime(value, path)
			return value
		}
	}
	fmt.Printf("Bootstrap: no sidecar found. Checked: %s\n", strings.Join(bootstrapConfigCandidates(), ", "))
	setBootstrapRuntime(bootstrapConfig{}, defaultBootstrapRuntimePath())
	return bootstrapConfig{}
}

func readBootstrapConfigFile(path string) (bootstrapConfig, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return bootstrapConfig{}, false
	}
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return bootstrapConfig{}, false
	}
	if strings.HasPrefix(raw, "{") {
		var payload bootstrapConfig
		if err := json.Unmarshal(data, &payload); err == nil {
			payload.Server = strings.TrimSpace(payload.Server)
			if payload.Server != "" && !looksLikeServerAddress(payload.Server) {
				payload.Server = ""
			}
			payload.Domain = strings.TrimSpace(payload.Domain)
			payload.InstallID = strings.TrimSpace(payload.InstallID)
			payload.PublicID = strings.TrimSpace(payload.PublicID)
			payload.Credential = strings.TrimSpace(payload.Credential)
			payload.CredentialType = strings.TrimSpace(payload.CredentialType)
			return payload, payload.Server != "" || payload.Domain != "" || payload.InstallID != "" || payload.PublicID != "" || payload.Credential != ""
		}
	}
	if !looksLikeServerAddress(raw) {
		return bootstrapConfig{}, false
	}
	return bootstrapConfig{Server: raw}, true
}

func looksLikeServerAddress(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil || strings.TrimSpace(parsed.Host) == "" {
			return false
		}
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https", "ws", "wss":
			return true
		default:
			return false
		}
	}

	host := value
	if splitHost, _, err := net.SplitHostPort(value); err == nil {
		host = splitHost
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil {
		return true
	}
	return strings.Contains(host, ".")
}

func resolveInstallIDOverride(flagValue string, sidecar bootstrapConfig) string {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("UNYDESK_INSTALL_ID")); value != "" {
		return value
	}
	if value := strings.TrimSpace(sidecar.InstallID); value != "" {
		return value
	}
	if value := strings.TrimSpace(defaultInstallID); value != "" {
		return value
	}
	return ""
}

func resolveServerCredential(sidecar bootstrapConfig) string {
	if value := strings.TrimSpace(os.Getenv("UNYDESK_SERVER_AUTH")); value != "" {
		return normalizeAuthorizationHeader(value)
	}
	if value := strings.TrimSpace(sidecar.Credential); value != "" {
		if strings.EqualFold(strings.TrimSpace(sidecar.CredentialType), "basic") || strings.EqualFold(strings.TrimSpace(sidecar.CredentialType), "") {
			return normalizeAuthorizationHeader(value)
		}
		if strings.Contains(value, " ") {
			return value
		}
		return strings.TrimSpace(sidecar.CredentialType) + " " + value
	}
	return ""
}

func runHostDiagnostics(serverURL, serverSource, installIDOverride string, info hostInfo) {
	fmt.Println()
	fmt.Println("UnyDesk Host diagnostics")
	fmt.Println("------------------------")
	fmt.Printf("Version : %s\n", info.Version)
	fmt.Printf("System  : %s/%s\n", info.OS, info.Arch)
	if runtime.GOOS == "windows" {
		fmt.Printf("Admin   : %t\n", processHasAdminRights())
	}
	if hostname, err := os.Hostname(); err == nil {
		fmt.Printf("Hostname: %s\n", hostname)
	}

	installID, err := loadOrCreateInstallID(installIDOverride)
	if err != nil {
		fmt.Printf("Install ID: FAIL (%v)\n", err)
	} else {
		fmt.Printf("Install ID: OK (%s)\n", installID)
	}

	if strings.TrimSpace(serverURL) == "" {
		fmt.Println("Server  : SKIP (no --server, UNYDESK_SERVER, sidecar, or embedded default)")
	} else {
		if strings.TrimSpace(serverSource) != "" {
			fmt.Printf("Server source: %s\n", serverSource)
		}
		fmt.Printf("Server target: %s\n", serverURL)
		diagnoseHTTP("healthz", strings.TrimRight(normalizeServerHTTPURL(serverURL), "/")+"/healthz")
		diagnoseHTTP("info", strings.TrimRight(normalizeServerHTTPURL(serverURL), "/")+"/api/v1/info")
	}

	started := time.Now()
	capture := defaultCaptureProvider()
	img, captureErr := capture.CapturePrimaryDisplay()
	if captureErr != nil {
		fmt.Printf("Screen capture: FAIL (%s, %v)\n", capture.Name(), captureErr)
	} else {
		bounds := img.Bounds()
		fmt.Printf("Screen capture: OK (%s, %dx%d in %s)\n", capture.Name(), bounds.Dx(), bounds.Dy(), time.Since(started).Round(time.Millisecond))
	}

	ffmpegPath, ffmpegOK := lookupFFmpegBinary()
	if ffmpegOK {
		fmt.Printf("FFmpeg : OK (%s)\n", ffmpegPath)
	} else {
		fmt.Println("FFmpeg : FAIL (H.264 WebRTC video disabled until ffmpeg is available)")
	}
	profiles := h264AdaptiveProfiles()
	if len(profiles) > 0 {
		policy := remoteFabricPolicyFor(ffmpegPath, profiles[0], capture)
		fmt.Printf("Fabric capture : %s [%s/%s]\n", policy.CaptureProvider, policy.CaptureKind, policy.CaptureStatus)
		fmt.Printf("Fabric encoder : %s [%s/%s/%s]\n", policy.EncoderProvider, policy.EncoderCodec, policy.EncoderMode, policy.EncoderStatus)
		fmt.Printf("Fabric transport: %s [%s], fallback=%s\n", policy.TransportPrimary, policy.TransportStatus, policy.TransportFallback)
		fmt.Printf("Fabric caps     : capture=%s\n", policy.CaptureCapabilities)
		fmt.Printf("Fabric caps     : encoder=%s\n", policy.EncoderCapabilities)
		fmt.Printf("Fabric caps     : transport=%s\n", policy.TransportCapabilities)
	}
}

func normalizeServerHTTPURL(serverURL string) string {
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		return ""
	}
	if !strings.Contains(serverURL, "://") {
		return "http://" + serverURL
	}
	if strings.HasPrefix(serverURL, "ws://") {
		return "http://" + strings.TrimPrefix(serverURL, "ws://")
	}
	if strings.HasPrefix(serverURL, "wss://") {
		return "https://" + strings.TrimPrefix(serverURL, "wss://")
	}
	return serverURL
}

func diagnoseHTTP(label, endpoint string) {
	client := http.Client{Timeout: 5 * time.Second}
	started := time.Now()
	resp, err := client.Get(endpoint)
	if err != nil {
		fmt.Printf("HTTP %-7s: FAIL (%v)\n", label, err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Printf("HTTP %-7s: FAIL (%s in %s)\n", label, resp.Status, time.Since(started).Round(time.Millisecond))
		return
	}
	fmt.Printf("HTTP %-7s: OK (%s in %s)\n", label, resp.Status, time.Since(started).Round(time.Millisecond))
}

func startHostFileLogging() (*os.File, string, error) {
	path := hostLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, "", err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, "", err
	}
	os.Stdout = file
	os.Stderr = file
	return file, path, nil
}

func hostLogPath() string {
	if runtime.GOOS == "windows" {
		if programData := strings.TrimSpace(os.Getenv("ProgramData")); programData != "" {
			return filepath.Join(programData, "UnyDesk", "unydesk-host.log")
		}
	}
	if configDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(configDir) != "" {
		return filepath.Join(configDir, "UnyDesk", "unydesk-host.log")
	}
	return filepath.Join(".", "unydesk-host.log")
}

func runPersistentHost(ctx context.Context, shutdown context.CancelFunc, serverURL string, info hostInfo, preferredInstallID, preferredPublicID string, autoOpenUI bool, trayMode bool) error {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	installID, err := loadOrCreateInstallID(preferredInstallID)
	if err != nil {
		return err
	}
	accessPassword, err := rotatePersistentHostAccessPassword()
	if err != nil {
		return err
	}
	setHostAccessEnabled(true)
	publicID := strings.TrimSpace(preferredPublicID)
	if publicID == "" {
		publicID = remote.StablePublicID("host", installID)
	}
	identity := hostIdentity{InstallID: installID, PublicID: publicID, AccessPassword: accessPassword}
	localHostUI.setBootstrap(info, hostname, serverURL, installID, publicID, accessPassword, strings.TrimSpace(currentRuntimeServerCredential()) != "")
	startLocalHostUI(ctx, autoOpenUI)
	if trayMode {
		startLocalHostTray(ctx, shutdown)
	}

	backoff := 2 * time.Second

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		currentRuntime := currentBootstrapRuntime()
		currentServerURL, _ := resolveServerURL("", currentRuntime)
		currentCredential := strings.TrimSpace(currentRuntimeServerCredential())
		if strings.TrimSpace(currentServerURL) == "" {
			localHostUI.setAwaitingProvisioning(currentServerURL)
			fmt.Println("Bootstrap: waiting for a server route from the web bootstrap API.")
			if !waitForBootstrapUpdate(ctx) {
				return nil
			}
			continue
		}

		wsURL, err := toWebSocketURL(currentServerURL)
		if err != nil {
			localHostUI.setDisconnected(err, backoff)
			fmt.Printf("Bootstrap: invalid server route %q (%v)\n", currentServerURL, err)
			if !waitForBootstrapUpdate(ctx) {
				return nil
			}
			continue
		}

		localHostUI.setConnecting(currentServerURL, currentCredential != "")
		err = connectAndServe(ctx, wsURL, currentServerURL, currentCredential, hostname, info, &identity)
		if ctx.Err() != nil {
			return nil
		}

		fmt.Println()
		fmt.Printf("Connection lost: %v\n", err)
		fmt.Printf("Reconnecting in %s...\n", backoff)
		localHostUI.setDisconnected(err, backoff)

		select {
		case <-ctx.Done():
			return nil
		case <-bootstrapRuntimeWake:
			backoff = 2 * time.Second
			continue
		case <-time.After(backoff):
		}

		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func connectAndServe(ctx context.Context, wsURL, serverURL, serverCredential, hostname string, info hostInfo, identity *hostIdentity) error {
	dialer := *websocket.DefaultDialer
	conn, resp, err := dialer.Dial(wsURL, serverAuthHeaders(serverURL, serverCredential))
	if err != nil {
		return presentableWebSocketDialError(err, resp)
	}
	defer conn.Close()

	var writeMu sync.Mutex
	writeJSON := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(v)
	}

	if err := writeJSON(map[string]any{
		"type": "register",
		"host": map[string]any{
			"install_id":      identity.InstallID,
			"role":            "host",
			"access_enabled":  currentHostAccessEnabled(),
			"access_password": currentHostAccessPassword(),
			"name":            info.Name,
			"os":              info.OS,
			"arch":            info.Arch,
			"version":         info.Version,
			"hostname":        hostname,
		},
	}); err != nil {
		return err
	}

	var registered registerResponse
	if err := conn.ReadJSON(&registered); err != nil {
		return err
	}
	if registered.Type == "error" {
		return fmt.Errorf("%s", registered.Error)
	}
	if registered.Type != "registered" {
		return fmt.Errorf("unexpected websocket response %q", registered.Type)
	}

	if identity.HostID != "" && identity.HostID != registered.ID {
		fmt.Printf("Warning: host ID changed unexpectedly from %s to %s\n", identity.HostID, registered.ID)
	}
	if identity.PublicID != "" && identity.PublicID != registered.PublicID {
		fmt.Printf("Warning: public ID changed unexpectedly from %s to %s\n", identity.PublicID, registered.PublicID)
	}

	firstConnect := identity.HostID == ""
	identity.HostID = registered.ID
	identity.PublicID = registered.PublicID
	accountURL := linkedAccountURL(serverURL, identity.InstallID, registered.PublicID)
	localHostUI.setConnected(*identity, accountURL)
	runtimeCfg, runtimeErr := fetchRuntimeConfig(serverURL, serverCredential)
	if runtimeErr != nil {
		fmt.Printf("Warning: realtime runtime config unavailable: %v\n", runtimeErr)
		runtimeCfg = runtimeConfig{Features: normalizeRuntimeFeatures(runtimeFeatureWire{}, nil, nil)}
	}

	fmt.Println()
	if firstConnect {
		fmt.Println("Registration successful")
		fmt.Println("-----------------------")
	} else {
		fmt.Println("Reconnected successfully")
		fmt.Println("------------------------")
	}
	fmt.Printf("Server  : %s\n", serverURL)
	fmt.Printf("Host ID : %s\n", registered.ID)
	fmt.Printf("Public ID : %s\n", registered.PublicID)
	fmt.Printf("Install ID : %s\n", identity.InstallID)
	fmt.Printf("Access Password : %s\n", currentHostAccessPassword())
	fmt.Println("Transport : WebSocket")
	fmt.Printf("WebRTC ICE : %d server(s)\n", len(runtimeCfg.ICEServers))
	fmt.Printf("Realtime : H265=%t H264=%t AV1=%t QUIC=%t codecs=%s\n", runtimeCfg.Features.H265, runtimeCfg.Features.H264, runtimeCfg.Features.AV1, runtimeCfg.Features.QUIC, strings.Join(runtimeCfg.Features.PreferredVideoCodecs, ","))
	if linkURL := accountURL; linkURL != "" {
		fmt.Printf("Browser link : %s\n", linkURL)
	}
	fmt.Println()
	fmt.Println("Heartbeat loop started. Press Ctrl+C to stop.")

	type activeSessionWorker struct {
		stop             chan struct{}
		startedAt        time.Time
		sessionUpdatedAt time.Time
		offerDigest      string
		requestFallback  func(string)
	}
	activeSessions := make(map[string]activeSessionWorker)
	var activeMu sync.Mutex
	defer func() {
		activeMu.Lock()
		defer activeMu.Unlock()
		for _, worker := range activeSessions {
			close(worker.stop)
		}
	}()

	sessionControlActive := func(sessionID string) bool {
		sessionID = strings.TrimSpace(sessionID)
		if sessionID == "" {
			return false
		}
		activeMu.Lock()
		defer activeMu.Unlock()
		_, exists := activeSessions[sessionID]
		return exists
	}

	startSessionRealtime := func(session hostSessionDispatch, restart bool) {
		sessionID := strings.TrimSpace(session.ID)
		if sessionID == "" {
			return
		}
		activeMu.Lock()
		if worker, exists := activeSessions[sessionID]; exists {
			if !restart {
				activeMu.Unlock()
				fmt.Printf("Realtime : session %s already active; keeping existing WebRTC worker\n", sessionID)
				return
			}
			incomingOfferDigest := strings.TrimSpace(session.OfferDigest)
			currentOfferDigest := strings.TrimSpace(worker.offerDigest)
			if incomingOfferDigest == "" || currentOfferDigest == "" || incomingOfferDigest == currentOfferDigest {
				workerAge := time.Since(worker.startedAt)
				if workerAge < 4*time.Second {
					activeMu.Unlock()
					if currentOfferDigest != "" {
						fmt.Printf("Realtime : session %s duplicate offered dispatch ignored; offer digest %s is already active for %s\n", sessionID, currentOfferDigest, workerAge.Round(time.Millisecond))
					} else {
						fmt.Printf("Realtime : session %s duplicate offered dispatch ignored; worker is already active for %s\n", sessionID, workerAge.Round(time.Millisecond))
					}
					return
				}
				close(worker.stop)
				delete(activeSessions, sessionID)
				if currentOfferDigest != "" {
					fmt.Printf("Realtime : session %s offered dispatch repeated without answer after %s; restarting stalled WebRTC worker for digest %s\n", sessionID, workerAge.Round(time.Millisecond), currentOfferDigest)
				} else {
					fmt.Printf("Realtime : session %s offered dispatch repeated without answer after %s; restarting stalled WebRTC worker\n", sessionID, workerAge.Round(time.Millisecond))
				}
			} else {
				close(worker.stop)
				delete(activeSessions, sessionID)
				fmt.Printf("Realtime : session %s received a fresh offer; restarting WebRTC worker after %s (%s -> %s)\n", sessionID, time.Since(worker.startedAt).Round(time.Millisecond), currentOfferDigest, incomingOfferDigest)
			}
		}
		stop := make(chan struct{})
		activeSessions[sessionID] = activeSessionWorker{stop: stop, startedAt: time.Now(), sessionUpdatedAt: session.UpdatedAt, offerDigest: strings.TrimSpace(session.OfferDigest)}
		activeMu.Unlock()

		sessionURL := strings.TrimRight(serverURL, "/") + "/api/v1/sessions/" + url.PathEscape(sessionID)
		emitServerEvent := func(sessionID string, payload map[string]any) {
			if sessionID == "" || len(payload) == 0 {
				return
			}
			if err := writeJSON(map[string]any{
				"type":       "session_event",
				"session_id": sessionID,
				"payload":    payload,
			}); err != nil {
				fmt.Printf("Host session event %s send failed: %v\n", sessionID, err)
			}
		}
		go func() {
			defer func() {
				activeMu.Lock()
				if current, ok := activeSessions[sessionID]; ok && current.stop == stop {
					delete(activeSessions, sessionID)
				}
				activeMu.Unlock()
			}()
			runWebRTCSession(ctx, stop, serverURL, registered.ID, sessionURL, sessionID, runtimeCfg, emitServerEvent, func(handler func(string)) {
				activeMu.Lock()
				defer activeMu.Unlock()
				current, ok := activeSessions[sessionID]
				if !ok || current.stop != stop {
					return
				}
				current.requestFallback = handler
				activeSessions[sessionID] = current
			})
		}()
	}

	handleDispatch := func(session hostSessionDispatch) error {
		fmt.Println()
		fmt.Println("Incoming control request")
		fmt.Println("------------------------")
		fmt.Printf("Session  : %s\n", session.ID)
		fmt.Printf("Viewer   : %s\n", session.Viewer)
		fmt.Printf("Target   : %s\n", session.Target)
		fmt.Printf("Host     : %s (%s)\n", session.RoutedHostname, session.RoutedHostPublicID)
		fmt.Printf("Created  : %s\n", session.CreatedAt.Local().Format(time.RFC1123))
		fmt.Printf("Status   : %s\n", session.Status)
		fmt.Println("Next     : pure WebSocket control is active on this host.")
		localHostUI.noteSession(session)
		if !currentHostAccessEnabled() {
			fmt.Println("Access   : paused locally, refusing new client session.")
			if err := writeJSON(map[string]string{
				"type":       "session_ack",
				"session_id": session.ID,
				"action":     "busy",
			}); err != nil {
				return fmt.Errorf("session busy send failed: %w", err)
			}
			return nil
		}
		if runtime.GOOS == "windows" {
			if hasCachedApprovedHostAccess(session) {
				fmt.Println("Access   : recent local approval reused for this session after a brief interruption.")
				if err := writeJSON(map[string]string{
					"type":       "session_ack",
					"session_id": session.ID,
					"action":     "accept",
				}); err != nil {
					return fmt.Errorf("session ack send failed: %w", err)
				}
				restartRealtime := strings.EqualFold(strings.TrimSpace(session.Status), "offered")
				startSessionRealtime(session, restartRealtime)
				return nil
			}
			fmt.Println("Access   : waiting for local approval from the tray menu.")
			if !queuePendingHostApproval(session, func(action string) {
				if err := writeJSON(map[string]string{
					"type":       "session_ack",
					"session_id": session.ID,
					"action":     action,
				}); err != nil {
					fmt.Printf("session ack send failed: %v\n", err)
					return
				}
				if action != "accept" {
					return
				}
				restartRealtime := strings.EqualFold(strings.TrimSpace(session.Status), "offered")
				startSessionRealtime(session, restartRealtime)
			}) {
				fmt.Println("Access   : another request is already waiting for approval, refusing this one as busy.")
				if err := writeJSON(map[string]string{
					"type":       "session_ack",
					"session_id": session.ID,
					"action":     "busy",
				}); err != nil {
					return fmt.Errorf("session busy send failed: %w", err)
				}
			}
			return nil
		}
		fmt.Println("Access   : auto-accepted; local confirmation is unavailable on this platform.")
		if err := writeJSON(map[string]string{
			"type":       "session_ack",
			"session_id": session.ID,
			"action":     "accept",
		}); err != nil {
			return fmt.Errorf("session ack send failed: %w", err)
		}
		restartRealtime := strings.EqualFold(strings.TrimSpace(session.Status), "offered")
		startSessionRealtime(session, restartRealtime)
		return nil
	}

	repromptDispatch := func(session hostSessionDispatch) error {
		localHostUI.noteSession(session)
		if !currentHostAccessEnabled() {
			fmt.Println("Access   : local access is paused, approval prompt not reopened.")
			return nil
		}
		if hasCachedApprovedHostAccess(session) {
			fmt.Println("Access   : recent local approval already covers this session.")
			return nil
		}
		if runtime.GOOS != "windows" {
			return nil
		}
		fmt.Println("Access   : reopening local approval prompt.")
		if !repromptPendingHostApproval(session, func(action string) {
			if err := writeJSON(map[string]string{
				"type":       "session_ack",
				"session_id": session.ID,
				"action":     action,
			}); err != nil {
				fmt.Printf("session ack send failed: %v\n", err)
				return
			}
			if action != "accept" {
				return
			}
			restartRealtime := strings.EqualFold(strings.TrimSpace(session.Status), "offered")
			startSessionRealtime(session, restartRealtime)
		}) {
			fmt.Println("Access   : unable to reopen approval prompt while another request is pending.")
		}
		return nil
	}

	heartbeatDone := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		sendHeartbeat := func() error {
			return writeJSON(map[string]any{
				"type": "heartbeat",
				"payload": map[string]any{
					"role":            "host",
					"access_enabled":  currentHostAccessEnabled(),
					"access_password": currentHostAccessPassword(),
				},
			})
		}

		if err := sendHeartbeat(); err != nil {
			heartbeatDone <- fmt.Errorf("heartbeat send failed: %w", err)
			return
		}

		for {
			select {
			case <-ctx.Done():
				heartbeatDone <- nil
				return
			case <-ticker.C:
				if err := sendHeartbeat(); err != nil {
					heartbeatDone <- fmt.Errorf("heartbeat send failed: %w", err)
					return
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-heartbeatDone:
			return err
		default:
		}

		var msg registerResponse
		if err := conn.ReadJSON(&msg); err != nil {
			return fmt.Errorf("ws read failed: %w", err)
		}

		switch msg.Type {
		case "heartbeat_ack":
			fmt.Printf("[%s] heartbeat ok for %s\n", time.Now().Format("15:04:05"), registered.PublicID)
			localHostUI.noteHeartbeat()
			for _, session := range msg.Sessions {
				if err := handleDispatch(session); err != nil {
					return err
				}
			}
		case "session_acknowledged":
			fmt.Printf("Acked    : %s (%s)\n", msg.SessionID, msg.Action)
		case "approval_prompt":
			for _, session := range msg.Sessions {
				if err := repromptDispatch(session); err != nil {
					return err
				}
			}
		case "control":
			if reason, ok := parseScreenFallbackRequest(msg.Payload); ok {
				activeMu.Lock()
				worker, exists := activeSessions[msg.SessionID]
				fallback := worker.requestFallback
				activeMu.Unlock()
				if exists && fallback != nil {
					fmt.Printf("Host control %s screen fallback requested via signaling: %s\n", msg.SessionID, reason)
					fallback(reason)
					continue
				}
			}
			if !sessionControlActive(msg.SessionID) {
				fmt.Printf("Host control %s blocked until local approval is granted\n", msg.SessionID)
				if err := writeJSON(map[string]any{
					"type":       "session_event",
					"session_id": msg.SessionID,
					"payload": map[string]any{
						"type":   "control_blocked",
						"reason": "approval-required",
					},
				}); err != nil {
					fmt.Printf("Host session event %s blocked-notice send failed: %v\n", msg.SessionID, err)
				}
				continue
			}
			handleHostControlMessage(msg.SessionID, msg.Payload, func(sessionID string, payload map[string]any) {
				if sessionID == "" || len(payload) == 0 {
					return
				}
				if err := writeJSON(map[string]any{
					"type":       "session_event",
					"session_id": sessionID,
					"payload":    payload,
				}); err != nil {
					fmt.Printf("Host session event %s send failed: %v\n", sessionID, err)
				}
			})
		case "error":
			return fmt.Errorf("%s", msg.Error)
		}
	}
}

func handleHostControlMessage(sessionID string, raw json.RawMessage, emit func(string, map[string]any)) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		fmt.Printf("Host control %s invalid json\n", sessionID)
		return
	}

	switch strings.TrimSpace(fmt.Sprint(payload["type"])) {
	case "ping":
		fmt.Printf("Host control %s ping\n", sessionID)
		emit(sessionID, map[string]any{
			"type":        "pong",
			"received_at": time.Now().UTC().Format(time.RFC3339),
		})
	case "mouse_move":
		x := numberValue(payload["x"])
		y := numberValue(payload["y"])
		if err := injectMouseMove(x, y); err != nil {
			fmt.Printf("Host input %s mouse_move failed: %v\n", sessionID, err)
			emit(sessionID, map[string]any{
				"type":    "input_error",
				"control": "mouse_move",
				"error":   err.Error(),
			})
			return
		}
		fmt.Printf("Host input %s mouse_move x=%0.4f y=%0.4f\n", sessionID, x, y)
	case "mouse_down":
		button := int(numberValue(payload["button"]))
		x := numberValue(payload["x"])
		y := numberValue(payload["y"])
		if err := injectMouseButton(button, x, y, true); err != nil {
			fmt.Printf("Host input %s mouse_down failed: %v\n", sessionID, err)
			emit(sessionID, map[string]any{
				"type":    "input_error",
				"control": "mouse_down",
				"error":   err.Error(),
			})
			return
		}
		fmt.Printf("Host input %s mouse_down button=%d x=%0.4f y=%0.4f\n", sessionID, button, x, y)
	case "mouse_up":
		button := int(numberValue(payload["button"]))
		x := numberValue(payload["x"])
		y := numberValue(payload["y"])
		if err := injectMouseButton(button, x, y, false); err != nil {
			fmt.Printf("Host input %s mouse_up failed: %v\n", sessionID, err)
			emit(sessionID, map[string]any{
				"type":    "input_error",
				"control": "mouse_up",
				"error":   err.Error(),
			})
			return
		}
		fmt.Printf("Host input %s mouse_up button=%d x=%0.4f y=%0.4f\n", sessionID, button, x, y)
	case "mouse_wheel":
		deltaY := int(numberValue(payload["delta_y"]))
		x := numberValue(payload["x"])
		y := numberValue(payload["y"])
		if err := injectMouseWheel(x, y, deltaY); err != nil {
			fmt.Printf("Host input %s mouse_wheel failed: %v\n", sessionID, err)
			emit(sessionID, map[string]any{
				"type":    "input_error",
				"control": "mouse_wheel",
				"error":   err.Error(),
			})
			return
		}
		fmt.Printf("Host input %s mouse_wheel delta_y=%d x=%0.4f y=%0.4f\n", sessionID, deltaY, x, y)
	case "mouse_click":
		button := int(numberValue(payload["button"]))
		x := numberValue(payload["x"])
		y := numberValue(payload["y"])
		if err := injectMouseClick(button, x, y); err != nil {
			fmt.Printf("Host input %s mouse_click failed: %v\n", sessionID, err)
			emit(sessionID, map[string]any{
				"type":    "input_error",
				"control": "mouse_click",
				"error":   err.Error(),
			})
			return
		}
		fmt.Printf("Host input %s mouse_click button=%d x=%0.4f y=%0.4f\n", sessionID, button, x, y)
	case "key_down":
		key := stringValue(payload["key"])
		code := stringValue(payload["code"])
		if err := injectKeyEvent(key, code, true); err != nil {
			fmt.Printf("Host input %s key_down failed: %v\n", sessionID, err)
			emit(sessionID, map[string]any{
				"type":    "input_error",
				"control": "key_down",
				"error":   err.Error(),
			})
			return
		}
		fmt.Printf("Host input %s key_down key=%s code=%s\n", sessionID, key, code)
	case "key_up":
		key := stringValue(payload["key"])
		code := stringValue(payload["code"])
		if err := injectKeyEvent(key, code, false); err != nil {
			fmt.Printf("Host input %s key_up failed: %v\n", sessionID, err)
			emit(sessionID, map[string]any{
				"type":    "input_error",
				"control": "key_up",
				"error":   err.Error(),
			})
			return
		}
		fmt.Printf("Host input %s key_up key=%s code=%s\n", sessionID, key, code)
	case "clipboard_get":
		text, err := readClipboardText()
		if err != nil {
			fmt.Printf("Host clipboard %s read failed: %v\n", sessionID, err)
			emit(sessionID, map[string]any{
				"type":   "clipboard",
				"action": "get",
				"status": "error",
				"error":  err.Error(),
			})
			return
		}
		emit(sessionID, map[string]any{
			"type":   "clipboard",
			"action": "get",
			"status": "ok",
			"text":   text,
		})
	case "clipboard_set":
		text := stringValue(payload["text"])
		if err := writeClipboardText(text); err != nil {
			fmt.Printf("Host clipboard %s write failed: %v\n", sessionID, err)
			emit(sessionID, map[string]any{
				"type":   "clipboard",
				"action": "set",
				"status": "error",
				"error":  err.Error(),
			})
			return
		}
		emit(sessionID, map[string]any{
			"type":        "clipboard",
			"action":      "set",
			"status":      "ok",
			"length":      len(text),
			"received_at": time.Now().UTC().Format(time.RFC3339),
		})
	case "file_begin":
		beginIncomingFileTransfer(sessionID, payload, emit)
	case "file_chunk":
		writeIncomingFileTransferChunk(sessionID, payload, emit)
	case "file_complete":
		completeIncomingFileTransfer(sessionID, payload, emit)
	case "file_cancel":
		cancelIncomingFileTransfer(sessionID, payload, "cancelled by viewer", emit)
	case "screen_fallback_request":
		fmt.Printf("Host control %s could not route signaling fallback request to an active WebRTC worker\n", sessionID)
		emit(sessionID, map[string]any{
			"type":      "screen_status",
			"transport": "video",
			"action":    "fallback-unavailable",
			"reason":    "no active WebRTC worker could accept the signaling fallback request",
		})
	default:
		fmt.Printf("Host control %s unsupported type=%v\n", sessionID, payload["type"])
		emit(sessionID, map[string]any{
			"type":    "unsupported",
			"control": fmt.Sprint(payload["type"]),
		})
	}
}

func presentableWebSocketDialError(err error, resp *http.Response) error {
	if err == nil {
		return nil
	}
	if resp == nil {
		return err
	}
	detail := strings.TrimSpace(resp.Status)
	if authenticate := strings.TrimSpace(resp.Header.Get("WWW-Authenticate")); authenticate != "" {
		detail += ", WWW-Authenticate: " + authenticate
	}
	return fmt.Errorf("%w (%s)", err, detail)
}

func serverAuthHeaders(serverURL, explicitCredential string) http.Header {
	header := http.Header{}
	if value := strings.TrimSpace(explicitCredential); value != "" {
		header.Set("Authorization", value)
		return header
	}
	parsed, err := url.Parse(serverURL)
	if err != nil || parsed.User == nil {
		return header
	}
	password, _ := parsed.User.Password()
	header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(parsed.User.Username()+":"+password)))
	return header
}

func normalizeAuthorizationHeader(value string) string {
	if strings.Contains(value, " ") {
		return value
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(value))
}

func numberValue(value any) float64 {
	switch cast := value.(type) {
	case float64:
		return cast
	case float32:
		return float64(cast)
	case int:
		return float64(cast)
	case int64:
		return float64(cast)
	default:
		return 0
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func int64Value(value any) int64 {
	switch cast := value.(type) {
	case int64:
		return cast
	case int:
		return int64(cast)
	case float64:
		return int64(cast)
	case float32:
		return int64(cast)
	default:
		return 0
	}
}

func beginIncomingFileTransfer(sessionID string, payload map[string]any, emit func(string, map[string]any)) {
	transferID := stringValue(payload["transfer_id"])
	if transferID == "" {
		emit(sessionID, map[string]any{
			"type":   "file_transfer",
			"stage":  "error",
			"status": "error",
			"error":  "missing transfer id",
		})
		return
	}

	name := sanitizeTransferName(stringValue(payload["name"]))
	if name == "" {
		name = "unydesk-upload.bin"
	}
	declaredSize := int64Value(payload["size"])
	destinationDir := hostTransferDirectory()
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		emit(sessionID, map[string]any{
			"type":        "file_transfer",
			"transfer_id": transferID,
			"name":        name,
			"stage":       "error",
			"status":      "error",
			"error":       err.Error(),
		})
		return
	}

	tempFile, err := os.CreateTemp(destinationDir, "unydesk-upload-*.part")
	if err != nil {
		emit(sessionID, map[string]any{
			"type":        "file_transfer",
			"transfer_id": transferID,
			"name":        name,
			"stage":       "error",
			"status":      "error",
			"error":       err.Error(),
		})
		return
	}

	fileTransferMu.Lock()
	if existing := incomingTransfers[transferID]; existing != nil {
		fileTransferMu.Unlock()
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		cancelIncomingFileTransfer(sessionID, map[string]any{"transfer_id": transferID}, "replaced by a new transfer", emit)
		fileTransferMu.Lock()
	}
	incomingTransfers[transferID] = &incomingFileTransfer{
		ID:             transferID,
		Name:           name,
		DeclaredSize:   declaredSize,
		TempPath:       tempFile.Name(),
		DestinationDir: destinationDir,
		File:           tempFile,
	}
	fileTransferMu.Unlock()

	emit(sessionID, map[string]any{
		"type":        "file_transfer",
		"transfer_id": transferID,
		"name":        name,
		"stage":       "started",
		"status":      "ok",
		"total_bytes": declaredSize,
		"destination": destinationDir,
	})
}

func writeIncomingFileTransferChunk(sessionID string, payload map[string]any, emit func(string, map[string]any)) {
	transferID := stringValue(payload["transfer_id"])
	index := int(int64Value(payload["index"]))
	encoded := stringValue(payload["data"])
	if transferID == "" || encoded == "" {
		emit(sessionID, map[string]any{
			"type":   "file_transfer",
			"stage":  "error",
			"status": "error",
			"error":  "missing file chunk data",
		})
		return
	}

	fileTransferMu.Lock()
	transfer := incomingTransfers[transferID]
	fileTransferMu.Unlock()
	if transfer == nil {
		emit(sessionID, map[string]any{
			"type":        "file_transfer",
			"transfer_id": transferID,
			"stage":       "error",
			"status":      "error",
			"error":       "unknown transfer id",
		})
		return
	}
	if index != transfer.NextChunkIndex {
		cancelIncomingFileTransfer(sessionID, payload, fmt.Sprintf("unexpected chunk index %d", index), emit)
		return
	}

	chunk, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		cancelIncomingFileTransfer(sessionID, payload, fmt.Sprintf("invalid file chunk: %v", err), emit)
		return
	}
	if transfer.DeclaredSize > 0 && transfer.ReceivedBytes+int64(len(chunk)) > transfer.DeclaredSize {
		cancelIncomingFileTransfer(sessionID, payload, "transfer size exceeded declared limit", emit)
		return
	}
	if _, err := transfer.File.Write(chunk); err != nil {
		cancelIncomingFileTransfer(sessionID, payload, fmt.Sprintf("write failed: %v", err), emit)
		return
	}

	transfer.ReceivedBytes += int64(len(chunk))
	transfer.NextChunkIndex++
	now := time.Now()
	if transfer.LastProgressAt.IsZero() || now.Sub(transfer.LastProgressAt) >= 350*time.Millisecond || transfer.ReceivedBytes == transfer.DeclaredSize || transfer.ReceivedBytes-transfer.LastProgressBytes >= 256*1024 {
		transfer.LastProgressAt = now
		transfer.LastProgressBytes = transfer.ReceivedBytes
		emit(sessionID, map[string]any{
			"type":           "file_transfer",
			"transfer_id":    transferID,
			"name":           transfer.Name,
			"stage":          "progress",
			"status":         "ok",
			"bytes_received": transfer.ReceivedBytes,
			"total_bytes":    transfer.DeclaredSize,
		})
	}
}

func completeIncomingFileTransfer(sessionID string, payload map[string]any, emit func(string, map[string]any)) {
	transferID := stringValue(payload["transfer_id"])
	if transferID == "" {
		return
	}

	fileTransferMu.Lock()
	transfer := incomingTransfers[transferID]
	if transfer != nil {
		delete(incomingTransfers, transferID)
	}
	fileTransferMu.Unlock()
	if transfer == nil {
		emit(sessionID, map[string]any{
			"type":        "file_transfer",
			"transfer_id": transferID,
			"stage":       "error",
			"status":      "error",
			"error":       "unknown transfer id",
		})
		return
	}

	if transfer.File != nil {
		if err := transfer.File.Close(); err != nil {
			_ = os.Remove(transfer.TempPath)
			emit(sessionID, map[string]any{
				"type":        "file_transfer",
				"transfer_id": transferID,
				"name":        transfer.Name,
				"stage":       "error",
				"status":      "error",
				"error":       err.Error(),
			})
			return
		}
	}
	if transfer.DeclaredSize > 0 && transfer.ReceivedBytes != transfer.DeclaredSize {
		_ = os.Remove(transfer.TempPath)
		emit(sessionID, map[string]any{
			"type":        "file_transfer",
			"transfer_id": transferID,
			"name":        transfer.Name,
			"stage":       "error",
			"status":      "error",
			"error":       fmt.Sprintf("incomplete transfer: received %d of %d bytes", transfer.ReceivedBytes, transfer.DeclaredSize),
		})
		return
	}

	destinationPath := uniqueTransferPath(filepath.Join(transfer.DestinationDir, transfer.Name))
	if err := os.Rename(transfer.TempPath, destinationPath); err != nil {
		_ = os.Remove(transfer.TempPath)
		emit(sessionID, map[string]any{
			"type":        "file_transfer",
			"transfer_id": transferID,
			"name":        transfer.Name,
			"stage":       "error",
			"status":      "error",
			"error":       err.Error(),
		})
		return
	}

	emit(sessionID, map[string]any{
		"type":           "file_transfer",
		"transfer_id":    transferID,
		"name":           transfer.Name,
		"stage":          "completed",
		"status":         "ok",
		"bytes_received": transfer.ReceivedBytes,
		"total_bytes":    transfer.DeclaredSize,
		"path":           destinationPath,
	})
}

func cancelIncomingFileTransfer(sessionID string, payload map[string]any, reason string, emit func(string, map[string]any)) {
	transferID := stringValue(payload["transfer_id"])
	if transferID == "" {
		return
	}

	fileTransferMu.Lock()
	transfer := incomingTransfers[transferID]
	if transfer != nil {
		delete(incomingTransfers, transferID)
	}
	fileTransferMu.Unlock()

	name := ""
	if transfer != nil {
		name = transfer.Name
		if transfer.File != nil {
			_ = transfer.File.Close()
		}
		if transfer.TempPath != "" {
			_ = os.Remove(transfer.TempPath)
		}
	}

	emit(sessionID, map[string]any{
		"type":        "file_transfer",
		"transfer_id": transferID,
		"name":        name,
		"stage":       "error",
		"status":      "error",
		"error":       reason,
	})
}

func sanitizeTransferName(name string) string {
	base := strings.TrimSpace(filepath.Base(name))
	base = strings.ReplaceAll(base, "\\", "_")
	base = strings.ReplaceAll(base, "/", "_")
	base = strings.Trim(base, ". ")
	if base == "" || base == "." || base == ".." {
		return ""
	}
	return base
}

func hostTransferDirectory() string {
	if homeDir, err := os.UserHomeDir(); err == nil && strings.TrimSpace(homeDir) != "" {
		downloads := filepath.Join(homeDir, "Downloads")
		if err := os.MkdirAll(downloads, 0o755); err == nil {
			return downloads
		}
		desktop := filepath.Join(homeDir, "Desktop")
		if err := os.MkdirAll(desktop, 0o755); err == nil {
			return desktop
		}
	}
	fallback := filepath.Join(os.TempDir(), "UnyDesk Transfers")
	_ = os.MkdirAll(fallback, 0o755)
	return fallback
}

func uniqueTransferPath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	dir := filepath.Dir(path)
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(filepath.Base(path), ext)
	for index := 2; index <= 9999; index++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, index, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, time.Now().Unix(), ext))
}

func fetchSessionSnapshot(endpoint string) (sessionSnapshot, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return sessionSnapshot{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return sessionSnapshot{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return sessionSnapshot{}, fmt.Errorf("session fetch failed: %s", strings.TrimSpace(string(body)))
	}
	var session sessionSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return sessionSnapshot{}, err
	}
	return session, nil
}

func postJSON(endpoint string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("post failed: %s", strings.TrimSpace(string(raw)))
	}
	return nil
}

func linkedAccountURL(serverURL, installID, publicID string) string {
	if strings.TrimSpace(serverURL) == "" {
		return ""
	}
	if !strings.Contains(serverURL, "://") {
		serverURL = "http://" + serverURL
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		return ""
	}
	u.Path = "/account/"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func toScreenWebSocketURL(serverURL, sessionID, hostID string) (string, error) {
	if !strings.Contains(serverURL, "://") {
		serverURL = "http://" + serverURL
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported server scheme %q", u.Scheme)
	}
	u.Path = "/api/v1/sessions/" + url.PathEscape(sessionID) + "/screen/ws"
	query := url.Values{}
	query.Set("role", "host")
	query.Set("host_id", hostID)
	u.RawQuery = query.Encode()
	u.Fragment = ""
	return u.String(), nil
}

func toWebSocketURL(serverURL string) (string, error) {
	if !strings.Contains(serverURL, "://") {
		serverURL = "http://" + serverURL
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported server scheme %q", u.Scheme)
	}
	u.Path = "/api/v1/hosts/ws"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func waitForEnter() {
	fmt.Println()
	fmt.Print("Press Enter to close...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

func loadOrCreateInstallID(preferred string) (string, error) {
	paths := installIDPaths()
	if value := strings.TrimSpace(preferred); value != "" {
		syncInstallID(paths, value)
		return value, nil
	}
	for _, path := range paths {
		if value, ok := readInstallID(path); ok {
			syncInstallID(paths, value)
			return value, nil
		}
	}

	value := stableMachineInstallID()
	if strings.TrimSpace(value) == "" {
		randomValue, err := randomInstallID()
		if err != nil {
			return "", err
		}
		value = randomValue
	}

	syncInstallID(paths, value)
	return value, nil
}

func installIDPaths() []string {
	paths := make([]string, 0, 3)
	if configDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(configDir) != "" {
		paths = append(paths, filepath.Join(configDir, "UnyDesk", "host-install-id"))
	}
	if homeDir, err := os.UserHomeDir(); err == nil && strings.TrimSpace(homeDir) != "" {
		paths = append(paths, filepath.Join(homeDir, ".unydesk", "host-install-id"))
	}
	switch runtime.GOOS {
	case "windows":
		if programData := strings.TrimSpace(os.Getenv("ProgramData")); programData != "" {
			paths = append(paths, filepath.Join(programData, "UnyDesk", "host-install-id"))
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
		out = append(out, filepath.Join(".", "host-install-id"))
	}
	return out
}

func readInstallID(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", false
	}
	return value, true
}

func syncInstallID(paths []string, value string) {
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			continue
		}
		_ = os.WriteFile(path, []byte(value+"\n"), 0o600)
	}
}

func stableMachineInstallID() string {
	fingerprint := strings.TrimSpace(machineFingerprint())
	if fingerprint == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("unydesk-host:" + fingerprint))
	return base64.RawURLEncoding.EncodeToString(sum[:24])
}

func randomInstallID() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
