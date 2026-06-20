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
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

type hostInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Status  string `json:"status"`
	Next    string `json:"next"`
}

type registerResponse struct {
	Type     string                `json:"type"`
	ID       string                `json:"host_id"`
	PublicID string                `json:"public_id"`
	Action   string                `json:"action"`
	Sessions []hostSessionDispatch `json:"sessions"`
	Error    string                `json:"error"`
}

type hostSessionDispatch struct {
	ID                 string    `json:"id"`
	Target             string    `json:"target"`
	Viewer             string    `json:"viewer"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
	RoutedHostID       string    `json:"routed_host_id"`
	RoutedHostPublicID string    `json:"routed_host_public_id"`
	RoutedHostname     string    `json:"routed_hostname"`
}

type hostIdentity struct {
	InstallID string
	HostID    string
	PublicID  string
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
}

func main() {
	jsonMode := flag.Bool("json", false, "print machine-readable output")
	noPause := flag.Bool("no-pause", false, "do not wait for Enter before exit")
	serverURL := flag.String("server", "", "register this host against an UnyDesk server, for example http://127.0.0.1:8890")
	flag.Parse()

	info := hostInfo{
		Name:    "UnyDesk Host",
		Version: "0.1.0-alpha",
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Status:  "bootstrap",
		Next:    "Host tunnel, screen capture, and input control will be added next.",
	}

	if *jsonMode {
		printJSON(info)
		return
	}

	printBanner(info)
	if *serverURL != "" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		if err := runPersistentHost(ctx, *serverURL, info); err != nil && ctx.Err() == nil {
			fmt.Println()
			fmt.Printf("Registration error: %v\n", err)
		}
	}

	if runtime.GOOS == "windows" && !*noPause {
		waitForEnter()
	}
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
	fmt.Println("This binary is currently a bootstrap utility.")
	fmt.Println("It validates the download and target platform, and can now")
	fmt.Println("open a resilient WebSocket registration channel to an UnyDesk server.")
	fmt.Println()
	fmt.Println("Next step planned:")
	fmt.Printf("- %s\n", info.Next)
	fmt.Println()
	fmt.Println("Available options:")
	fmt.Println("- --json     print machine-readable output")
	fmt.Println("- --server   connect this host to an UnyDesk server over WebSocket")
	fmt.Println("- --no-pause exit immediately on Windows")
}

func runPersistentHost(ctx context.Context, serverURL string, info hostInfo) error {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	installID, err := loadOrCreateInstallID()
	if err != nil {
		return err
	}

	wsURL, err := toWebSocketURL(serverURL)
	if err != nil {
		return err
	}

	identity := hostIdentity{InstallID: installID}
	backoff := 2 * time.Second

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		err = connectAndServe(ctx, wsURL, serverURL, hostname, info, &identity)
		if ctx.Err() != nil {
			return nil
		}

		fmt.Println()
		fmt.Printf("Connection lost: %v\n", err)
		fmt.Printf("Reconnecting in %s...\n", backoff)

		select {
		case <-ctx.Done():
			return nil
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

func connectAndServe(ctx context.Context, wsURL, serverURL, hostname string, info hostInfo, identity *hostIdentity) error {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]any{
		"type": "register",
		"host": map[string]string{
			"install_id": identity.InstallID,
			"name":       info.Name,
			"os":         info.OS,
			"arch":       info.Arch,
			"version":    info.Version,
			"hostname":   hostname,
		},
	}); err != nil {
		return err
	}

	var registered registerResponse
	if err := conn.ReadJSON(&registered); err != nil {
		return err
	}
	if registered.Type == "error" {
		return fmt.Errorf(registered.Error)
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
	fmt.Println("Transport : WebSocket")
	fmt.Println()
	fmt.Println("Heartbeat loop started. Press Ctrl+C to stop.")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	activeSessions := make(map[string]struct{})
	var activeMu sync.Mutex

	processHeartbeat := func() error {
		if err := conn.WriteJSON(map[string]string{"type": "heartbeat"}); err != nil {
			return fmt.Errorf("heartbeat send failed: %w", err)
		}
		var ack registerResponse
		if err := conn.ReadJSON(&ack); err != nil {
			return fmt.Errorf("heartbeat read failed: %w", err)
		}
		if ack.Type == "error" {
			return fmt.Errorf(ack.Error)
		}
		fmt.Printf("[%s] heartbeat ok for %s\n", time.Now().Format("15:04:05"), registered.PublicID)
		for _, session := range ack.Sessions {
			fmt.Println()
			fmt.Println("Incoming control request")
			fmt.Println("------------------------")
			fmt.Printf("Session  : %s\n", session.ID)
			fmt.Printf("Viewer   : %s\n", session.Viewer)
			fmt.Printf("Target   : %s\n", session.Target)
			fmt.Printf("Host     : %s (%s)\n", session.RoutedHostname, session.RoutedHostPublicID)
			fmt.Printf("Created  : %s\n", session.CreatedAt.Local().Format(time.RFC1123))
			fmt.Printf("Status   : %s\n", session.Status)
			fmt.Println("Next     : screen stream and input channel wiring will attach here.")
			if err := conn.WriteJSON(map[string]string{
				"type":       "session_ack",
				"session_id": session.ID,
				"action":     "accept",
			}); err != nil {
				return fmt.Errorf("session ack send failed: %w", err)
			}
			var sessionAck registerResponse
			if err := conn.ReadJSON(&sessionAck); err != nil {
				return fmt.Errorf("session ack read failed: %w", err)
			}
			if sessionAck.Type == "error" {
				return fmt.Errorf(sessionAck.Error)
			}
			fmt.Printf("Acked    : %s (%s)\n", session.ID, sessionAck.Action)
			activeMu.Lock()
			_, exists := activeSessions[session.ID]
			if !exists {
				activeSessions[session.ID] = struct{}{}
			}
			activeMu.Unlock()
			if exists {
				continue
			}
			go func(sessionID string) {
				defer func() {
					activeMu.Lock()
					delete(activeSessions, sessionID)
					activeMu.Unlock()
				}()
				if err := establishHostAnswer(ctx, serverURL, sessionID); err != nil && ctx.Err() == nil {
					fmt.Printf("Host signaling error for %s: %v\n", sessionID, err)
				}
			}(session.ID)
		}
		return nil
	}

	if err := processHeartbeat(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := processHeartbeat(); err != nil {
				return err
			}
		}
	}
}

func establishHostAnswer(ctx context.Context, serverURL, sessionID string) error {
	sessionURL := strings.TrimRight(serverURL, "/") + "/api/v1/sessions/" + url.PathEscape(sessionID)
	answerURL := sessionURL + "/answer"
	candidatesURL := sessionURL + "/candidates"

	session, err := waitForSessionOffer(ctx, sessionURL, sessionID, 3*time.Minute)
	if err != nil {
		return err
	}

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	})
	if err != nil {
		return err
	}
	defer pc.Close()
	done := make(chan struct{})
	var doneOnce sync.Once
	stop := func() {
		doneOnce.Do(func() {
			close(done)
		})
	}

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		fmt.Printf("Host data channel announced for %s: %s\n", sessionID, dc.Label())
		dc.OnOpen(func() {
			fmt.Printf("Host data channel open for %s\n", sessionID)
		})
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			var payload map[string]any
			if err := json.Unmarshal(msg.Data, &payload); err != nil {
				_ = dc.SendText(`{"type":"error","message":"invalid json"}`)
				return
			}
			switch strings.TrimSpace(fmt.Sprint(payload["type"])) {
			case "hello":
				_ = dc.SendText(fmt.Sprintf(`{"type":"hello_ack","session_id":"%s","host_id":"%s","host_public_id":"%s"}`, sessionID, "", ""))
			case "ping":
				_ = dc.SendText(fmt.Sprintf(`{"type":"pong","session_id":"%s","sent_at":%q,"received_at":%q}`, sessionID, fmt.Sprint(payload["sent_at"]), time.Now().UTC().Format(time.RFC3339Nano)))
			case "mouse_move":
				fmt.Printf("Host input %s mouse_move x=%v y=%v\n", sessionID, payload["x"], payload["y"])
			case "mouse_click":
				fmt.Printf("Host input %s mouse_click button=%v x=%v y=%v\n", sessionID, payload["button"], payload["x"], payload["y"])
			case "key_down":
				fmt.Printf("Host input %s key_down key=%v code=%v\n", sessionID, payload["key"], payload["code"])
			case "key_up":
				fmt.Printf("Host input %s key_up key=%v code=%v\n", sessionID, payload["key"], payload["code"])
			default:
				_ = dc.SendText(`{"type":"unsupported"}`)
			}
		})
	})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Printf("Host peer state for %s: %s\n", sessionID, state.String())
		if state == webrtc.PeerConnectionStateClosed ||
			state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateDisconnected {
			stop()
		}
	})

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		_ = postJSON(candidatesURL, map[string]string{
			"candidate": candidate.ToJSON().Candidate,
			"source":    "host",
		})
	})

	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  session.OfferSDP,
	}); err != nil {
		return err
	}

	for _, candidate := range session.ViewerICECandidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		_ = pc.AddICECandidate(webrtc.ICECandidateInit{Candidate: candidate})
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		return err
	}

	select {
	case <-webrtc.GatheringCompletePromise(pc):
	case <-time.After(1500 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}

	local := pc.LocalDescription()
	if local == nil || strings.TrimSpace(local.SDP) == "" {
		return fmt.Errorf("local answer missing")
	}
	if err := postJSON(answerURL, map[string]string{"sdp": local.SDP}); err != nil {
		return err
	}

	fmt.Printf("Host answer published for %s\n", sessionID)
	appliedViewerCandidates := make(map[string]struct{}, len(session.ViewerICECandidates))
	for _, candidate := range session.ViewerICECandidates {
		appliedViewerCandidates[candidate] = struct{}{}
	}

	pollTicker := time.NewTicker(300 * time.Millisecond)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-done:
			return nil
		case <-pollTicker.C:
			snapshot, err := fetchSessionSnapshot(sessionURL)
			if err != nil {
				continue
			}
			for _, candidate := range snapshot.ViewerICECandidates {
				if strings.TrimSpace(candidate) == "" {
					continue
				}
				if _, ok := appliedViewerCandidates[candidate]; ok {
					continue
				}
				appliedViewerCandidates[candidate] = struct{}{}
				_ = pc.AddICECandidate(webrtc.ICECandidateInit{Candidate: candidate})
			}
		}
	}
}

func waitForSessionOffer(ctx context.Context, sessionURL, sessionID string, timeout time.Duration) (sessionSnapshot, error) {
	deadline := time.Now().Add(timeout)
	waitLogged := false

	for {
		session, err := fetchSessionSnapshot(sessionURL)
		if err != nil {
			return sessionSnapshot{}, err
		}
		if strings.TrimSpace(session.OfferSDP) != "" {
			return session, nil
		}
		if strings.EqualFold(strings.TrimSpace(session.Status), "closed") {
			return sessionSnapshot{}, fmt.Errorf("session %s was closed before offer publication", sessionID)
		}
		if !waitLogged {
			fmt.Printf("Waiting for viewer offer on session %s...\n", sessionID)
			waitLogged = true
		}
		if time.Now().After(deadline) {
			return sessionSnapshot{}, fmt.Errorf("offer not available for session %s within %s", sessionID, timeout)
		}
		select {
		case <-ctx.Done():
			return sessionSnapshot{}, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
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

func loadOrCreateInstallID() (string, error) {
	paths := installIDPaths()
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
