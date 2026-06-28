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
	screenChunkMagic    = "USDT"
	screenChunkVersion  = 1
	screenChunkHeader   = 20
	screenChunkPayload  = 32 * 1024
	screenMaxBuffered   = 8 * 1024 * 1024
	sessionPollInterval = 100 * time.Millisecond
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

func runWebRTCSession(ctx context.Context, done <-chan struct{}, sessionURL, sessionID string) {
	if err := serveWebRTCSession(ctx, done, sessionURL, sessionID); err != nil && ctx.Err() == nil {
		fmt.Printf("Host WebRTC session error for %s: %v\n", sessionID, err)
	}
}

func serveWebRTCSession(ctx context.Context, done <-chan struct{}, sessionURL, sessionID string) error {
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return err
	}
	defer pc.Close()

	channels := &sessionChannels{}
	var screenFrameID uint32
	var remoteOfferApplied atomic.Bool
	var hostAnswerPosted atomic.Bool
	var screenStarted atomic.Bool
	var h264Started atomic.Bool
	var h264Failed atomic.Bool
	var h264Mu sync.RWMutex
	var h264Track *webrtc.TrackLocalStaticSample
	var h264Network *h264NetworkMonitor
	hostCandidatesSeen := make(map[string]struct{})
	viewerCandidatesSeen := make(map[string]struct{})
	var candidateMu sync.Mutex
	connectionDone := make(chan struct{})
	connectionClosed := make(chan struct{})

	emitEvent := func(sessionID string, payload map[string]any) {
		channel := channels.eventChannel()
		if channel == nil || channel.ReadyState() != webrtc.DataChannelStateOpen {
			return
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return
		}
		if sendErr := channel.SendText(string(body)); sendErr != nil {
			fmt.Printf("Host session event %s send failed: %v\n", sessionID, sendErr)
		}
	}

	getH264Track := func() (*webrtc.TrackLocalStaticSample, *h264NetworkMonitor) {
		h264Mu.RLock()
		defer h264Mu.RUnlock()
		return h264Track, h264Network
	}

	setH264Track := func(track *webrtc.TrackLocalStaticSample, network *h264NetworkMonitor) {
		h264Mu.Lock()
		h264Track = track
		h264Network = network
		h264Mu.Unlock()
	}

	registerControlChannel := func(dc *webrtc.DataChannel) {
		label := strings.TrimSpace(dc.Label())
		channels.set(label, dc)
		dc.OnOpen(func() {
			fmt.Printf("WebRTC channel %s open for %s\n", label, sessionID)
		})
		dc.OnClose(func() {
			fmt.Printf("WebRTC channel %s closed for %s\n", label, sessionID)
		})
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			if !msg.IsString || len(msg.Data) == 0 {
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
		fmt.Printf("WebRTC ICE state for %s: %s\n", sessionID, state.String())
	})

	pc.OnICEGatheringStateChange(func(state webrtc.ICEGatheringState) {
		fmt.Printf("WebRTC ICE gathering for %s: %s\n", sessionID, state.String())
	})

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			fmt.Printf("Host ICE gathering complete for %s\n", sessionID)
			return
		}
		fmt.Printf("Host ICE candidate for %s: %s/%s\n", sessionID, candidate.Typ.String(), candidate.Protocol.String())
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

	ordered := true
	screenChannel, err := pc.CreateDataChannel("screen", &webrtc.DataChannelInit{
		Ordered: &ordered,
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
	screenChannel.OnOpen(func() {
		track, _ := getH264Track()
		if track != nil && !h264Failed.Load() {
			fmt.Printf("WebRTC screen data channel idle for %s because H264 video is primary\n", sessionID)
			return
		}
		startScreenDataChannel("H264 unavailable")
	})
	screenChannel.OnClose(func() {
		fmt.Printf("WebRTC screen channel closed for %s\n", sessionID)
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Printf("WebRTC state for %s: %s\n", sessionID, state.String())
		switch state {
		case webrtc.PeerConnectionStateConnected:
			select {
			case <-connectionDone:
			default:
				close(connectionDone)
			}
			track, network := getH264Track()
			if track != nil && h264Started.CompareAndSwap(false, true) {
				go streamScreenH264ToTrack(ctx, done, sessionID, track, network, func(payload map[string]any) {
					emitEvent(sessionID, payload)
				}, func(message string) {
					h264Failed.Store(true)
					emitEvent(sessionID, map[string]any{
						"type":  "screen_error",
						"error": message,
					})
					if screenChannel.ReadyState() == webrtc.DataChannelStateOpen {
						startScreenDataChannel("H264 encoder fallback")
					}
				})
			}
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateDisconnected:
			select {
			case <-connectionClosed:
			default:
				close(connectionClosed)
			}
		}
	})

	ticker := time.NewTicker(sessionPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-done:
			return nil
		case <-connectionClosed:
			return nil
		case <-ticker.C:
			session, err := fetchSessionSnapshot(sessionURL)
			if err != nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(session.Status), "closed") {
				return nil
			}
			if !remoteOfferApplied.Load() && strings.TrimSpace(session.OfferSDP) != "" {
				offer, err := parseSessionDescription(session.OfferSDP)
				if err != nil {
					return fmt.Errorf("parse offer: %w", err)
				}
				if err := pc.SetRemoteDescription(offer); err != nil {
					return fmt.Errorf("set remote description: %w", err)
				}
				if track, network, trackErr := newH264ScreenTrack(pc, sessionID); trackErr != nil {
					fmt.Printf("H264 screen track disabled for %s: %v\n", sessionID, trackErr)
				} else {
					setH264Track(track, network)
				}
				answer, err := pc.CreateAnswer(nil)
				if err != nil {
					return fmt.Errorf("create answer: %w", err)
				}
				if err := pc.SetLocalDescription(answer); err != nil {
					return fmt.Errorf("set local description: %w", err)
				}
				if err := waitForLocalICEGathering(ctx, done, pc, sessionID, 4*time.Second); err != nil {
					fmt.Printf("Host ICE gathering wait for %s: %v\n", sessionID, err)
				}
				remoteOfferApplied.Store(true)
			}
			if remoteOfferApplied.Load() && !hostAnswerPosted.Load() && pc.LocalDescription() != nil {
				if err := postSessionDescription(sessionURL+"/answer", *pc.LocalDescription()); err != nil {
					return fmt.Errorf("post answer: %w", err)
				}
				hostAnswerPosted.Store(true)
			}
			if remoteOfferApplied.Load() {
				if err := addPendingRemoteCandidates(pc, session.ViewerICECandidates, viewerCandidatesSeen, &candidateMu); err != nil {
					fmt.Printf("Host viewer candidate add failed for %s: %v\n", sessionID, err)
				}
			}
		}
	}
}

func streamScreenFramesToDataChannel(ctx context.Context, done <-chan struct{}, sessionURL, sessionID string, dc *webrtc.DataChannel, frameID *uint32, emitCursor func(string), emitError func(string)) {
	pipeline := newScreenPipeline(defaultScreenPipelineProfile())
	lastCursorKind := ""
	lastCongestionLogAt := time.Time{}
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
		return sendChunkedBinary(ctx, done, dc, atomic.AddUint32(frameID, 1), payload)
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

func addPendingRemoteCandidates(pc *webrtc.PeerConnection, rawCandidates []string, seen map[string]struct{}, mu *sync.Mutex) error {
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
	}
	return nil
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
