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
	codecs := features.PreferredVideoCodecs
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

func sdpOffersVideoCodec(sdp, codec string) bool {
	return strings.Contains(strings.ToLower(sdp), strings.ToLower(codec))
}

func sdpOffersH265Codec(sdp string) bool {
	return sdpOffersVideoCodec(sdp, "H265") || sdpOffersVideoCodec(sdp, "HEVC")
}
