package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const Version = "0.1.0-alpha"

var defaultICEServers = []ICEServer{
	{URLs: []string{"stun:stun.l.google.com:19302"}},
}

type Settings struct {
	Name       string        `yaml:"name"`
	ListenAddr string        `yaml:"listen_addr"`
	ServerURL  string        `yaml:"server_url"`
	Security   Security      `yaml:"security"`
	Remote     RemoteRuntime `yaml:"remote"`
	WebRTC     WebRTC        `yaml:"webrtc"`
	HTTP3      HTTP3         `yaml:"http3"`
	Features   Features      `yaml:"features"`
	Paths      Paths         `yaml:"paths"`
}

type Security struct {
	AllowOrigin string `yaml:"allow_origin"`
}

type RemoteRuntime struct {
	SessionTTLMinutes    int `yaml:"session_ttl_minutes"`
	HostHeartbeatSeconds int `yaml:"host_heartbeat_seconds"`
}

type WebRTC struct {
	ICEServers []ICEServer `yaml:"ice_servers" json:"ice_servers"`
}

type ICEServer struct {
	URLs           []string `yaml:"urls" json:"urls"`
	Username       string   `yaml:"username,omitempty" json:"username,omitempty"`
	Credential     string   `yaml:"credential,omitempty" json:"credential,omitempty"`
	CredentialType string   `yaml:"credential_type,omitempty" json:"credentialType,omitempty"`
}

type Features struct {
	WebRTC               bool     `yaml:"webrtc" json:"webrtc"`
	H265                 bool     `yaml:"h265" json:"h265"`
	H264                 bool     `yaml:"h264" json:"h264"`
	AV1                  bool     `yaml:"av1" json:"av1"`
	QUIC                 bool     `yaml:"quic" json:"quic"`
	PreferredVideoCodecs []string `yaml:"preferred_video_codecs" json:"preferred_video_codecs"`
	configured           bool     `yaml:"-" json:"-"`
}

type HTTP3 struct {
	Enabled      bool   `yaml:"enabled"`
	CertFile     string `yaml:"cert_file"`
	KeyFile      string `yaml:"key_file"`
	Port         int    `yaml:"port"`
	RedirectHTTP bool   `yaml:"redirect_http"`
}

type Paths struct {
	PublicIDFile     string `yaml:"public_id_file"`
	HostDownloadsDir string `yaml:"host_downloads_dir"`
	FrontendDir      string `yaml:"frontend_dir"`
	UsersFile        string `yaml:"users_file"`
	HostsFile        string `yaml:"hosts_file"`
	TrustedHostsFile string `yaml:"trusted_hosts_file"`
}

func Load(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Settings{}, fmt.Errorf("read settings: %w", err)
	}

	var cfg Settings
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Settings{}, fmt.Errorf("parse settings: %w", err)
	}

	applyDefaults(&cfg)
	return cfg, nil
}

func applyDefaults(cfg *Settings) {
	if cfg.Name == "" {
		cfg.Name = "UnyDesk"
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8890"
	}
	applyServerURLEnv(cfg)
	if cfg.Security.AllowOrigin == "" {
		cfg.Security.AllowOrigin = "*"
	}
	if cfg.Remote.SessionTTLMinutes <= 0 {
		cfg.Remote.SessionTTLMinutes = 30
	}
	if cfg.Remote.HostHeartbeatSeconds <= 0 {
		cfg.Remote.HostHeartbeatSeconds = 35
	}
	if cfg.Paths.PublicIDFile == "" {
		cfg.Paths.PublicIDFile = "settings/public-id"
	}
	if cfg.Paths.HostDownloadsDir == "" {
		cfg.Paths.HostDownloadsDir = "tmp/host-downloads"
	}
	if cfg.Paths.FrontendDir == "" {
		cfg.Paths.FrontendDir = "../frontend/public"
	}
	if cfg.Paths.UsersFile == "" {
		cfg.Paths.UsersFile = "settings/users.json"
	}
	if cfg.Paths.HostsFile == "" {
		cfg.Paths.HostsFile = "settings/hosts.json"
	}
	if cfg.Paths.TrustedHostsFile == "" {
		cfg.Paths.TrustedHostsFile = "settings/trusted-hosts.json"
	}
	if assetsDir := os.Getenv("UNYDESK_ASSETS"); assetsDir != "" {
		cfg.Paths.FrontendDir = assetsDir
	}
	applyHTTP3Env(cfg)
	applyWebRTCEnv(cfg)
	cfg.WebRTC.ICEServers = normalizeICEServers(cfg.WebRTC.ICEServers)
	if len(cfg.WebRTC.ICEServers) == 0 && !envBool("UNYDESK_DISABLE_DEFAULT_STUN", false) {
		cfg.WebRTC.ICEServers = cloneICEServers(defaultICEServers)
	}
	applyFeatureDefaults(cfg)
	if cfg.HTTP3.Port <= 0 {
		cfg.HTTP3.Port = 8443
	}
}

func applyServerURLEnv(cfg *Settings) {
	for _, name := range []string{"UNYDESK_SERVER_URL", "SERVER_URL", "UNYDESK_PUBLIC_SERVER"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			cfg.ServerURL = value
			return
		}
	}
	cfg.ServerURL = strings.TrimSpace(cfg.ServerURL)
}

func applyWebRTCEnv(cfg *Settings) {
	if raw := strings.TrimSpace(os.Getenv("UNYDESK_ICE_SERVERS")); raw != "" {
		var servers []ICEServer
		if err := json.Unmarshal([]byte(raw), &servers); err != nil {
			fmt.Fprintf(os.Stderr, "warning: UNYDESK_ICE_SERVERS ignored: %v\n", err)
			return
		}
		cfg.WebRTC.ICEServers = servers
		return
	}

	servers := append([]ICEServer(nil), cfg.WebRTC.ICEServers...)
	if urls := envList("UNYDESK_STUN_URLS"); len(urls) > 0 {
		servers = append(servers, ICEServer{URLs: urls})
	}
	if urls := envList("UNYDESK_TURN_URLS"); len(urls) > 0 {
		servers = append(servers, ICEServer{
			URLs:           urls,
			Username:       strings.TrimSpace(os.Getenv("UNYDESK_TURN_USERNAME")),
			Credential:     strings.TrimSpace(os.Getenv("UNYDESK_TURN_CREDENTIAL")),
			CredentialType: "password",
		})
	}
	cfg.WebRTC.ICEServers = servers
}

func applyHTTP3Env(cfg *Settings) {
	cfg.HTTP3.Enabled = envBool("UNYDESK_HTTP3_ENABLED", cfg.HTTP3.Enabled)

	if value := strings.TrimSpace(os.Getenv("UNYDESK_HTTP3_CERT_FILE")); value != "" {
		cfg.HTTP3.CertFile = value
	}
	if value := strings.TrimSpace(os.Getenv("UNYDESK_HTTP3_KEY_FILE")); value != "" {
		cfg.HTTP3.KeyFile = value
	}
	if value := strings.TrimSpace(os.Getenv("UNYDESK_HTTP3_PORT")); value != "" {
		if port, err := strconv.Atoi(value); err == nil && port > 0 {
			cfg.HTTP3.Port = port
		}
	}
	cfg.HTTP3.RedirectHTTP = envBool("UNYDESK_HTTP3_REDIRECT_HTTP", cfg.HTTP3.RedirectHTTP)
}

func applyFeatureDefaults(cfg *Settings) {
	if !cfg.Features.configured {
		cfg.Features = Features{
			WebRTC:               true,
			H265:                 true,
			H264:                 true,
			AV1:                  true,
			QUIC:                 true,
			PreferredVideoCodecs: []string{"h264", "av1", "h265"},
		}
	}

	cfg.Features.WebRTC = envBool("UNYDESK_WEBRTC_ENABLED", cfg.Features.WebRTC)
	cfg.Features.H265 = envBool("UNYDESK_H265_ENABLED", cfg.Features.H265)
	cfg.Features.H264 = envBool("UNYDESK_H264_ENABLED", cfg.Features.H264)
	cfg.Features.AV1 = envBool("UNYDESK_AV1_ENABLED", cfg.Features.AV1)
	cfg.Features.QUIC = envBool("UNYDESK_QUIC_ENABLED", cfg.Features.QUIC)

	codecs := envList("UNYDESK_VIDEO_CODECS")
	if len(codecs) == 0 {
		codecs = cfg.Features.PreferredVideoCodecs
	}
	if len(codecs) == 0 {
		codecs = []string{"h264", "av1", "h265"}
	}
	cfg.Features.PreferredVideoCodecs = normalizeVideoCodecs(codecs, cfg.Features)
}

func (features *Features) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		WebRTC               *bool    `yaml:"webrtc"`
		H265                 *bool    `yaml:"h265"`
		H264                 *bool    `yaml:"h264"`
		AV1                  *bool    `yaml:"av1"`
		QUIC                 *bool    `yaml:"quic"`
		PreferredVideoCodecs []string `yaml:"preferred_video_codecs"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	features.configured = true
	features.WebRTC = boolPtrValue(raw.WebRTC, true)
	features.H265 = boolPtrValue(raw.H265, true)
	features.H264 = boolPtrValue(raw.H264, true)
	features.AV1 = boolPtrValue(raw.AV1, true)
	features.QUIC = boolPtrValue(raw.QUIC, true)
	features.PreferredVideoCodecs = raw.PreferredVideoCodecs
	return nil
}

func boolPtrValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func envBool(name string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch raw {
	case "":
		return fallback
	case "1", "true", "yes", "on", "enabled":
		return true
	case "0", "false", "no", "off", "disabled":
		return false
	default:
		return fallback
	}
}

func normalizeVideoCodecs(codecs []string, features Features) []string {
	normalized := make([]string, 0, len(codecs))
	seen := map[string]struct{}{}
	for _, raw := range codecs {
		codec := strings.ToLower(strings.TrimSpace(raw))
		switch codec {
		case "h265", "h.265", "hevc", "video/h265", "video/hevc":
			codec = "h265"
			if !features.H265 {
				continue
			}
		case "av1", "video/av1":
			codec = "av1"
			if !features.AV1 {
				continue
			}
		case "h264", "h.264", "video/h264":
			codec = "h264"
			if !features.H264 {
				continue
			}
		default:
			continue
		}
		if _, exists := seen[codec]; exists {
			continue
		}
		seen[codec] = struct{}{}
		normalized = append(normalized, codec)
	}
	if len(normalized) == 0 && features.H264 {
		normalized = append(normalized, "h264")
	}
	if len(normalized) == 0 && features.H265 {
		normalized = append(normalized, "h265")
	}
	return normalized
}

func envList(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func normalizeICEServers(servers []ICEServer) []ICEServer {
	normalized := make([]ICEServer, 0, len(servers))
	for _, server := range servers {
		urls := make([]string, 0, len(server.URLs))
		seen := make(map[string]struct{}, len(server.URLs))
		for _, rawURL := range server.URLs {
			value := strings.TrimSpace(rawURL)
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			urls = append(urls, value)
		}
		if len(urls) == 0 {
			continue
		}
		server.URLs = urls
		server.Username = strings.TrimSpace(server.Username)
		server.Credential = strings.TrimSpace(server.Credential)
		server.CredentialType = strings.TrimSpace(server.CredentialType)
		if server.Credential != "" && server.CredentialType == "" {
			server.CredentialType = "password"
		}
		normalized = append(normalized, server)
	}
	return normalized
}

func cloneICEServers(servers []ICEServer) []ICEServer {
	cloned := make([]ICEServer, 0, len(servers))
	for _, server := range servers {
		copyServer := server
		copyServer.URLs = append([]string(nil), server.URLs...)
		cloned = append(cloned, copyServer)
	}
	return cloned
}
