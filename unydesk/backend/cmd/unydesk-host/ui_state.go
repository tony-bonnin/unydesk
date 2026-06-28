package main

import (
	"strings"
	"sync"
	"time"
)

type hostUIStatus struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	Hostname        string `json:"hostname"`
	ServerURL       string `json:"server_url"`
	InstallID       string `json:"install_id"`
	PublicID        string `json:"public_id"`
	HostID          string `json:"host_id"`
	Role            string `json:"role"`
	Admin           bool   `json:"admin"`
	AccessEnabled   bool   `json:"access_enabled"`
	AccessState     string `json:"access_state"`
	AccountURL      string `json:"account_url"`
	ConnectionState string `json:"connection_state"`
	ConnectionNote  string `json:"connection_note"`
	LastHeartbeatAt string `json:"last_heartbeat_at"`
	LastError       string `json:"last_error"`
	LastSessionID   string `json:"last_session_id"`
	LastSessionMeta string `json:"last_session_meta"`
	PendingApproval bool   `json:"pending_approval"`
	PendingViewer   string `json:"pending_viewer"`
	LocalUIURL      string `json:"local_ui_url"`
	Connected       bool   `json:"connected"`
}

type hostUILiveState struct {
	mu     sync.RWMutex
	status hostUIStatus
}

var localHostUI = &hostUILiveState{
	status: hostUIStatus{
		ConnectionState: "Starting",
		ConnectionNote:  "Preparing host runtime.",
	},
}

func (s *hostUILiveState) setBootstrap(info hostInfo, hostname, serverURL, installID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Name = info.Name
	s.status.Version = info.Version
	s.status.Hostname = strings.TrimSpace(hostname)
	s.status.ServerURL = strings.TrimSpace(serverURL)
	s.status.InstallID = strings.TrimSpace(installID)
	s.status.AccountURL = linkedAccountURL(serverURL, installID, "")
	s.status.Role = "Host"
	s.status.Admin = processHasAdminRights()
	s.status.AccessEnabled = true
	s.status.AccessState = "Enabled"
	s.status.ConnectionState = "Connecting"
	s.status.ConnectionNote = "Opening outbound WebSocket transport."
}

func (s *hostUILiveState) setLocalUIURL(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LocalUIURL = strings.TrimSpace(value)
}

func (s *hostUILiveState) setConnected(identity hostIdentity, accountURL string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.HostID = strings.TrimSpace(identity.HostID)
	s.status.PublicID = strings.TrimSpace(identity.PublicID)
	s.status.AccountURL = strings.TrimSpace(accountURL)
	s.status.PendingApproval = false
	s.status.PendingViewer = ""
	s.status.ConnectionState = "Connected"
	s.status.ConnectionNote = connectedHostNote(s.status.AccessEnabled)
	s.status.LastError = ""
	s.status.Connected = true
}

func (s *hostUILiveState) noteHeartbeat() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastHeartbeatAt = time.Now().UTC().Format(time.RFC3339)
	if s.status.PendingApproval {
		s.status.ConnectionState = "Approval needed"
		if strings.TrimSpace(s.status.PendingViewer) != "" {
			s.status.ConnectionNote = "Waiting for local approval for " + strings.TrimSpace(s.status.PendingViewer) + "."
		} else {
			s.status.ConnectionNote = "Waiting for local approval for the current remote access request."
		}
	} else {
		s.status.ConnectionState = "Connected"
		s.status.ConnectionNote = heartbeatHostNote(s.status.AccessEnabled)
	}
	s.status.Connected = true
}

func (s *hostUILiveState) noteSession(session hostSessionDispatch) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastSessionID = strings.TrimSpace(session.ID)
	target := strings.TrimSpace(session.Target)
	viewer := strings.TrimSpace(session.Viewer)
	if target == "" && viewer == "" {
		s.status.LastSessionMeta = ""
		return
	}
	switch {
	case target != "" && viewer != "":
		s.status.LastSessionMeta = target + " · " + viewer
	case target != "":
		s.status.LastSessionMeta = target
	default:
		s.status.LastSessionMeta = viewer
	}
}

func (s *hostUILiveState) noteApprovalRequested(session hostSessionDispatch) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.PendingApproval = true
	s.status.PendingViewer = strings.TrimSpace(session.Viewer)
	s.status.ConnectionState = "Approval needed"
	if s.status.PendingViewer != "" {
		s.status.ConnectionNote = "Waiting for local approval for " + s.status.PendingViewer + "."
	} else {
		s.status.ConnectionNote = "Waiting for local approval for the current remote access request."
	}
}

func (s *hostUILiveState) clearApprovalRequest() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.PendingApproval = false
	s.status.PendingViewer = ""
	if s.status.Connected {
		s.status.ConnectionState = "Connected"
		s.status.ConnectionNote = connectedHostNote(s.status.AccessEnabled)
		return
	}
	s.status.ConnectionState = "Reconnecting"
	s.status.ConnectionNote = "Transport disconnected."
}

func (s *hostUILiveState) setDisconnected(err error, reconnectDelay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Connected = false
	s.status.PendingApproval = false
	s.status.PendingViewer = ""
	s.status.ConnectionState = "Reconnecting"
	if reconnectDelay > 0 {
		s.status.ConnectionNote = "Retrying in " + reconnectDelay.String() + "."
	} else {
		s.status.ConnectionNote = "Transport disconnected."
	}
	if err != nil {
		s.status.LastError = strings.TrimSpace(err.Error())
	}
}

func (s *hostUILiveState) setAccessEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.AccessEnabled = enabled
	if enabled {
		s.status.AccessState = "Enabled"
		if s.status.PendingApproval {
			s.status.ConnectionState = "Approval needed"
			if strings.TrimSpace(s.status.PendingViewer) != "" {
				s.status.ConnectionNote = "Waiting for local approval for " + strings.TrimSpace(s.status.PendingViewer) + "."
			}
		} else if s.status.Connected {
			s.status.ConnectionNote = connectedHostNote(true)
		}
		return
	}
	s.status.AccessState = "Paused"
	if s.status.PendingApproval {
		s.status.ConnectionState = "Approval needed"
		s.status.ConnectionNote = "Waiting for local approval while remote access is being stopped."
	} else if s.status.Connected {
		s.status.ConnectionNote = connectedHostNote(false)
	}
}

func (s *hostUILiveState) snapshot() hostUIStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func connectedHostNote(accessEnabled bool) string {
	if accessEnabled {
		return "Heartbeat loop is active and client sessions are allowed."
	}
	return "Heartbeat loop is active but client sessions are paused."
}

func heartbeatHostNote(accessEnabled bool) string {
	if accessEnabled {
		return "Heartbeat acknowledged. Client sessions are allowed."
	}
	return "Heartbeat acknowledged. Client sessions stay paused."
}
