package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

type av1EncodedSample struct {
	data          []byte
	queuedAt      time.Time
	advancesClock bool
}

var (
	av1EncoderLookupMu    sync.Mutex
	av1EncoderLookupCache = map[string]cachedEncoderLookup{}
)

func newAV1ScreenTrack(pc *webrtc.PeerConnection, sessionID string) (*webrtc.TrackLocalStaticSample, *h264NetworkMonitor, error) {
	ffmpegPath, ok := lookupFFmpegBinary()
	if !ok {
		return nil, nil, fmt.Errorf("ffmpeg encoder not found")
	}
	if _, ok := lookupAV1Encoder(ffmpegPath); !ok {
		return nil, nil, fmt.Errorf("ffmpeg AV1 encoder not found")
	}

	track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypeAV1,
		ClockRate: 90000,
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

	fmt.Printf("AV1 screen track negotiated for %s\n", sessionID)
	return track, networkMonitor, nil
}

func streamScreenAV1ToTrack(ctx context.Context, done <-chan struct{}, sessionID string, track *webrtc.TrackLocalStaticSample, networkMonitor *h264NetworkMonitor, emitStatus func(map[string]any), emitError func(string), markFirstSample func()) {
	ffmpegPath, ok := lookupFFmpegBinary()
	if !ok {
		emitError("AV1 encoder unavailable: install ffmpeg next to unydesk-host or add FFmpeg to PATH.")
		return
	}
	encoder, ok := lookupAV1Encoder(ffmpegPath)
	if !ok {
		emitError("AV1 encoder unavailable: FFmpeg has no AV1 encoder. Falling back to H.264/data-channel.")
		return
	}
	if networkMonitor == nil {
		networkMonitor = newH264NetworkMonitor()
	}

	profiles := av1AdaptiveProfiles()
	capture := defaultCaptureProvider()
	profileIndex := 0
	emitProfileStatus := func(action string, profile h264AdaptiveProfile, network *h264NetworkSnapshot) {
		if emitStatus == nil {
			return
		}
		payload := map[string]any{
			"type":      "screen_status",
			"transport": "av1",
			"action":    action,
			"profile":   profile.Name,
			"encoder":   encoder,
			"max_edge":  profile.MaxEdge,
			"fps":       profile.FPS,
			"crf":       profile.CRF,
			"maxrate":   profile.MaxRateKbps,
			"age_ms":    profile.MaxFrameAge.Milliseconds(),
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
		err := runAV1EncoderSession(ctx, done, sessionID, ffmpegPath, encoder, pipeline, track, networkMonitor, profile, emitStatus, markFirstSample)
		if err == nil || ctx.Err() != nil {
			return
		}
		if errors.Is(err, errH264ResolutionChanged) {
			fmt.Printf("AV1 screen restarting for %s after resolution change\n", sessionID)
			time.Sleep(150 * time.Millisecond)
			continue
		}
		if (errors.Is(err, errH264EncoderOverloaded) || errors.Is(err, errH264NetworkCongested)) && profileIndex < len(profiles)-1 {
			nextProfile := profiles[profileIndex+1]
			fmt.Printf("AV1 adaptive downshift for %s: %s -> %s\n", sessionID, profile.Name, nextProfile.Name)
			profileIndex++
			emitProfileStatus("downshift", nextProfile, nil)
			time.Sleep(120 * time.Millisecond)
			continue
		}
		if errors.Is(err, errH264StableUpgrade) && profileIndex > 0 {
			nextProfile := profiles[profileIndex-1]
			fmt.Printf("AV1 adaptive upshift for %s: %s -> %s\n", sessionID, profile.Name, nextProfile.Name)
			profileIndex--
			emitProfileStatus("upshift", nextProfile, nil)
			time.Sleep(120 * time.Millisecond)
			continue
		}
		message := fmt.Sprintf("AV1 screen stream stopped: %v", err)
		fmt.Printf("AV1 screen stopped for %s: %v\n", sessionID, err)
		emitError(message)
		return
	}
}

func runAV1EncoderSession(ctx context.Context, done <-chan struct{}, sessionID, ffmpegPath, encoder string, pipeline *screenPipeline, track *webrtc.TrackLocalStaticSample, networkMonitor *h264NetworkMonitor, profile h264AdaptiveProfile, emitStatus func(map[string]any), markFirstSample func()) error {
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

	args := av1FFmpegArgs(width, height, fps, encoder, profile)
	fmt.Printf("AV1 encoder starting for %s: encoder=%s profile=%s, %dx%d @ %dfps, CRF %d\n", sessionID, encoder, profile.Name, width, height, fps, profile.CRF)
	metrics := newRealtimeVideoMetrics("av1", sessionID, profile, emitStatus)
	cmd := newBackgroundCommandContext(encoderCtx, ffmpegPath, args...)
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
	go logAV1EncoderStderr(sessionID, stderr)
	go func() {
		errCh <- writeH264RawFrames(encoderCtx, done, pipeline, stdin, firstFrame, width, height, frameDuration, sessionID, profile, metrics)
	}()
	go func() {
		errCh <- writeAV1Samples(stdout, track, frameDuration, sessionID, profile, metrics, markFirstSample)
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
		if err != nil && encoderCtx.Err() == nil && firstErr == nil {
			firstErr = err
		}
		cancel()
		_ = stdin.Close()
	}
	if firstErr != nil && !errors.Is(firstErr, context.Canceled) && !errors.Is(firstErr, io.EOF) {
		return firstErr
	}
	return nil
}

func av1FFmpegArgs(width, height, fps int, encoder string, profile h264AdaptiveProfile) []string {
	gop := strconv.Itoa(maxInt(1, fps))
	args := []string{
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
		"-c:v", encoder,
	}
	switch encoder {
	case "libaom-av1":
		args = append(args, "-deadline", "realtime", "-cpu-used", "8", "-row-mt", "1", "-lag-in-frames", "0", "-error-resilient", "1", "-crf", strconv.Itoa(profile.CRF), "-b:v", "0")
	case "libsvtav1":
		args = append(args, "-preset", "12", "-crf", strconv.Itoa(profile.CRF))
	default:
		args = append(args, "-b:v", fmt.Sprintf("%dk", profile.MaxRateKbps))
	}
	args = append(args,
		"-g", gop,
		"-keyint_min", gop,
		"-sc_threshold", "0",
		"-pix_fmt", "yuv420p",
		"-flush_packets", "1",
		"-muxdelay", "0",
		"-muxpreload", "0",
		"-f", "ivf",
		"pipe:1",
	)
	return args
}

func av1AdaptiveProfiles() []h264AdaptiveProfile {
	profiles := h264AdaptiveProfiles()
	for i := range profiles {
		profiles[i].CRF = clampInt(h264IntEnv("UNYDESK_AV1_CRF", profiles[i].CRF+8, 18, 45), 18, 45)
		profiles[i].MaxRateKbps = minInt(profiles[i].MaxRateKbps, h264IntEnv("UNYDESK_AV1_MAXRATE_KBPS", 9000, 750, 80000))
		profiles[i].FPS = minInt(profiles[i].FPS, h264IntEnv("UNYDESK_AV1_FPS", 30, 10, 60))
		profiles[i].MaxFrameAge = minDuration(profiles[i].MaxFrameAge, time.Duration(h264IntEnv("UNYDESK_AV1_MAX_FRAME_AGE_MS", 120, 0, 1000))*time.Millisecond)
	}
	return profiles
}

func lookupAV1Encoder(ffmpegPath string) (string, bool) {
	cacheKey := ffmpegPath + "\x00" + strings.TrimSpace(os.Getenv("UNYDESK_AV1_ENCODER"))
	av1EncoderLookupMu.Lock()
	if cached, exists := av1EncoderLookupCache[cacheKey]; exists {
		av1EncoderLookupMu.Unlock()
		return cached.encoder, cached.ok
	}
	av1EncoderLookupMu.Unlock()

	if configured := strings.TrimSpace(os.Getenv("UNYDESK_AV1_ENCODER")); configured != "" {
		storeAV1EncoderLookup(cacheKey, configured, true)
		return configured, true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := newBackgroundCommandContext(ctx, ffmpegPath, "-hide_banner", "-encoders").CombinedOutput()
	if err != nil && len(out) == 0 {
		return "", false
	}
	encoders := string(out)
	for _, candidate := range []string{"av1_nvenc", "av1_qsv", "av1_amf", "libsvtav1", "libaom-av1"} {
		if strings.Contains(encoders, candidate) {
			storeAV1EncoderLookup(cacheKey, candidate, true)
			return candidate, true
		}
	}
	storeAV1EncoderLookup(cacheKey, "", false)
	return "", false
}

func storeAV1EncoderLookup(cacheKey, encoder string, ok bool) {
	av1EncoderLookupMu.Lock()
	defer av1EncoderLookupMu.Unlock()
	av1EncoderLookupCache[cacheKey] = cachedEncoderLookup{encoder: encoder, ok: ok}
}

func writeAV1Samples(reader io.Reader, track *webrtc.TrackLocalStaticSample, frameDuration time.Duration, sessionID string, profile h264AdaptiveProfile, metrics *realtimeVideoMetrics, markFirstSample func()) error {
	samples := make(chan av1EncodedSample, 8)
	encodedDropCh := make(chan int, 16)
	readErrCh := make(chan error, 1)
	go func() {
		readErrCh <- readAV1IVFSamples(reader, samples, encodedDropCh)
		close(samples)
	}()

	lastDropLogAt := time.Time{}
	firstSampleMarked := false
	overloadTracker := newH264OverloadTracker(profile.EncodedDropThreshold, profile.OverloadWindow)
	recordDrop := func(score int, message string) error {
		if score <= 0 {
			return nil
		}
		if lastDropLogAt.IsZero() || time.Since(lastDropLogAt) > 2*time.Second {
			lastDropLogAt = time.Now()
			fmt.Printf("AV1 screen %s %s\n", sessionID, message)
		}
		metrics.observeEncodedDrop(score)
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
			if profile.MaxFrameAge > 0 && sample.advancesClock && time.Since(sample.queuedAt) > profile.MaxFrameAge {
				if err := recordDrop(3, "dropped stale encoded frame before RTP send"); err != nil {
					return err
				}
				continue
			}
			duration := time.Duration(0)
			if sample.advancesClock {
				duration = frameDuration
			}
			if err := track.WriteSample(media.Sample{Data: sample.data, Duration: duration}); err != nil {
				return err
			}
			metrics.observeEncodedSample(len(sample.data), sample.queuedAt, sample.advancesClock)
			if sample.advancesClock && !firstSampleMarked {
				firstSampleMarked = true
				if markFirstSample != nil {
					markFirstSample()
				}
				fmt.Printf("AV1 first encoded frame sent for %s\n", sessionID)
			}
		case err := <-readErrCh:
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func readAV1IVFSamples(reader io.Reader, samples chan av1EncodedSample, encodedDropCh chan<- int) error {
	buffered := bufio.NewReader(reader)
	header := make([]byte, 32)
	if _, err := io.ReadFull(buffered, header); err != nil {
		return err
	}
	if !bytes.Equal(header[0:4], []byte("DKIF")) {
		return fmt.Errorf("invalid AV1 IVF stream")
	}
	frameHeader := make([]byte, 12)
	for {
		if _, err := io.ReadFull(buffered, frameHeader); err != nil {
			return err
		}
		size := binary.LittleEndian.Uint32(frameHeader[0:4])
		if size == 0 || size > 32*1024*1024 {
			return fmt.Errorf("invalid AV1 frame size %d", size)
		}
		frame := make([]byte, int(size))
		if _, err := io.ReadFull(buffered, frame); err != nil {
			return err
		}
		obus := splitAV1OBUs(frame)
		if len(obus) == 0 {
			obus = [][]byte{frame}
		}
		videoIndexes := av1VideoOBUIndexes(obus)
		lastVideoIndex := -1
		if len(videoIndexes) > 0 {
			lastVideoIndex = videoIndexes[len(videoIndexes)-1]
		}
		for index, obu := range obus {
			if len(obu) == 0 || av1ShouldSkipOBU(obu) {
				continue
			}
			sample := av1EncodedSample{data: append([]byte(nil), obu...), queuedAt: time.Now(), advancesClock: index == lastVideoIndex}
			if dropped := sendLatestAV1Sample(samples, sample); dropped > 0 {
				notifyH264FrameDrops(encodedDropCh, dropped)
			}
		}
	}
}

func splitAV1OBUs(frame []byte) [][]byte {
	var obus [][]byte
	for offset := 0; offset < len(frame); {
		start := offset
		header := frame[offset]
		offset++
		if header&0x04 != 0 {
			offset++
			if offset > len(frame) {
				return nil
			}
		}
		if header&0x02 == 0 {
			return [][]byte{frame}
		}
		size, read, ok := decodeAV1LEB128(frame[offset:])
		if !ok {
			return nil
		}
		offset += read
		end := offset + int(size)
		if end > len(frame) {
			return nil
		}
		obus = append(obus, frame[start:end])
		offset = end
	}
	return obus
}

func decodeAV1LEB128(data []byte) (uint64, int, bool) {
	var value uint64
	for i := 0; i < len(data) && i < 8; i++ {
		value |= uint64(data[i]&0x7f) << (7 * i)
		if data[i]&0x80 == 0 {
			return value, i + 1, true
		}
	}
	return 0, 0, false
}

func av1VideoOBUIndexes(obus [][]byte) []int {
	indexes := []int{}
	for index, obu := range obus {
		switch av1OBUType(obu) {
		case 3, 4, 6, 7:
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func av1ShouldSkipOBU(obu []byte) bool {
	switch av1OBUType(obu) {
	case 2, 15:
		return true
	default:
		return false
	}
}

func av1OBUType(obu []byte) byte {
	if len(obu) == 0 {
		return 0
	}
	return (obu[0] >> 3) & 0x0f
}

func sendLatestAV1Sample(samples chan av1EncodedSample, sample av1EncodedSample) int {
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

func logAV1EncoderStderr(sessionID string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			fmt.Printf("AV1 encoder %s: %s\n", sessionID, line)
		}
	}
}
