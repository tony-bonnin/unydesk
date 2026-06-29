package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type iceServerConfig struct {
	URLs           []string `json:"urls"`
	Username       string   `json:"username,omitempty"`
	Credential     string   `json:"credential,omitempty"`
	CredentialType string   `json:"credentialType,omitempty"`
}

type runtimeInfoResponse struct {
	ICEServers []iceServerConfig  `json:"ice_servers"`
	Features   runtimeFeatureWire `json:"features"`
	WebRTC     struct {
		Enabled              *bool             `json:"enabled"`
		ICEServers           []iceServerConfig `json:"ice_servers"`
		PreferredVideoCodecs []string          `json:"preferred_video_codecs"`
	} `json:"webrtc"`
}

type runtimeConfig struct {
	ICEServers []iceServerConfig
	Features   runtimeFeatures
}

type runtimeFeatures struct {
	WebRTC               bool     `json:"webrtc"`
	H265                 bool     `json:"h265"`
	H264                 bool     `json:"h264"`
	AV1                  bool     `json:"av1"`
	QUIC                 bool     `json:"quic"`
	PreferredVideoCodecs []string `json:"preferred_video_codecs"`
}

type runtimeFeatureWire struct {
	WebRTC               *bool    `json:"webrtc"`
	H265                 *bool    `json:"h265"`
	H264                 *bool    `json:"h264"`
	AV1                  *bool    `json:"av1"`
	QUIC                 *bool    `json:"quic"`
	PreferredVideoCodecs []string `json:"preferred_video_codecs"`
}

func fetchRuntimeConfig(serverURL string) (runtimeConfig, error) {
	endpoint := strings.TrimRight(serverURL, "/") + "/api/v1/info"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return runtimeConfig{}, err
	}
	for key, values := range serverAuthHeaders(serverURL) {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return runtimeConfig{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return runtimeConfig{}, fmt.Errorf("runtime info fetch failed: %s", strings.TrimSpace(string(body)))
	}

	var info runtimeInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return runtimeConfig{}, err
	}
	servers := info.ICEServers
	if len(servers) == 0 {
		servers = info.WebRTC.ICEServers
	}
	features := normalizeRuntimeFeatures(info.Features, info.WebRTC.Enabled, info.WebRTC.PreferredVideoCodecs)
	return runtimeConfig{
		ICEServers: normalizeRuntimeICEServers(servers),
		Features:   features,
	}, nil
}

func normalizeRuntimeICEServers(servers []iceServerConfig) []iceServerConfig {
	normalized := make([]iceServerConfig, 0, len(servers))
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

func normalizeRuntimeFeatures(wire runtimeFeatureWire, webRTCEnabled *bool, preferred []string) runtimeFeatures {
	features := runtimeFeatures{
		WebRTC: true,
		H265:   true,
		H264:   true,
		AV1:    true,
		QUIC:   true,
	}
	if wire.WebRTC != nil {
		features.WebRTC = *wire.WebRTC
	} else if webRTCEnabled != nil {
		features.WebRTC = *webRTCEnabled
	}
	if wire.H265 != nil {
		features.H265 = *wire.H265
	}
	if wire.H264 != nil {
		features.H264 = *wire.H264
	}
	if wire.AV1 != nil {
		features.AV1 = *wire.AV1
	}
	if wire.QUIC != nil {
		features.QUIC = *wire.QUIC
	}
	if len(features.PreferredVideoCodecs) == 0 {
		features.PreferredVideoCodecs = wire.PreferredVideoCodecs
	}
	if len(features.PreferredVideoCodecs) == 0 {
		features.PreferredVideoCodecs = preferred
	}
	if len(features.PreferredVideoCodecs) == 0 {
		features.PreferredVideoCodecs = []string{"h264", "h265", "av1"}
	}
	features.PreferredVideoCodecs = normalizeRuntimeVideoCodecs(features.PreferredVideoCodecs, features)
	return features
}

func normalizeRuntimeVideoCodecs(codecs []string, features runtimeFeatures) []string {
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
