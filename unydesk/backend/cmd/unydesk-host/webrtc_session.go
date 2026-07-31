package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v4"
)

const (
	screenChunkMagic               = "USDT"
	screenChunkVersion             = 1
	screenChunkHeader              = 20
	screenChunkPayload             = 16 * 1024
	screenMaxBuffered              = 512 * 1024
	sessionOfferPollInterval       = 100 * time.Millisecond
	sessionSignalingPollInterval   = 300 * time.Millisecond
	sessionEstablishedPollInterval = 2 * time.Second
)

type sessionChannels struct {
	mu     sync.RWMutex
	input  *webrtc.DataChannel
	aux    *webrtc.DataChannel
	screen *webrtc.DataChannel
}

func (c *sessionChannels) set(label string, dc *webrtc.DataChannel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch label {
	case "input":
		c.input = dc
	case "aux":
		c.aux = dc
	case "screen":
		c.screen = dc
	}
}

func (c *sessionChannels) eventChannel() *webrtc.DataChannel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.aux != nil {
		return c.aux
	}
	return c.input
}

func (c *sessionChannels) anyOpenEventChannel() *webrtc.DataChannel {
	channel := c.eventChannel()
	if channel != nil && channel.ReadyState() == webrtc.DataChannelStateOpen {
		return channel
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.screen != nil && c.screen.ReadyState() == webrtc.DataChannelStateOpen {
		return c.screen
	}
	return nil
}

func runWebRTCSession(ctx context.Context, done <-chan struct{}, serverURL, hostID, sessionURL, sessionID string, runtimeCfg runtimeConfig, emitServerEvent func(string, map[string]any), registerFallbackHandler func(func(string))) {
	if err := serveWebRTCSession(ctx, done, serverURL, hostID, sessionURL, sessionID, runtimeCfg, emitServerEvent, registerFallbackHandler); err != nil && ctx.Err() == nil {
		fmt.Printf("Host WebRTC session error for %s: %v\n", sessionID, err)
	}
}

func serveWebRTCSession(ctx context.Context, done <-chan struct{}, serverURL, hostID, sessionURL, sessionID string, runtimeCfg runtimeConfig, emitServerEvent func(string, map[string]any), registerFallbackHandler func(func(string))) error {
	sessionStartedAt := time.Now()
	peerICEServers := toPeerICEServers(runtimeCfg.ICEServers)
	fmt.Printf("WebRTC ICE servers for %s: %d configured\n", sessionID, len(peerICEServers))
	pc, err := newWebRTCPeerConnection(runtimeCfg, webrtc.Configuration{ICEServers: peerICEServers})
	if err != nil {
		return err
	}
	defer pc.Close()

	channels := &sessionChannels{}
	var screenFrameID uint32
	var remoteOfferApplied atomic.Bool
	var hostAnswerPosted atomic.Bool
	var screenStarted atomic.Bool
	var videoStarted atomic.Bool
	var videoFailed atomic.Bool
	var videoFirstSample atomic.Bool
	var videoMu sync.RWMutex
	var videoTrack *screenVideoTrack
	var videoCancelMu sync.Mutex
	var activeVideoCancel context.CancelFunc
	hostCandidatesSeen := make(map[string]struct{})
	viewerCandidatesSeen := make(map[string]struct{})
	var candidateMu sync.Mutex
	var pendingEventMu sync.Mutex
	pendingEvents := make([]string, 0, 8)
	connectionDone := make(chan struct{})
	connectionClosed := make(chan struct{})
	var requestScreenFallback func(string)
	var startRealtimeVideo func(string)
	var videoStartBlockedLogged atomic.Bool

	videoStartBlockStatus := func(video *screenVideoTrack) (string, string, bool, bool, bool) {
		transport := "video"
		videoPresent := video != nil
		trackPresent := false
		streamPresent := false
		reason := "video-track-not-ready"
		if videoFailed.Load() {
			reason = "video-marked-failed"
		}
		if video != nil {
			if strings.TrimSpace(video.Codec) != "" {
				transport = video.Codec
			}
			trackPresent = video.Track != nil
			streamPresent = video.Stream != nil
			switch {
			case !trackPresent:
				reason = "track-nil"
			case !streamPresent:
				reason = "stream-nil"
			case videoFailed.Load():
				reason = "video-marked-failed"
			}
		}
		return reason, transport, videoPresent, trackPresent, streamPresent
	}

	flushPendingEvents := func() {
		channel := channels.anyOpenEventChannel()
		if channel == nil {
			return
		}
		pendingEventMu.Lock()
		events := append([]string(nil), pendingEvents...)
		pendingEvents = pendingEvents[:0]
		pendingEventMu.Unlock()
		for _, body := range events {
			if sendErr := channel.SendText(body); sendErr != nil {
				fmt.Printf("Host queued session event %s send failed: %v\n", sessionID, sendErr)
				pendingEventMu.Lock()
				pendingEvents = append([]string{body}, pendingEvents...)
				pendingEventMu.Unlock()
				return
			}
		}
	}

	emitEvent := func(sessionID string, payload map[string]any) {
		body, err := json.Marshal(payload)
		if err != nil {
			return
		}
		channel := channels.anyOpenEventChannel()
		if channel == nil {
			if isRealtimeDiagnosticPayload(payload) && emitServerEvent != nil {
				emitServerEvent(sessionID, payload)
				return
			}
			pendingEventMu.Lock()
			if len(pendingEvents) >= 32 {
				pendingEvents = pendingEvents[1:]
			}
			pendingEvents = append(pendingEvents, string(body))
			pendingEventMu.Unlock()
			return
		}
		if sendErr := channel.SendText(string(body)); sendErr != nil {
			fmt.Printf("Host session event %s send failed: %v\n", sessionID, sendErr)
			if isRealtimeDiagnosticPayload(payload) && emitServerEvent != nil {
				emitServerEvent(sessionID, payload)
			}
		}
	}

	getVideoTrack := func() *screenVideoTrack {
		videoMu.RLock()
		defer videoMu.RUnlock()
		return videoTrack
	}

	setVideoTrack := func(track *screenVideoTrack) {
		videoMu.Lock()
		videoTrack = track
		videoMu.Unlock()
	}

	setActiveVideoCancel := func(cancel context.CancelFunc) {
		videoCancelMu.Lock()
		activeVideoCancel = cancel
		videoCancelMu.Unlock()
	}

	cancelActiveVideo := func() {
		videoCancelMu.Lock()
		cancel := activeVideoCancel
		activeVideoCancel = nil
		videoCancelMu.Unlock()
		if cancel != nil {
			cancel()
		}
	}
	defer cancelActiveVideo()

	registerControlChannel := func(dc *webrtc.DataChannel) {
		label := strings.TrimSpace(dc.Label())
		channels.set(label, dc)
		dc.OnOpen(func() {
			fmt.Printf("WebRTC channel %s open for %s after %s\n", label, sessionID, time.Since(sessionStartedAt).Round(time.Millisecond))
			flushPendingEvents()
		})
		dc.OnClose(func() {
			fmt.Printf("WebRTC channel %s closed for %s after %s\n", label, sessionID, time.Since(sessionStartedAt).Round(time.Millisecond))
		})
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			if !msg.IsString || len(msg.Data) == 0 {
				return
			}
			if reason, ok := parseScreenFallbackRequest(msg.Data); ok {
				if requestScreenFallback != nil {
					requestScreenFallback(reason)
				}
				return
			}
			handleHostControlMessage(sessionID, json.RawMessage(msg.Data), emitEvent)
		})
	}

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		switch strings.TrimSpace(dc.Label()) {
		case "input", "aux":
			registerControlChannel(dc)
		default:
			fmt.Printf("Host WebRTC session %s ignored channel %s\n", sessionID, dc.Label())
		}
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		fmt.Printf("WebRTC ICE state for %s: %s after %s\n", sessionID, state.String(), time.Since(sessionStartedAt).Round(time.Millisecond))
		switch state {
		case webrtc.ICEConnectionStateConnected, webrtc.ICEConnectionStateCompleted:
			if !videoStarted.Load() && !videoFailed.Load() {
				startRealtimeVideo("ice-connected")
			}
		}
	})

	pc.OnICEGatheringStateChange(func(state webrtc.ICEGatheringState) {
		fmt.Printf("WebRTC ICE gathering for %s: %s after %s\n", sessionID, state.String(), time.Since(sessionStartedAt).Round(time.Millisecond))
	})

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			fmt.Printf("Host ICE gathering complete for %s after %s\n", sessionID, time.Since(sessionStartedAt).Round(time.Millisecond))
			return
		}
		fmt.Printf("Host ICE candidate for %s: %s/%s addr=%s port=%d after %s\n", sessionID, candidate.Typ.String(), candidate.Protocol.String(), candidate.Address, candidate.Port, time.Since(sessionStartedAt).Round(time.Millisecond))
		candidateJSON, err := json.Marshal(candidate.ToJSON())
		if err != nil {
			return
		}
		key := string(candidateJSON)
		candidateMu.Lock()
		if _, exists := hostCandidatesSeen[key]; exists {
			candidateMu.Unlock()
			return
		}
		hostCandidatesSeen[key] = struct{}{}
		candidateMu.Unlock()
		if err := postJSON(sessionURL+"/candidates", map[string]string{
			"candidate": key,
			"source":    "host",
		}); err != nil {
			fmt.Printf("Host ICE candidate post failed for %s: %v\n", sessionID, err)
		}
	})

	var startScreenDataChannel func(reason string)
	var startScreenWebSocketFallback func(reason string)
	var screenChannel *webrtc.DataChannel
	startRealtimeVideo = func(trigger string) {
		video := getVideoTrack()
		if video == nil || video.Track == nil || video.Stream == nil || videoFailed.Load() {
			if videoStartBlockedLogged.CompareAndSwap(false, true) {
				reason, transport, videoPresent, trackPresent, streamPresent := videoStartBlockStatus(video)
				fmt.Printf("Realtime video start blocked for %s: trigger=%s reason=%s\n", sessionID, trigger, reason)
				emitEvent(sessionID, map[string]any{
					"type":           "screen_status",
					"transport":      transport,
					"action":         "rtp-start-blocked",
					"trigger":        trigger,
					"error":          reason,
					"video_present":  videoPresent,
					"track_present":  trackPresent,
					"stream_present": streamPresent,
				})
			}
			return
		}
		if !hostAnswerPosted.Load() {
			if videoStartBlockedLogged.CompareAndSwap(false, true) {
				fmt.Printf("Realtime video start blocked for %s: trigger=%s reason=answer-not-posted\n", sessionID, trigger)
				emitEvent(sessionID, map[string]any{
					"type":      "screen_status",
					"transport": video.Codec,
					"action":    "rtp-start-blocked",
					"trigger":   trigger,
					"error":     "answer-not-posted",
				})
			}
			return
		}
		switch pc.ConnectionState() {
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateDisconnected:
			if videoStartBlockedLogged.CompareAndSwap(false, true) {
				fmt.Printf("Realtime video start blocked for %s: trigger=%s reason=peer-closed\n", sessionID, trigger)
				emitEvent(sessionID, map[string]any{
					"type":      "screen_status",
					"transport": video.Codec,
					"action":    "rtp-start-blocked",
					"trigger":   trigger,
					"error":     "peer-closed",
				})
			}
			return
		}
		if !videoStarted.CompareAndSwap(false, true) {
			return
		}

		videoCtx, cancelVideo := context.WithCancel(ctx)
		setActiveVideoCancel(cancelVideo)
		if trigger == "" {
			trigger = "unknown"
		}
		fmt.Printf("%s realtime video start armed for %s: trigger=%s after %s\n", strings.ToUpper(video.Codec), sessionID, trigger, time.Since(sessionStartedAt).Round(time.Millisecond))
		emitEvent(sessionID, map[string]any{
			"type":      "screen_status",
			"transport": video.Codec,
			"action":    "rtp-starting",
			"trigger":   trigger,
		})
		go monitorHostPeerStats(videoCtx, done, pc, sessionID, video.Codec, emitEvent)
		go func() {
			timer := time.NewTimer(h264StartupTimeout())
			defer timer.Stop()
			select {
			case <-timer.C:
				if videoFirstSample.Load() {
					return
				}
				videoFailed.Store(true)
				cancelVideo()
				codec := strings.ToUpper(video.Codec)
				message := codec + " startup timeout; using peer frame fallback."
				fmt.Printf("%s startup watchdog for %s: no encoded frame before timeout\n", codec, sessionID)
				emitEvent(sessionID, map[string]any{
					"type":  "screen_error",
					"error": message,
				})
				if screenChannel != nil && screenChannel.ReadyState() == webrtc.DataChannelStateOpen {
					startScreenDataChannel(codec + " startup timeout")
				}
			case <-done:
			case <-videoCtx.Done():
			}
		}()
		go func() {
			select {
			case <-time.After(120 * time.Millisecond):
			case <-done:
				return
			case <-videoCtx.Done():
				return
			}
			fmt.Printf("%s realtime video stream starting for %s after %s\n", strings.ToUpper(video.Codec), sessionID, time.Since(sessionStartedAt).Round(time.Millisecond))
			video.Stream(videoCtx, done, sessionID, video.Network, func(payload map[string]any) {
				emitEvent(sessionID, payload)
			}, func(message string) {
				videoFailed.Store(true)
				cancelActiveVideo()
				emitEvent(sessionID, map[string]any{
					"type":  "screen_error",
					"error": message,
				})
				if screenChannel != nil && screenChannel.ReadyState() == webrtc.DataChannelStateOpen {
					startScreenDataChannel(strings.ToUpper(video.Codec) + " encoder fallback")
				}
			}, func() {
				videoFirstSample.Store(true)
			})
			if videoCtx.Err() == nil && !videoFirstSample.Load() {
				message := strings.ToUpper(video.Codec) + " video stream ended before first encoded frame."
				videoFailed.Store(true)
				emitEvent(sessionID, map[string]any{
					"type":  "screen_error",
					"error": message,
				})
			}
			fmt.Printf("%s realtime video stream ended for %s after %s first_sample=%t\n", strings.ToUpper(video.Codec), sessionID, time.Since(sessionStartedAt).Round(time.Millisecond), videoFirstSample.Load())
		}()
	}
	scheduleRealtimeVideoStart := func(trigger string) {
		delays := []time.Duration{
			0,
			50 * time.Millisecond,
			150 * time.Millisecond,
			350 * time.Millisecond,
			750 * time.Millisecond,
			1200 * time.Millisecond,
		}
		for _, delay := range delays {
			delay := delay
			go func() {
				if delay > 0 {
					timer := time.NewTimer(delay)
					defer timer.Stop()
					select {
					case <-timer.C:
					case <-done:
						return
					case <-ctx.Done():
						return
					}
				}
				if videoStarted.Load() || videoFailed.Load() {
					return
				}
				startRealtimeVideo(trigger)
			}()
		}
	}

	ordered := false
	maxRetransmits := uint16(0)
	screenChannel, err = pc.CreateDataChannel("screen", &webrtc.DataChannelInit{
		Ordered:        &ordered,
		MaxRetransmits: &maxRetransmits,
	})
	if err != nil {
		return err
	}
	channels.set("screen", screenChannel)
	startScreenDataChannel = func(reason string) {
		if !screenStarted.CompareAndSwap(false, true) {
			return
		}
		if reason != "" {
			fmt.Printf("WebRTC screen data channel streaming for %s: %s\n", sessionID, reason)
		}
		go streamScreenFramesToDataChannel(ctx, done, sessionURL, sessionID, screenChannel, &screenFrameID, func(cursor string) {
			emitEvent(sessionID, map[string]any{
				"type":   "cursor",
				"cursor": cursor,
			})
		}, func(message string) {
			emitEvent(sessionID, map[string]any{
				"type":  "screen_error",
				"error": message,
			})
		})
	}
	startScreenWebSocketFallback = func(reason string) {
		if !screenStarted.CompareAndSwap(false, true) {
			return
		}
		screenWSURL, err := toScreenWebSocketURL(serverURL, sessionID, hostID)
		if err != nil {
			fmt.Printf("WebRTC screen websocket fallback unavailable for %s: %v\n", sessionID, err)
			emitEvent(sessionID, map[string]any{
				"type":      "screen_status",
				"transport": "peer-frame",
				"action":    "fallback-unavailable",
				"reason":    err.Error(),
			})
			return
		}
		if reason != "" {
			fmt.Printf("WebRTC screen websocket fallback for %s: %s\n", sessionID, reason)
		}
		go streamScreenFrames(ctx, done, sessionURL, screenWSURL, sessionID)
	}
	requestScreenFallback = func(reason string) {
		videoFailed.Store(true)
		cancelActiveVideo()
		if reason == "" {
			reason = "viewer requested fallback"
		}
		fmt.Printf("WebRTC screen fallback requested for %s: %s\n", sessionID, reason)
		emitEvent(sessionID, map[string]any{
			"type":      "screen_status",
			"transport": "video",
			"action":    "fallback",
			"reason":    reason,
		})
		if screenChannel.ReadyState() == webrtc.DataChannelStateOpen {
			startScreenDataChannel(reason)
			return
		}
		startScreenWebSocketFallback(reason)
	}
	if registerFallbackHandler != nil {
		registerFallbackHandler(requestScreenFallback)
		defer registerFallbackHandler(nil)
	}
	screenChannel.OnOpen(func() {
		flushPendingEvents()
		video := getVideoTrack()
		if video != nil && video.Track != nil && !videoFailed.Load() {
			startRealtimeVideo("screen-channel-open")
			fmt.Printf("WebRTC screen data channel idle for %s because %s video is primary\n", sessionID, strings.ToUpper(video.Codec))
			return
		}
		if !remoteOfferApplied.Load() || !hostAnswerPosted.Load() {
			scheduleRealtimeVideoStart("screen-channel-open-waiting-track")
			fmt.Printf("WebRTC screen data channel waiting for realtime video track for %s\n", sessionID)
			return
		}
		startScreenDataChannel("realtime video unavailable")
	})
	screenChannel.OnClose(func() {
		fmt.Printf("WebRTC screen channel closed for %s after %s\n", sessionID, time.Since(sessionStartedAt).Round(time.Millisecond))
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Printf("WebRTC state for %s: %s after %s\n", sessionID, state.String(), time.Since(sessionStartedAt).Round(time.Millisecond))
		switch state {
		case webrtc.PeerConnectionStateConnected:
			select {
			case <-connectionDone:
			default:
				close(connectionDone)
			}
			startRealtimeVideo("pc-connected")
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateDisconnected:
			cancelActiveVideo()
			select {
			case <-connectionClosed:
			default:
				close(connectionClosed)
			}
		}
	})

	pollInterval := func() time.Duration {
		if !remoteOfferApplied.Load() {
			return sessionOfferPollInterval
		}
		if !hostAnswerPosted.Load() || pc.ConnectionState() != webrtc.PeerConnectionStateConnected {
			return sessionSignalingPollInterval
		}
		return sessionEstablishedPollInterval
	}
	pollTimer := time.NewTimer(sessionOfferPollInterval)
	defer pollTimer.Stop()
	lastPollErrLogAt := time.Time{}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-done:
			return nil
		case <-connectionClosed:
			return nil
		case <-pollTimer.C:
			session, err := fetchSessionSnapshot(sessionURL)
			if err != nil {
				if lastPollErrLogAt.IsZero() || time.Since(lastPollErrLogAt) > 3*time.Second {
					lastPollErrLogAt = time.Now()
					fmt.Printf("WebRTC session poll failed for %s after %s: %v\n", sessionID, time.Since(sessionStartedAt).Round(time.Millisecond), err)
				}
				pollTimer.Reset(pollInterval())
				continue
			}
			if strings.EqualFold(strings.TrimSpace(session.Status), "closed") {
				return nil
			}
			if !remoteOfferApplied.Load() && strings.TrimSpace(session.OfferSDP) != "" {
				fmt.Printf("WebRTC offer detected for %s after %s: offer=%dB viewer_candidates=%d status=%s\n", sessionID, time.Since(sessionStartedAt).Round(time.Millisecond), len(session.OfferSDP), len(session.ViewerICECandidates), strings.TrimSpace(session.Status))
				offer, err := parseSessionDescription(session.OfferSDP)
				if err != nil {
					return fmt.Errorf("parse offer: %w", err)
				}
				started := time.Now()
				if err := pc.SetRemoteDescription(offer); err != nil {
					return fmt.Errorf("set remote description: %w", err)
				}
				fmt.Printf("WebRTC remote offer applied for %s in %s\n", sessionID, time.Since(started).Round(time.Millisecond))
				if track, trackErr := newPreferredScreenVideoTrack(pc, sessionID, session.OfferSDP, runtimeCfg.Features); trackErr != nil {
					fmt.Printf("Realtime video track disabled for %s: %v\n", sessionID, trackErr)
					videoFailed.Store(true)
					emitEvent(sessionID, map[string]any{
						"type":      "screen_status",
						"transport": "video",
						"action":    "track-unavailable",
						"error":     trackErr.Error(),
					})
					emitEvent(sessionID, map[string]any{
						"type":  "screen_error",
						"error": "Realtime video track unavailable: " + trackErr.Error(),
					})
					if screenChannel != nil && screenChannel.ReadyState() == webrtc.DataChannelStateOpen {
						startScreenDataChannel("realtime video track unavailable")
					}
				} else {
					setVideoTrack(track)
					videoStartBlockedLogged.Store(false)
					fmt.Printf("Realtime video track selected for %s: codec=%s\n", sessionID, strings.ToUpper(track.Codec))
					scheduleRealtimeVideoStart("track-ready")
				}
				started = time.Now()
				answer, err := pc.CreateAnswer(nil)
				if err != nil {
					return fmt.Errorf("create answer: %w", err)
				}
				fmt.Printf("WebRTC answer created for %s in %s: answer=%dB\n", sessionID, time.Since(started).Round(time.Millisecond), len(answer.SDP))
				started = time.Now()
				if err := pc.SetLocalDescription(answer); err != nil {
					return fmt.Errorf("set local description: %w", err)
				}
				fmt.Printf("WebRTC local answer applied for %s in %s\n", sessionID, time.Since(started).Round(time.Millisecond))
				if err := waitForLocalICEGathering(ctx, done, pc, sessionID, 4*time.Second); err != nil {
					fmt.Printf("Host ICE gathering wait for %s: %v\n", sessionID, err)
				}
				remoteOfferApplied.Store(true)
			}
			if remoteOfferApplied.Load() && !hostAnswerPosted.Load() && pc.LocalDescription() != nil {
				started := time.Now()
				if err := postSessionDescription(sessionURL+"/answer", *pc.LocalDescription()); err != nil {
					return fmt.Errorf("post answer: %w", err)
				}
				candidateMu.Lock()
				hostCandidateCount := len(hostCandidatesSeen)
				candidateMu.Unlock()
				fmt.Printf("WebRTC answer posted for %s in %s: local_sdp=%dB host_candidates=%d\n", sessionID, time.Since(started).Round(time.Millisecond), len(pc.LocalDescription().SDP), hostCandidateCount)
				hostAnswerPosted.Store(true)
				if !videoStarted.Load() && !videoFailed.Load() {
					startRealtimeVideo("answer-posted")
					scheduleRealtimeVideoStart("answer-posted-retry")
				}
			}
			if remoteOfferApplied.Load() {
				if err := addPendingRemoteCandidates(sessionID, pc, session.ViewerICECandidates, viewerCandidatesSeen, &candidateMu); err != nil {
					fmt.Printf("Host viewer candidate add failed for %s: %v\n", sessionID, err)
				}
			}
			if hostAnswerPosted.Load() && !videoStarted.Load() && !videoFailed.Load() {
				if pc.ConnectionState() == webrtc.PeerConnectionStateConnected {
					startRealtimeVideo("poll-pc-connected")
				} else if screenChannel != nil && screenChannel.ReadyState() == webrtc.DataChannelStateOpen {
					startRealtimeVideo("poll-screen-channel-open")
				}
			}
			pollTimer.Reset(pollInterval())
		}
	}
}

func newWebRTCPeerConnection(runtimeCfg runtimeConfig, config webrtc.Configuration) (*webrtc.PeerConnection, error) {
	if !runtimeCfg.Features.H265 {
		return webrtc.NewPeerConnection(config)
	}
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}
	videoFeedback := []webrtc.RTCPFeedback{
		{Type: webrtc.TypeRTCPFBGoogREMB},
		{Type: webrtc.TypeRTCPFBCCM, Parameter: "fir"},
		{Type: webrtc.TypeRTCPFBNACK},
		{Type: webrtc.TypeRTCPFBNACK, Parameter: "pli"},
	}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeH265,
			ClockRate:    90000,
			RTCPFeedback: videoFeedback,
		},
		PayloadType: 116,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	return api.NewPeerConnection(config)
}

func streamScreenFramesToDataChannel(ctx context.Context, done <-chan struct{}, sessionURL, sessionID string, dc *webrtc.DataChannel, frameID *uint32, emitCursor func(string), emitError func(string)) {
	pipeline := newScreenPipeline(defaultScreenPipelineProfile())
	lastCursorKind := ""
	lastCongestionLogAt := time.Time{}
	lastStatsLogAt := time.Now()
	statsFrames := 0
	statsBytes := 0
	statsSendDuration := time.Duration(0)
	frameTicker := time.NewTicker(pipeline.profile.FrameInterval)
	defer frameTicker.Stop()
	statusTicker := time.NewTicker(2 * time.Second)
	defer statusTicker.Stop()

	sendCaptureError := func(captureErr error) {
		friendly, shouldReport := pipeline.captureErrorMessage(captureErr)
		if !shouldReport {
			return
		}
		fmt.Printf("[%s] screen error: %v\n", time.Now().Format("15:04:05"), captureErr)
		emitError(friendly)
	}

	sendCursorUpdate := func(force bool) {
		nextCursorKind := normalizeRemoteCursorKind(detectHostCursorKind())
		if !force && nextCursorKind == lastCursorKind {
			return
		}
		lastCursorKind = nextCursorKind
		emitCursor(nextCursorKind)
	}

	sendFrame := func(frame screenFrame) error {
		payload := encodeScreenWireFrame(frame)
		started := time.Now()
		if err := sendChunkedBinary(ctx, done, dc, atomic.AddUint32(frameID, 1), payload); err != nil {
			return err
		}
		statsFrames++
		statsBytes += len(payload)
		statsSendDuration += time.Since(started)
		if time.Since(lastStatsLogAt) >= 2*time.Second {
			elapsed := time.Since(lastStatsLogAt)
			fps := float64(statsFrames) / elapsed.Seconds()
			kbps := float64(statsBytes*8) / 1000 / elapsed.Seconds()
			avgSendMs := float64(statsSendDuration.Microseconds()) / 1000 / float64(maxInt(1, statsFrames))
			fmt.Printf("Screen data channel metrics for %s: frames=%d fps=%.1f bitrate=%.0fkbps avg_send=%.1fms buffered=%d bytes\n", sessionID, statsFrames, fps, kbps, avgSendMs, dc.BufferedAmount())
			lastStatsLogAt = time.Now()
			statsFrames = 0
			statsBytes = 0
			statsSendDuration = 0
		}
		return nil
	}

	sendCursorUpdate(true)

	initialFrame, changed, err := pipeline.nextFrame()
	if err != nil {
		sendCaptureError(err)
	} else if changed {
		if err := sendFrame(initialFrame); err != nil {
			fmt.Printf("Initial screen frame send failed for %s: %v\n", sessionID, err)
			return
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-statusTicker.C:
			snapshot, statusErr := fetchSessionSnapshot(sessionURL)
			if statusErr == nil && strings.EqualFold(strings.TrimSpace(snapshot.Status), "closed") {
				return
			}
		case <-frameTicker.C:
			if dc.ReadyState() != webrtc.DataChannelStateOpen {
				return
			}
			if dc.BufferedAmount() > screenMaxBuffered {
				if lastCongestionLogAt.IsZero() || time.Since(lastCongestionLogAt) > 2*time.Second {
					lastCongestionLogAt = time.Now()
					fmt.Printf("[%s] screen congestion for %s: buffered=%d bytes\n", time.Now().Format("15:04:05"), sessionID, dc.BufferedAmount())
				}
				// Skip this tick instead of queuing stale frames and increasing latency.
				continue
			}
			sendCursorUpdate(false)
			frame, changed, captureErr := pipeline.nextFrame()
			if captureErr != nil {
				sendCaptureError(captureErr)
				time.Sleep(pipeline.profile.CaptureFailureBackoff)
				continue
			}
			if !changed {
				continue
			}
			if err := sendFrame(frame); err != nil {
				fmt.Printf("Screen frame send failed for %s: %v\n", sessionID, err)
				return
			}
		}
	}
}

func sendChunkedBinary(ctx context.Context, done <-chan struct{}, dc *webrtc.DataChannel, frameID uint32, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	totalChunks := (len(payload) + screenChunkPayload - 1) / screenChunkPayload
	for chunkIndex := 0; chunkIndex < totalChunks; chunkIndex++ {
		if err := waitForScreenBuffer(ctx, done, dc); err != nil {
			return err
		}
		start := chunkIndex * screenChunkPayload
		end := start + screenChunkPayload
		if end > len(payload) {
			end = len(payload)
		}
		packet := make([]byte, screenChunkHeader+(end-start))
		copy(packet[0:4], []byte(screenChunkMagic))
		packet[4] = screenChunkVersion
		packet[5] = 0
		binary.BigEndian.PutUint32(packet[8:12], frameID)
		binary.BigEndian.PutUint32(packet[12:16], uint32(chunkIndex))
		binary.BigEndian.PutUint32(packet[16:20], uint32(totalChunks))
		copy(packet[screenChunkHeader:], payload[start:end])
		if err := dc.Send(packet); err != nil {
			return err
		}
	}
	return nil
}

func toPeerICEServers(servers []iceServerConfig) []webrtc.ICEServer {
	peerServers := make([]webrtc.ICEServer, 0, len(servers))
	for _, server := range servers {
		urls := append([]string(nil), server.URLs...)
		if requiresTURNCredentials(urls) && (server.Username == "" || server.Credential == "") {
			urls = filterNonTURNURLs(urls)
			if len(urls) == 0 {
				fmt.Println("Skipping TURN ICE server without credentials")
				continue
			}
		}
		peerServer := webrtc.ICEServer{URLs: urls}
		if server.Username != "" || server.Credential != "" {
			peerServer.Username = server.Username
			peerServer.Credential = server.Credential
			peerServer.CredentialType = webrtc.ICECredentialTypePassword
		}
		peerServers = append(peerServers, peerServer)
	}
	return peerServers
}

func requiresTURNCredentials(urls []string) bool {
	for _, rawURL := range urls {
		if isTURNURL(rawURL) {
			return true
		}
	}
	return false
}

func filterNonTURNURLs(urls []string) []string {
	filtered := make([]string, 0, len(urls))
	for _, rawURL := range urls {
		if !isTURNURL(rawURL) {
			filtered = append(filtered, rawURL)
		}
	}
	return filtered
}

func isTURNURL(rawURL string) bool {
	value := strings.ToLower(strings.TrimSpace(rawURL))
	return strings.HasPrefix(value, "turn:") || strings.HasPrefix(value, "turns:")
}

func waitForScreenBuffer(ctx context.Context, done <-chan struct{}, dc *webrtc.DataChannel) error {
	for dc.BufferedAmount() > screenMaxBuffered {
		if dc.ReadyState() != webrtc.DataChannelStateOpen {
			return fmt.Errorf("screen data channel closed")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return fmt.Errorf("screen session stopped")
		case <-time.After(5 * time.Millisecond):
		}
	}
	return nil
}

func waitForLocalICEGathering(ctx context.Context, done <-chan struct{}, pc *webrtc.PeerConnection, sessionID string, timeout time.Duration) error {
	if pc.ICEGatheringState() == webrtc.ICEGatheringStateComplete {
		return nil
	}
	complete := webrtc.GatheringCompletePromise(pc)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-complete:
		fmt.Printf("Host ICE gathering completed before answer for %s\n", sessionID)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return fmt.Errorf("session stopped")
	case <-timer.C:
		return fmt.Errorf("timeout after %s", timeout)
	}
}

func monitorHostPeerStats(ctx context.Context, done <-chan struct{}, pc *webrtc.PeerConnection, sessionID, codec string, emitEvent func(string, map[string]any)) {
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()
	var previousPackets uint32
	var previousBytes uint64
	var previousAt time.Time
	emitStats := func(phase string) {
		packets, bytesSent, discardedPackets, discardedBytes := hostOutboundVideoTotals(pc.GetStats())
		pairPackets, pairBytes, pairState := hostSelectedPairTotals(pc.GetStats())
		now := time.Now()
		deltaPackets := uint32(0)
		deltaBytes := uint64(0)
		kbps := 0.0
		if !previousAt.IsZero() {
			if packets >= previousPackets {
				deltaPackets = packets - previousPackets
			}
			if bytesSent >= previousBytes {
				deltaBytes = bytesSent - previousBytes
			}
			elapsed := now.Sub(previousAt).Seconds()
			if elapsed > 0 {
				kbps = float64(deltaBytes*8) / 1000 / elapsed
			}
		}
		previousPackets = packets
		previousBytes = bytesSent
		previousAt = now

		fmt.Printf("Host RTP stats for %s: codec=%s phase=%s outbound_packets=%d delta_packets=%d outbound_bytes=%d bitrate=%.0fkbps discarded_packets=%d discarded_bytes=%d pair_packets=%d pair_bytes=%d pair_state=%s\n",
			sessionID,
			strings.ToUpper(codec),
			phase,
			packets,
			deltaPackets,
			bytesSent,
			kbps,
			discardedPackets,
			discardedBytes,
			pairPackets,
			pairBytes,
			pairState,
		)
		if emitEvent != nil {
			emitEvent(sessionID, map[string]any{
				"type":                  "screen_status",
				"transport":             codec,
				"action":                "host-rtp",
				"phase":                 phase,
				"rtp_packets":           packets,
				"rtp_delta_packets":     deltaPackets,
				"rtp_bytes":             bytesSent,
				"rtp_kbps":              kbps,
				"rtp_discarded_packets": discardedPackets,
				"rtp_discarded_bytes":   discardedBytes,
				"pair_packets":          pairPackets,
				"pair_bytes":            pairBytes,
				"pair_state":            pairState,
			})
		}
	}

	emitStats("start")

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			emitStats("tick")
		}
	}
}

func hostOutboundVideoTotals(report webrtc.StatsReport) (packets uint32, bytesSent uint64, discardedPackets uint32, discardedBytes uint64) {
	for _, stat := range report {
		outbound, ok := stat.(webrtc.OutboundRTPStreamStats)
		if !ok || !strings.EqualFold(outbound.Kind, "video") {
			continue
		}
		packets += outbound.PacketsSent
		bytesSent += outbound.BytesSent
		discardedPackets += outbound.PacketsDiscardedOnSend
		discardedBytes += outbound.BytesDiscardedOnSend
	}
	return packets, bytesSent, discardedPackets, discardedBytes
}

func hostSelectedPairTotals(report webrtc.StatsReport) (packets uint32, bytesSent uint64, state string) {
	state = "unknown"
	for _, stat := range report {
		pair, ok := stat.(webrtc.ICECandidatePairStats)
		if !ok || !pair.Nominated {
			continue
		}
		return pair.PacketsSent, pair.BytesSent, string(pair.State)
	}
	return 0, 0, state
}

func addPendingRemoteCandidates(sessionID string, pc *webrtc.PeerConnection, rawCandidates []string, seen map[string]struct{}, mu *sync.Mutex) error {
	for _, raw := range rawCandidates {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			continue
		}
		mu.Lock()
		if _, exists := seen[candidate]; exists {
			mu.Unlock()
			continue
		}
		seen[candidate] = struct{}{}
		mu.Unlock()

		var init webrtc.ICECandidateInit
		if err := json.Unmarshal([]byte(candidate), &init); err != nil {
			return err
		}
		if err := pc.AddICECandidate(init); err != nil {
			return err
		}
		fmt.Printf("Host added viewer ICE candidate for %s: %s\n", sessionID, describeICECandidateInit(init))
	}
	return nil
}

func describeICECandidateInit(candidate webrtc.ICECandidateInit) string {
	raw := strings.ToLower(strings.TrimSpace(candidate.Candidate))
	candidateType := "unknown"
	protocol := "unknown"
	parts := strings.Fields(raw)
	for index, part := range parts {
		if part == "typ" && index+1 < len(parts) {
			candidateType = parts[index+1]
		}
		if part == "udp" || part == "tcp" {
			protocol = part
		}
	}
	return candidateType + "/" + protocol
}

func isRealtimeDiagnosticPayload(payload map[string]any) bool {
	switch strings.TrimSpace(fmt.Sprint(payload["type"])) {
	case "screen_status", "screen_error":
		return true
	default:
		return false
	}
}

func parseScreenFallbackRequest(raw []byte) (string, bool) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", false
	}
	if strings.TrimSpace(fmt.Sprint(payload["type"])) != "screen_fallback_request" {
		return "", false
	}
	reason := strings.TrimSpace(fmt.Sprint(payload["reason"]))
	if reason == "" || reason == "<nil>" {
		reason = "viewer requested fallback"
	}
	return reason, true
}

func parseSessionDescription(raw string) (webrtc.SessionDescription, error) {
	var desc webrtc.SessionDescription
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &desc); err != nil {
		return webrtc.SessionDescription{}, err
	}
	return desc, nil
}

func postSessionDescription(endpoint string, desc webrtc.SessionDescription) error {
	body, err := json.Marshal(desc)
	if err != nil {
		return err
	}
	return postJSON(endpoint, map[string]string{"sdp": string(body)})
}
