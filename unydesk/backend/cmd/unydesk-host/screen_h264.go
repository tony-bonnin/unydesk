package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/h264reader"
)

var (
	errH264ResolutionChanged = errors.New("screen resolution changed")
	errH264EncoderOverloaded = errors.New("h264 encoder overloaded")
	errH264NetworkCongested  = errors.New("h264 network congested")
	errH264StableUpgrade     = errors.New("h264 encoder profile stable")
)

type h264AdaptiveProfile struct {
	Name                 string
	MaxEdge              int
	FPS                  int
	CRF                  int
	Preset               string
	MaxRateKbps          int
	MaxFrameAge          time.Duration
	UpgradeAfter         time.Duration
	OverloadWindow       time.Duration
	RawDropThreshold     int
	EncodedDropThreshold int
}

func newH264ScreenTrack(pc *webrtc.PeerConnection, sessionID string) (*webrtc.TrackLocalStaticSample, *h264NetworkMonitor, error) {
	if _, ok := lookupFFmpegBinary(); !ok {
		return nil, nil, fmt.Errorf("ffmpeg encoder not found")
	}

	track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType:    webrtc.MimeTypeH264,
		ClockRate:   90000,
		SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e033",
	}, "screen-video", "unydesk-screen")
	if err != nil {
		return nil, nil, err
	}

	sender, err := pc.AddTrack(track)
	if err != nil {
		return nil, nil, err
	}

	networkMonitor := newH264NetworkMonitor()
	go func() {
		for {
			packets, _, readErr := sender.ReadRTCP()
			if readErr != nil {
				return
			}
			networkMonitor.observe(packets)
		}
	}()

	fmt.Printf("H264 screen track negotiated for %s\n", sessionID)
	return track, networkMonitor, nil
}

func streamScreenH264ToTrack(ctx context.Context, done <-chan struct{}, sessionID string, track *webrtc.TrackLocalStaticSample, networkMonitor *h264NetworkMonitor, emitStatus func(map[string]any), emitError func(string)) {
	ffmpegPath, ok := lookupFFmpegBinary()
	if !ok {
		message := "H.264 encoder unavailable: install ffmpeg.exe next to unydesk-host.exe or add FFmpeg to PATH."
		fmt.Printf("H264 screen disabled for %s: ffmpeg not found\n", sessionID)
		emitError(message)
		return
	}
	if networkMonitor == nil {
		networkMonitor = newH264NetworkMonitor()
	}

	profiles := h264AdaptiveProfiles()
	capture := defaultCaptureProvider()
	profileIndex := 0
	floorOverloads := 0
	emitProfileStatus := func(action string, profile h264AdaptiveProfile, network *h264NetworkSnapshot) {
		if emitStatus == nil {
			return
		}
		payload := map[string]any{
			"type":      "screen_status",
			"transport": "h264",
			"action":    action,
			"profile":   profile.Name,
			"max_edge":  profile.MaxEdge,
			"fps":       profile.FPS,
			"crf":       profile.CRF,
			"preset":    profile.Preset,
			"maxrate":   profile.MaxRateKbps,
			"age_ms":    profile.MaxFrameAge.Milliseconds(),
			"window_ms": profile.OverloadWindow.Milliseconds(),
		}
		for key, value := range remoteFabricPolicyFor(ffmpegPath, profile, capture).statusFields() {
			payload[key] = value
		}
		if network != nil {
			payload["loss_pct"] = network.LossPercent
			payload["jitter_ms"] = network.JitterMillis
			payload["nacks"] = network.Nacks
			payload["plis"] = network.PLIs
		}
		emitStatus(payload)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		default:
		}

		profile := profiles[profileIndex]
		pipeline := newScreenPipelineWithCapture(profile.screenPipelineProfile(), capture)
		emitProfileStatus("profile", profile, nil)
		err := runH264EncoderSession(ctx, done, sessionID, ffmpegPath, pipeline, track, networkMonitor, profile)
		if err == nil || ctx.Err() != nil {
			return
		}
		select {
		case <-done:
			return
		default:
		}
		if errors.Is(err, errH264ResolutionChanged) {
			fmt.Printf("H264 screen restarting for %s after resolution change\n", sessionID)
			time.Sleep(150 * time.Millisecond)
			continue
		}
		if errors.Is(err, errH264EncoderOverloaded) && profileIndex < len(profiles)-1 {
			nextProfile := profiles[profileIndex+1]
			fmt.Printf("H264 adaptive downshift for %s: %s -> %s after encoder backlog\n", sessionID, profile.Name, nextProfile.Name)
			profileIndex++
			floorOverloads = 0
			emitProfileStatus("downshift", nextProfile, nil)
			time.Sleep(120 * time.Millisecond)
			continue
		}
		if errors.Is(err, errH264NetworkCongested) && profileIndex < len(profiles)-1 {
			nextProfile := profiles[profileIndex+1]
			snapshot := networkMonitor.lastSnapshot()
			fmt.Printf("H264 network downshift for %s: %s -> %s after loss %.1f%%, jitter %.0fms, nacks=%d, plis=%d\n", sessionID, profile.Name, nextProfile.Name, snapshot.LossPercent, snapshot.JitterMillis, snapshot.Nacks, snapshot.PLIs)
			profileIndex++
			floorOverloads = 0
			emitProfileStatus("network-downshift", nextProfile, &snapshot)
			time.Sleep(120 * time.Millisecond)
			continue
		}
		if (errors.Is(err, errH264EncoderOverloaded) || errors.Is(err, errH264NetworkCongested)) && profileIndex == len(profiles)-1 && h264AdaptiveEnabled() {
			floorOverloads++
			if floorOverloads <= 3 {
				fmt.Printf("H264 adaptive floor retry for %s: profile=%s, overload=%d/3\n", sessionID, profile.Name, floorOverloads)
				emitProfileStatus("floor-retry", profile, nil)
				time.Sleep(250 * time.Millisecond)
				continue
			}
		}
		if errors.Is(err, errH264StableUpgrade) && profileIndex > 0 {
			nextProfile := profiles[profileIndex-1]
			fmt.Printf("H264 adaptive upshift for %s: %s -> %s after stable stream\n", sessionID, profile.Name, nextProfile.Name)
			profileIndex--
			floorOverloads = 0
			emitProfileStatus("upshift", nextProfile, nil)
			time.Sleep(120 * time.Millisecond)
			continue
		}
		message := fmt.Sprintf("H.264 screen stream stopped: %v", err)
		fmt.Printf("H264 screen stopped for %s: %v\n", sessionID, err)
		emitError(message)
		return
	}
}

func runH264EncoderSession(ctx context.Context, done <-chan struct{}, sessionID, ffmpegPath string, pipeline *screenPipeline, track *webrtc.TrackLocalStaticSample, networkMonitor *h264NetworkMonitor, profile h264AdaptiveProfile) error {
	firstFrame, err := captureH264Frame(pipeline)
	if err != nil {
		return err
	}
	bounds := firstFrame.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid screen frame size")
	}

	frameDuration := pipeline.profile.FrameInterval
	fps := h264FPSForInterval(frameDuration)
	encoderCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-done:
			cancel()
		case <-encoderCtx.Done():
		}
	}()

	args := h264FFmpegArgs(width, height, fps, profile)
	fmt.Printf("H264 encoder starting for %s: profile=%s, %dx%d @ %dfps, CRF %d, preset %s, max frame age %s\n", sessionID, profile.Name, width, height, fps, profile.CRF, profile.Preset, profile.MaxFrameAge)
	cmd := exec.CommandContext(encoderCtx, ffmpegPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	componentCount := 3
	errCh := make(chan error, 5)
	go logH264EncoderStderr(sessionID, stderr)
	go func() {
		errCh <- writeH264RawFrames(encoderCtx, done, pipeline, stdin, firstFrame, width, height, frameDuration, sessionID, profile)
	}()
	go func() {
		errCh <- writeH264Samples(stdout, track, frameDuration, sessionID, profile)
	}()
	go func() {
		errCh <- cmd.Wait()
	}()
	if networkMonitor != nil && h264NetworkAdaptiveEnabled() {
		componentCount++
		go func() {
			errCh <- waitForH264NetworkCongestion(encoderCtx, done, networkMonitor)
		}()
	}
	if profile.UpgradeAfter > 0 {
		componentCount++
		go func() {
			timer := time.NewTimer(profile.UpgradeAfter)
			defer timer.Stop()
			select {
			case <-encoderCtx.Done():
				errCh <- nil
			case <-done:
				errCh <- nil
			case <-timer.C:
				errCh <- errH264StableUpgrade
			}
		}()
	}

	var firstErr error
	for i := 0; i < componentCount; i++ {
		err := <-errCh
		if err != nil && encoderCtx.Err() == nil {
			if firstErr == nil || errors.Is(firstErr, io.EOF) || errors.Is(firstErr, context.Canceled) {
				firstErr = err
			}
		}
		cancel()
		_ = stdin.Close()
	}

	if firstErr != nil && !errors.Is(firstErr, context.Canceled) && !errors.Is(firstErr, io.EOF) {
		return firstErr
	}
	if encoderCtx.Err() != nil || errors.Is(firstErr, context.Canceled) || errors.Is(firstErr, io.EOF) {
		return nil
	}
	return nil
}

func lookupFFmpegBinary() (string, bool) {
	candidates := []string{}
	if configured := strings.TrimSpace(os.Getenv("UNYDESK_FFMPEG_PATH")); configured != "" {
		candidates = append(candidates, configured)
	}
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		if runtime.GOOS == "windows" {
			candidates = append(candidates, filepath.Join(dir, "ffmpeg.exe"))
		}
		candidates = append(candidates, filepath.Join(dir, "ffmpeg"))
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, "ffmpeg.exe")
	}
	candidates = append(candidates, "ffmpeg")

	for _, candidate := range candidates {
		if strings.ContainsRune(candidate, filepath.Separator) {
			if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
				return candidate, true
			}
			continue
		}
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, true
		}
	}
	return "", false
}

func h264FFmpegArgs(width, height, fps int, profile h264AdaptiveProfile) []string {
	gop := strconv.Itoa(maxInt(1, fps))
	x264Params := fmt.Sprintf("keyint=%s:min-keyint=%s:scenecut=0:repeat-headers=1:aud=1:bframes=0:rc-lookahead=0:sync-lookahead=0:sliced-threads=1", gop, gop)
	maxRate := fmt.Sprintf("%dk", profile.MaxRateKbps)
	bufferSize := fmt.Sprintf("%dk", maxInt(500, profile.MaxRateKbps/2))
	return []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-probesize", "32",
		"-analyzeduration", "0",
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", width, height),
		"-framerate", strconv.Itoa(fps),
		"-i", "pipe:0",
		"-an",
		"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2,format=yuv420p",
		"-c:v", "libx264",
		"-preset", profile.Preset,
		"-tune", "zerolatency",
		"-crf", strconv.Itoa(profile.CRF),
		"-maxrate", maxRate,
		"-bufsize", bufferSize,
		"-threads", "0",
		"-pix_fmt", "yuv420p",
		"-profile:v", "baseline",
		"-level", "5.1",
		"-bf", "0",
		"-refs", "1",
		"-g", gop,
		"-keyint_min", gop,
		"-sc_threshold", "0",
		"-x264-params", x264Params,
		"-flush_packets", "1",
		"-muxdelay", "0",
		"-muxpreload", "0",
		"-f", "h264",
		"pipe:1",
	}
}

func (profile h264AdaptiveProfile) screenPipelineProfile() screenPipelineProfile {
	pipelineProfile := defaultScreenPipelineProfile()
	pipelineProfile.StreamMaxEdge = profile.MaxEdge
	pipelineProfile.FrameInterval = time.Second / time.Duration(profile.FPS)
	return pipelineProfile
}

func h264AdaptiveProfiles() []h264AdaptiveProfile {
	baseEdge := h264IntEnv("UNYDESK_H264_MAX_EDGE", 2880, 640, 4096)
	baseFPS := h264IntEnv("UNYDESK_H264_FPS", 45, 10, 60)
	baseCRF := h264IntEnv("UNYDESK_H264_CRF", 14, 12, 32)
	baseMaxRate := h264IntEnv("UNYDESK_H264_MAXRATE_KBPS", 16000, 1000, 80000)
	basePreset := h264Preset()
	baseFrameAge := h264MaxFrameAge()

	if !h264AdaptiveEnabled() {
		return []h264AdaptiveProfile{newH264AdaptiveProfile("manual", baseEdge, baseFPS, baseCRF, basePreset, baseMaxRate, baseFrameAge, 0)}
	}

	upgradeAfter := h264UpgradeAfter()
	return []h264AdaptiveProfile{
		newH264AdaptiveProfile("quality", baseEdge, baseFPS, baseCRF, basePreset, baseMaxRate, baseFrameAge, 0),
		newH264AdaptiveProfile("balanced", minInt(baseEdge, 2560), minInt(baseFPS, 40), clampInt(baseCRF+1, 12, 32), basePreset, minInt(baseMaxRate, 10000), minDuration(baseFrameAge, 80*time.Millisecond), upgradeAfter),
		newH264AdaptiveProfile("responsive", minInt(baseEdge, 2240), minInt(baseFPS, 35), clampInt(baseCRF+2, 12, 32), "ultrafast", minInt(baseMaxRate, 6500), minDuration(baseFrameAge, 70*time.Millisecond), upgradeAfter),
		newH264AdaptiveProfile("recovery", minInt(baseEdge, 1920), minInt(baseFPS, 30), clampInt(baseCRF+4, 12, 32), "ultrafast", minInt(baseMaxRate, 4000), minDuration(baseFrameAge, 60*time.Millisecond), upgradeAfter),
		newH264AdaptiveProfile("survival", minInt(baseEdge, 1600), minInt(baseFPS, 24), clampInt(baseCRF+6, 12, 32), "ultrafast", minInt(baseMaxRate, 2500), minDuration(baseFrameAge, 50*time.Millisecond), upgradeAfter),
	}
}

func newH264AdaptiveProfile(name string, maxEdge, fps, crf int, preset string, maxRateKbps int, maxFrameAge, upgradeAfter time.Duration) h264AdaptiveProfile {
	fps = clampInt(fps, 10, 60)
	return h264AdaptiveProfile{
		Name:                 name,
		MaxEdge:              clampInt(maxEdge, 640, 4096),
		FPS:                  fps,
		CRF:                  clampInt(crf, 12, 32),
		Preset:               normalizeH264Preset(preset, "ultrafast"),
		MaxRateKbps:          clampInt(maxRateKbps, 750, 80000),
		MaxFrameAge:          maxFrameAge,
		UpgradeAfter:         upgradeAfter,
		OverloadWindow:       h264OverloadWindow(),
		RawDropThreshold:     maxInt(3, fps/8),
		EncodedDropThreshold: maxInt(2, fps/10),
	}
}

func h264AdaptiveEnabled() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("UNYDESK_H264_ADAPTIVE")))
	return raw != "0" && raw != "false" && raw != "off" && raw != "no"
}

func h264UpgradeAfter() time.Duration {
	seconds := h264IntEnv("UNYDESK_H264_UPSHIFT_AFTER_SEC", 12, 0, 300)
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func h264OverloadWindow() time.Duration {
	ms := h264IntEnv("UNYDESK_H264_OVERLOAD_WINDOW_MS", 900, 250, 3000)
	return time.Duration(ms) * time.Millisecond
}

func h264Preset() string {
	return normalizeH264Preset(os.Getenv("UNYDESK_H264_PRESET"), "superfast")
}

func normalizeH264Preset(raw, fallback string) string {
	preset := strings.ToLower(strings.TrimSpace(raw))
	switch preset {
	case "ultrafast", "superfast", "veryfast", "faster", "fast", "medium":
		return preset
	default:
		return fallback
	}
}

func h264MaxFrameAge() time.Duration {
	ms := h264IntEnv("UNYDESK_H264_MAX_FRAME_AGE_MS", 90, 0, 1000)
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

func h264NetworkAdaptiveEnabled() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("UNYDESK_H264_NETWORK_ADAPTIVE")))
	return raw != "0" && raw != "false" && raw != "off" && raw != "no"
}

func h264NetworkLossThreshold() float64 {
	return h264FloatEnv("UNYDESK_H264_NETWORK_LOSS_PCT", 2.0, 0.1, 50)
}

func h264NetworkJitterThreshold() float64 {
	return h264FloatEnv("UNYDESK_H264_NETWORK_JITTER_MS", 60, 5, 1000)
}

func h264NetworkNackThreshold() int {
	return h264IntEnv("UNYDESK_H264_NETWORK_NACKS", 8, 1, 200)
}

func h264NetworkScoreThreshold() int {
	return h264IntEnv("UNYDESK_H264_NETWORK_SCORE", 5, 1, 100)
}

func minDuration(a, b time.Duration) time.Duration {
	if a <= 0 || b <= 0 {
		return 0
	}
	if a < b {
		return a
	}
	return b
}

func clampDuration(value, lower, upper time.Duration) time.Duration {
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}

func h264IntEnv(name string, fallback, minValue, maxValue int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return clampInt(value, minValue, maxValue)
}

func h264FloatEnv(name string, fallback, minValue, maxValue float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func h264FPSForInterval(interval time.Duration) int {
	if interval <= 0 {
		return 30
	}
	fps := int((time.Second + interval/2) / interval)
	return clampInt(fps, 10, 60)
}

func captureH264Frame(pipeline *screenPipeline) (*image.RGBA, error) {
	var (
		captured *image.RGBA
		err      error
	)
	for attempt := 0; attempt < 3; attempt++ {
		captured, err = pipeline.capturePrimaryDisplay()
		if err == nil {
			return pipeline.normalizeCapture(captured), nil
		}
		if attempt < 2 {
			time.Sleep(pipeline.profile.CaptureRetryDelay)
		}
	}
	return nil, err
}

type h264NetworkSnapshot struct {
	ObservedAt    time.Time
	LossPercent   float64
	JitterMillis  float64
	Nacks         int
	PLIs          int
	Score         int
	WindowScore   int
	CongestionSeq uint64
}

type h264NetworkMonitor struct {
	mu            sync.Mutex
	windowStart   time.Time
	windowScore   int
	congestionSeq uint64
	last          h264NetworkSnapshot
	lastLogAt     time.Time
}

func newH264NetworkMonitor() *h264NetworkMonitor {
	return &h264NetworkMonitor{}
}

func (monitor *h264NetworkMonitor) observe(packets []rtcp.Packet) {
	if monitor == nil || len(packets) == 0 {
		return
	}

	snapshot := h264NetworkSnapshot{ObservedAt: time.Now()}
	for _, packet := range packets {
		switch feedback := packet.(type) {
		case *rtcp.ReceiverReport:
			for _, report := range feedback.Reports {
				lossPercent := float64(report.FractionLost) * 100 / 256
				if lossPercent > snapshot.LossPercent {
					snapshot.LossPercent = lossPercent
				}
				jitterMillis := float64(report.Jitter) / 90
				if jitterMillis > snapshot.JitterMillis {
					snapshot.JitterMillis = jitterMillis
				}
			}
		case *rtcp.TransportLayerNack:
			snapshot.Nacks += h264NackPacketCount(feedback.Nacks)
		case *rtcp.PictureLossIndication:
			snapshot.PLIs++
		}
	}

	snapshot.Score = h264NetworkScore(snapshot)
	monitor.mu.Lock()
	defer monitor.mu.Unlock()

	now := snapshot.ObservedAt
	if monitor.windowStart.IsZero() || now.Sub(monitor.windowStart) > 1500*time.Millisecond {
		monitor.windowStart = now
		monitor.windowScore = 0
	}
	monitor.windowScore += snapshot.Score
	snapshot.WindowScore = monitor.windowScore
	monitor.last = snapshot

	if monitor.windowScore < h264NetworkScoreThreshold() {
		return
	}

	monitor.congestionSeq++
	monitor.windowScore = 0
	monitor.windowStart = now
	monitor.last.CongestionSeq = monitor.congestionSeq
	if monitor.lastLogAt.IsZero() || now.Sub(monitor.lastLogAt) > 2*time.Second {
		monitor.lastLogAt = now
		fmt.Printf("H264 network congestion: loss=%.1f%% jitter=%.0fms nacks=%d plis=%d score=%d\n", snapshot.LossPercent, snapshot.JitterMillis, snapshot.Nacks, snapshot.PLIs, snapshot.Score)
	}
}

func h264NackPacketCount(pairs []rtcp.NackPair) int {
	count := 0
	for _, pair := range pairs {
		count += len(pair.PacketList())
	}
	return count
}

func h264NetworkScore(snapshot h264NetworkSnapshot) int {
	score := 0
	lossThreshold := h264NetworkLossThreshold()
	if snapshot.LossPercent >= lossThreshold {
		score += 3
	}
	if snapshot.LossPercent >= lossThreshold*2 {
		score += 2
	}

	jitterThreshold := h264NetworkJitterThreshold()
	if snapshot.JitterMillis >= jitterThreshold {
		score += 2
	}
	if snapshot.JitterMillis >= jitterThreshold*2 {
		score += 2
	}

	nackThreshold := h264NetworkNackThreshold()
	if snapshot.Nacks >= nackThreshold {
		score += 3 + snapshot.Nacks/nackThreshold
	}
	if snapshot.PLIs > 0 {
		score += 5 * snapshot.PLIs
	}
	return score
}

func (monitor *h264NetworkMonitor) congestionSequence() uint64 {
	if monitor == nil {
		return 0
	}
	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	return monitor.congestionSeq
}

func (monitor *h264NetworkMonitor) congestionSince(previous uint64) (h264NetworkSnapshot, bool) {
	if monitor == nil {
		return h264NetworkSnapshot{}, false
	}
	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	if monitor.congestionSeq <= previous {
		return h264NetworkSnapshot{}, false
	}
	return monitor.last, true
}

func (monitor *h264NetworkMonitor) lastSnapshot() h264NetworkSnapshot {
	if monitor == nil {
		return h264NetworkSnapshot{}
	}
	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	return monitor.last
}

func waitForH264NetworkCongestion(ctx context.Context, done <-chan struct{}, monitor *h264NetworkMonitor) error {
	if monitor == nil {
		return nil
	}
	seen := monitor.congestionSequence()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-done:
			return nil
		case <-ticker.C:
			if _, congested := monitor.congestionSince(seen); congested {
				return errH264NetworkCongested
			}
		}
	}
}

type h264RawFrame struct {
	image      *image.RGBA
	capturedAt time.Time
}

type h264EncodedSample struct {
	data     []byte
	queuedAt time.Time
}

type h264OverloadTracker struct {
	windowStart time.Time
	window      time.Duration
	threshold   int
	score       int
}

func newH264OverloadTracker(threshold int, window time.Duration) *h264OverloadTracker {
	return &h264OverloadTracker{
		window:    window,
		threshold: maxInt(1, threshold),
	}
}

func (tracker *h264OverloadTracker) record(score int) bool {
	if score <= 0 {
		return false
	}
	now := time.Now()
	if tracker.windowStart.IsZero() || now.Sub(tracker.windowStart) > tracker.window {
		tracker.windowStart = now
		tracker.score = 0
	}
	tracker.score += score
	return tracker.score >= tracker.threshold
}

func writeH264RawFrames(ctx context.Context, done <-chan struct{}, pipeline *screenPipeline, writer io.WriteCloser, firstFrame *image.RGBA, width, height int, frameDuration time.Duration, sessionID string, profile h264AdaptiveProfile) error {
	defer writer.Close()
	frames := make(chan h264RawFrame, 1)
	rawDropCh := make(chan int, 16)
	captureErrCh := make(chan error, 1)
	sendLatestH264RawFrame(frames, h264RawFrame{image: firstFrame, capturedAt: time.Now()})

	go func() {
		captureErrCh <- captureH264RawFrames(ctx, done, pipeline, frames, rawDropCh, width, height, frameDuration)
	}()

	maxFrameAge := profile.MaxFrameAge
	lastDropLogAt := time.Time{}
	overloadTracker := newH264OverloadTracker(profile.RawDropThreshold, profile.OverloadWindow)
	recordDrop := func(score int, message string) error {
		if score <= 0 {
			return nil
		}
		if lastDropLogAt.IsZero() || time.Since(lastDropLogAt) > 2*time.Second {
			lastDropLogAt = time.Now()
			fmt.Printf("H264 screen %s %s\n", sessionID, message)
		}
		if overloadTracker.record(score) {
			return errH264EncoderOverloaded
		}
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-done:
			return nil
		case dropped := <-rawDropCh:
			if err := recordDrop(dropped, fmt.Sprintf("replaced %d raw frames before encoder", dropped)); err != nil {
				return err
			}
		case err := <-captureErrCh:
			return err
		case frame := <-frames:
			dropped := 0
		drain:
			for {
				select {
				case latest := <-frames:
					frame = latest
					dropped++
				default:
					break drain
				}
			}
			if maxFrameAge > 0 && time.Since(frame.capturedAt) > maxFrameAge {
				if err := recordDrop(dropped+3, "dropped stale raw frame before encoder to keep latency low"); err != nil {
					return err
				}
				continue
			}
			if dropped > 0 {
				if err := recordDrop(dropped, fmt.Sprintf("dropped %d stale raw frames before encoder", dropped)); err != nil {
					return err
				}
			}
			if err := writeRGBAFrame(writer, frame.image, width, height); err != nil {
				return err
			}
		}
	}
}

func captureH264RawFrames(ctx context.Context, done <-chan struct{}, pipeline *screenPipeline, frames chan h264RawFrame, rawDropCh chan<- int, width, height int, frameDuration time.Duration) error {
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-done:
			return nil
		case <-ticker.C:
			frame, err := captureH264Frame(pipeline)
			if err != nil {
				time.Sleep(pipeline.profile.CaptureFailureBackoff)
				continue
			}
			bounds := frame.Bounds()
			if bounds.Dx() != width || bounds.Dy() != height {
				return errH264ResolutionChanged
			}
			if dropped := sendLatestH264RawFrame(frames, h264RawFrame{image: frame, capturedAt: time.Now()}); dropped > 0 {
				notifyH264FrameDrops(rawDropCh, dropped)
			}
		}
	}
}

func sendLatestH264RawFrame(frames chan h264RawFrame, frame h264RawFrame) int {
	dropped := 0
	for {
		select {
		case frames <- frame:
			return dropped
		default:
			select {
			case <-frames:
				dropped++
			default:
			}
		}
	}
}

func notifyH264FrameDrops(dropCh chan<- int, dropped int) {
	if dropped <= 0 {
		return
	}
	select {
	case dropCh <- dropped:
	default:
	}
}

func writeRGBAFrame(writer io.Writer, frame *image.RGBA, width, height int) error {
	if frame == nil {
		return fmt.Errorf("empty frame")
	}
	data, ok := contiguousRGBAFrameBytes(frame, width, height)
	if !ok {
		return fmt.Errorf("invalid RGBA frame stride")
	}
	_, err := writer.Write(data)
	return err
}

func contiguousRGBAFrameBytes(frame *image.RGBA, width, height int) ([]byte, bool) {
	if width <= 0 || height <= 0 {
		return nil, false
	}
	length := width * height * 4
	if frame.Stride == width*4 && len(frame.Pix) >= length {
		return frame.Pix[:length], true
	}

	out := make([]byte, length)
	for row := 0; row < height; row++ {
		srcOffset := row * frame.Stride
		dstOffset := row * width * 4
		if srcOffset+width*4 > len(frame.Pix) || dstOffset+width*4 > len(out) {
			return nil, false
		}
		copy(out[dstOffset:dstOffset+width*4], frame.Pix[srcOffset:srcOffset+width*4])
	}
	return out, true
}

func writeH264Samples(reader io.Reader, track *webrtc.TrackLocalStaticSample, frameDuration time.Duration, sessionID string, profile h264AdaptiveProfile) error {
	samples := make(chan h264EncodedSample, 2)
	encodedDropCh := make(chan int, 16)
	readErrCh := make(chan error, 1)
	go func() {
		readErrCh <- readH264AccessUnits(reader, samples, encodedDropCh)
		close(samples)
	}()

	lastDropLogAt := time.Time{}
	lastSampleAt := time.Time{}
	overloadTracker := newH264OverloadTracker(profile.EncodedDropThreshold, profile.OverloadWindow)
	recordDrop := func(score int, message string) error {
		if score <= 0 {
			return nil
		}
		if lastDropLogAt.IsZero() || time.Since(lastDropLogAt) > 2*time.Second {
			lastDropLogAt = time.Now()
			fmt.Printf("H264 screen %s %s\n", sessionID, message)
		}
		if overloadTracker.record(score) {
			return errH264EncoderOverloaded
		}
		return nil
	}
	for {
		select {
		case dropped := <-encodedDropCh:
			if err := recordDrop(dropped, fmt.Sprintf("replaced %d encoded frames before RTP send", dropped)); err != nil {
				return err
			}
		case sample, ok := <-samples:
			if !ok {
				err := <-readErrCh
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
			dropped := 0
		drain:
			for {
				select {
				case latest, ok := <-samples:
					if !ok {
						break drain
					}
					sample = latest
					dropped++
				default:
					break drain
				}
			}
			if dropped > 0 {
				if err := recordDrop(dropped, fmt.Sprintf("dropped %d stale encoded frames to keep latency low", dropped)); err != nil {
					return err
				}
			}
			if profile.MaxFrameAge > 0 && time.Since(sample.queuedAt) > profile.MaxFrameAge {
				if err := recordDrop(dropped+3, "dropped stale encoded frame before RTP send"); err != nil {
					return err
				}
				continue
			}
			now := time.Now()
			sampleDuration := frameDuration
			if !lastSampleAt.IsZero() {
				sampleDuration = clampDuration(now.Sub(lastSampleAt), frameDuration, 500*time.Millisecond)
			}
			lastSampleAt = now
			if err := track.WriteSample(media.Sample{Data: sample.data, Duration: sampleDuration}); err != nil {
				return err
			}
		case err := <-readErrCh:
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func readH264AccessUnits(reader io.Reader, samples chan h264EncodedSample, encodedDropCh chan<- int) error {
	h264, err := h264reader.NewReader(reader)
	if err != nil {
		return err
	}

	var accessUnit [][]byte
	hasVCL := false
	sawAUD := false
	emitAccessUnit := func() error {
		if !hasVCL || len(accessUnit) == 0 {
			return nil
		}
		sample := make([]byte, 0, h264AccessUnitSize(accessUnit))
		for _, nal := range accessUnit {
			sample = append(sample, 0x00, 0x00, 0x00, 0x01)
			sample = append(sample, nal...)
		}
		accessUnit = accessUnit[:0]
		hasVCL = false
		sawAUD = false
		dropped := sendLatestH264Sample(samples, h264EncodedSample{data: sample, queuedAt: time.Now()})
		if dropped > 0 {
			notifyH264FrameDrops(encodedDropCh, dropped)
		}
		return nil
	}

	for {
		nal, readErr := h264.NextNAL()
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return emitAccessUnit()
			}
			return readErr
		}
		if nal == nil || len(nal.Data) == 0 {
			continue
		}
		if hasVCL && h264NALStartsNewAccessUnit(nal.UnitType) {
			if err := emitAccessUnit(); err != nil {
				return err
			}
		}
		if nal.UnitType == h264reader.NalUnitTypeAUD {
			sawAUD = true
		}
		if h264NALIsVCL(nal.UnitType) && hasVCL && !sawAUD {
			if err := emitAccessUnit(); err != nil {
				return err
			}
		}
		accessUnit = append(accessUnit, append([]byte(nil), nal.Data...))
		if h264NALIsVCL(nal.UnitType) {
			hasVCL = true
		}
	}
}

func sendLatestH264Sample(samples chan h264EncodedSample, sample h264EncodedSample) int {
	dropped := 0
	for {
		select {
		case samples <- sample:
			return dropped
		default:
			select {
			case <-samples:
				dropped++
			default:
			}
		}
	}
}

func h264AccessUnitSize(nals [][]byte) int {
	size := 0
	for _, nal := range nals {
		size += 4 + len(nal)
	}
	return size
}

func h264NALStartsNewAccessUnit(unitType h264reader.NalUnitType) bool {
	switch unitType {
	case h264reader.NalUnitTypeAUD, h264reader.NalUnitTypeSPS, h264reader.NalUnitTypePPS:
		return true
	default:
		return false
	}
}

func h264NALIsVCL(unitType h264reader.NalUnitType) bool {
	switch unitType {
	case h264reader.NalUnitTypeCodedSliceNonIdr,
		h264reader.NalUnitTypeCodedSliceDataPartitionA,
		h264reader.NalUnitTypeCodedSliceDataPartitionB,
		h264reader.NalUnitTypeCodedSliceDataPartitionC,
		h264reader.NalUnitTypeCodedSliceIdr,
		h264reader.NalUnitTypeCodedSliceAux:
		return true
	default:
		return false
	}
}

func logH264EncoderStderr(sessionID string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			fmt.Printf("H264 encoder %s: %s\n", sessionID, line)
		}
	}
}
