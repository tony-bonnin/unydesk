package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const Version = "0.1.0-alpha"

type Settings struct {
	Name       string        `yaml:"name"`
	ListenAddr string        `yaml:"listen_addr"`
	Security   Security      `yaml:"security"`
	Remote     RemoteRuntime `yaml:"remote"`
	Paths      Paths         `yaml:"paths"`
}

type Security struct {
	AllowOrigin string `yaml:"allow_origin"`
}

type RemoteRuntime struct {
	SessionTTLMinutes    int `yaml:"session_ttl_minutes"`
	HostHeartbeatSeconds int `yaml:"host_heartbeat_seconds"`
}

type Paths struct {
	PublicIDFile     string `yaml:"public_id_file"`
	HostDownloadsDir string `yaml:"host_downloads_dir"`
	FrontendDir      string `yaml:"frontend_dir"`
	UsersFile        string `yaml:"users_file"`
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
	if assetsDir := os.Getenv("UNYDESK_ASSETS"); assetsDir != "" {
		cfg.Paths.FrontendDir = assetsDir
	}
}
