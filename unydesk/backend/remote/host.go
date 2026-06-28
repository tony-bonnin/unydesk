package remote

import (
	"encoding/json"
	"time"
)

type Host struct {
	ID            string    `json:"id"`
	InstallID     string    `json:"install_id,omitempty"`
	Role          string    `json:"role"`
	AccessEnabled bool      `json:"access_enabled"`
	Name          string    `json:"name"`
	OS            string    `json:"os"`
	Arch          string    `json:"arch"`
	Version       string    `json:"version"`
	Hostname      string    `json:"hostname"`
	PublicID      string    `json:"public_id"`
	Status        string    `json:"status"`
	RegisteredAt  time.Time `json:"registered_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

type RegisterHostRequest struct {
	InstallID     string `json:"install_id"`
	Role          string `json:"role,omitempty"`
	AccessEnabled *bool  `json:"access_enabled,omitempty"`
	Name          string `json:"name"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	Version       string `json:"version"`
	Hostname      string `json:"hostname"`
}

type HostHeartbeatState struct {
	Role          string `json:"role,omitempty"`
	AccessEnabled *bool  `json:"access_enabled,omitempty"`
}

type HostWireMessage struct {
	Type      string                `json:"type"`
	Host      *RegisterHostRequest  `json:"host,omitempty"`
	HostID    string                `json:"host_id,omitempty"`
	PublicID  string                `json:"public_id,omitempty"`
	SessionID string                `json:"session_id,omitempty"`
	Action    string                `json:"action,omitempty"`
	Payload   json.RawMessage       `json:"payload,omitempty"`
	Sessions  []HostSessionDispatch `json:"sessions,omitempty"`
	Error     string                `json:"error,omitempty"`
}

type HostSessionDispatch struct {
	ID                 string    `json:"id"`
	Target             string    `json:"target"`
	Viewer             string    `json:"viewer"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
	RoutedHostID       string    `json:"routed_host_id,omitempty"`
	RoutedHostPublicID string    `json:"routed_host_public_id,omitempty"`
	RoutedHostname     string    `json:"routed_hostname,omitempty"`
}
