package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/pion/webrtc/v4"
)

type screenVideoTrack struct {
	Codec   string
	Track   webrtc.TrackLocal
	Network *h264NetworkMonitor
	Stream  func(context.Context, <-chan struct{}, string, *h264NetworkMonitor, func(map[string]any), func(string), func())
}

func newPreferredScreenVideoTrack(pc *webrtc.PeerConnection, sessionID, offerSDP string, features runtimeFeatures) (*screenVideoTrack, error) {
	if !features.WebRTC {
		return nil, fmt.Errorf("webrtc disabled by runtime policy")
	}
	codecs := offeredScreenVideoCodecs(offerSDP, features)
	if len(codecs) == 0 {
		codecs = features.PreferredVideoCodecs
	}
	if len(codecs) == 0 {
		codecs = []string{"h264", "h265", "av1"}
	}
	var lastErr error
	for _, codec := range codecs {
		switch strings.ToLower(strings.TrimSpace(codec)) {
		case "h265":
			if !features.H265 || !sdpOffersH265Codec(offerSDP) {
				continue
			}
			track, network, err := newH265ScreenTrack(pc, sessionID)
			if err != nil {
				lastErr = err
				fmt.Printf("H265 screen track disabled for %s: %v\n", sessionID, err)
				continue
			}
			return &screenVideoTrack{
				Codec:   "h265",
				Track:   track,
				Network: network,
				Stream: func(ctx context.Context, done <-chan struct{}, sessionID string, networkMonitor *h264NetworkMonitor, emitStatus func(map[string]any), emitError func(string), markFirstSample func()) {
					streamScreenH265ToTrack(ctx, done, sessionID, track, networkMonitor, emitStatus, emitError, markFirstSample)
				},
			}, nil
		case "av1":
			if !features.AV1 || !sdpOffersVideoCodec(offerSDP, "AV1") {
				continue
			}
			track, network, err := newAV1ScreenTrack(pc, sessionID)
			if err != nil {
				lastErr = err
				fmt.Printf("AV1 screen track disabled for %s: %v\n", sessionID, err)
				continue
			}
			return &screenVideoTrack{
				Codec:   "av1",
				Track:   track,
				Network: network,
				Stream: func(ctx context.Context, done <-chan struct{}, sessionID string, networkMonitor *h264NetworkMonitor, emitStatus func(map[string]any), emitError func(string), markFirstSample func()) {
					streamScreenAV1ToTrack(ctx, done, sessionID, track, networkMonitor, emitStatus, emitError, markFirstSample)
				},
			}, nil
		case "h264":
			if !features.H264 || !sdpOffersVideoCodec(offerSDP, "H264") {
				continue
			}
			track, network, err := newH264ScreenTrack(pc, sessionID)
			if err != nil {
				lastErr = err
				fmt.Printf("H264 screen track disabled for %s: %v\n", sessionID, err)
				continue
			}
			return &screenVideoTrack{
				Codec:   "h264",
				Track:   track,
				Network: network,
				Stream: func(ctx context.Context, done <-chan struct{}, sessionID string, networkMonitor *h264NetworkMonitor, emitStatus func(map[string]any), emitError func(string), markFirstSample func()) {
					streamScreenH264ToTrack(ctx, done, sessionID, track, networkMonitor, emitStatus, emitError, markFirstSample)
				},
			}, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no compatible realtime video codec offered")
}

func offeredScreenVideoCodecs(sdp string, features runtimeFeatures) []string {
	allowed := map[string]bool{}
	for _, raw := range features.PreferredVideoCodecs {
		codec := normalizeScreenVideoCodecName(raw)
		if codec == "" {
			continue
		}
		allowed[codec] = true
	}
	if len(allowed) == 0 {
		allowed["h264"] = features.H264
		allowed["h265"] = features.H265
		allowed["av1"] = features.AV1
	}

	lines := strings.Split(strings.ReplaceAll(sdp, "\r\n", "\n"), "\n")
	payloadOrder := []string{}
	payloadCodecs := map[string]string{}
	inVideo := false
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "m=") {
			inVideo = strings.HasPrefix(strings.ToLower(line), "m=video")
			if inVideo {
				fields := strings.Fields(line)
				if len(fields) > 3 {
					payloadOrder = append(payloadOrder, fields[3:]...)
				}
			}
			continue
		}
		if !inVideo || !strings.HasPrefix(strings.ToLower(line), "a=rtpmap:") {
			continue
		}
		rtpmap := strings.TrimPrefix(line, "a=rtpmap:")
		parts := strings.Fields(rtpmap)
		if len(parts) < 2 {
			continue
		}
		payloadType := strings.TrimSpace(parts[0])
		codec := normalizeScreenVideoCodecName(strings.Split(parts[1], "/")[0])
		if codec != "" {
			payloadCodecs[payloadType] = codec
		}
	}

	codecs := make([]string, 0, 3)
	seen := map[string]struct{}{}
	for _, payloadType := range payloadOrder {
		codec := payloadCodecs[payloadType]
		if codec == "" || !screenVideoCodecEnabled(codec, features) || !allowed[codec] {
			continue
		}
		if _, exists := seen[codec]; exists {
			continue
		}
		seen[codec] = struct{}{}
		codecs = append(codecs, codec)
	}
	return codecs
}

func normalizeScreenVideoCodecName(raw string) string {
	codec := strings.ToLower(strings.TrimSpace(raw))
	codec = strings.TrimPrefix(codec, "video/")
	switch codec {
	case "h264", "h.264":
		return "h264"
	case "h265", "h.265", "hevc":
		return "h265"
	case "av1":
		return "av1"
	default:
		return ""
	}
}

func screenVideoCodecEnabled(codec string, features runtimeFeatures) bool {
	switch codec {
	case "h264":
		return features.H264
	case "h265":
		return features.H265
	case "av1":
		return features.AV1
	default:
		return false
	}
}

func sdpOffersVideoCodec(sdp, codec string) bool {
	return strings.Contains(strings.ToLower(sdp), strings.ToLower(codec))
}

func sdpOffersH265Codec(sdp string) bool {
	return sdpOffersVideoCodec(sdp, "H265") || sdpOffersVideoCodec(sdp, "HEVC")
}
