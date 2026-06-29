package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

const (
	h265RTPClockRate      = 90000
	h265PayloadMTU        = 1200
	h265NALUHeaderSize    = 2
	h265FUHeaderSize      = 3
	h265NALTypeFU         = 49
	h265NALTypeAUD        = 35
	h265NALTypePrefixSEI  = 39
	h265NALTypeSuffixSEI  = 40
	h265NALTypeFillerData = 38
	h265ProbeTimeout      = 1500 * time.Millisecond
)

type h265Payloader struct{}

type cachedEncoderLookup struct {
	encoder string
	ok      bool
}

var (
	h265EncoderLookupMu    sync.Mutex
	h265EncoderLookupCache = map[string]cachedEncoderLookup{}
)

func newH265ScreenTrack(pc *webrtc.PeerConnection, sessionID string) (*webrtc.TrackLocalStaticRTP, *h264NetworkMonitor, error) {
	ffmpegPath, ok := lookupFFmpegBinary()
	if !ok {
		return nil, nil, fmt.Errorf("ffmpeg encoder not found")
	}
	if _, ok := lookupH265Encoder(ffmpegPath); !ok {
		return nil, nil, fmt.Errorf("ffmpeg H.265 encoder not found")
	}

	track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypeH265,
		ClockRate: h265RTPClockRate,
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

	fmt.Printf("H265 screen track negotiated for %s\n", sessionID)
	return track, networkMonitor, nil
}

func streamScreenH265ToTrack(ctx context.Context, done <-chan struct{}, sessionID string, track *webrtc.TrackLocalStaticRTP, networkMonitor *h264NetworkMonitor, emitStatus func(map[string]any), emitError func(string), markFirstSample func()) {
	ffmpegPath, ok := lookupFFmpegBinary()
	if !ok {
		message := "H.265 encoder unavailable: install ffmpeg.exe next to unydesk-host.exe or add FFmpeg to PATH."
		fmt.Printf("H265 screen disabled for %s: ffmpeg not found\n", sessionID)
		if emitStatus != nil {
			emitStatus(map[string]any{
				"type":      "screen_status",
				"transport": "h265",
				"action":    "encoder-unavailable",
				"error":     "ffmpeg not found",
			})
		}
		emitError(message)
		return
	}
	encoder, ok := lookupH265Encoder(ffmpegPath)
	if !ok {
		if emitStatus != nil {
			emitStatus(map[string]any{
				"type":      "screen_status",
				"transport": "h265",
				"action":    "encoder-unavailable",
				"error":     "no usable HEVC encoder",
				"ffmpeg":    filepath.Base(ffmpegPath),
			})
		}
		emitError("H.265 encoder unavailable: FFmpeg has no usable HEVC encoder. Falling back to peer frame channel.")
		return
	}
	if networkMonitor == nil {
		networkMonitor = newH264NetworkMonitor()
	}

	profiles := h265AdaptiveProfiles()
	capture := defaultCaptureProvider()
	profileIndex := h264InitialProfileIndex(profiles)
	floorOverloads := 0
	emitProfileStatus := func(action string, profile h264AdaptiveProfile, network *h264NetworkSnapshot) {
		if emitStatus == nil {
			return
		}
		payload := map[string]any{
			"type":      "screen_status",
			"transport": "h265",
			"action":    action,
			"profile":   profile.Name,
			"encoder":   encoder,
			"max_edge":  profile.MaxEdge,
			"fps":       profile.FPS,
			"crf":       profile.CRF,
			"preset":    profile.Preset,
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
		err := runH265EncoderSession(ctx, done, sessionID, ffmpegPath, encoder, pipeline, track, networkMonitor, profile, emitStatus, markFirstSample)
		if err == nil || ctx.Err() != nil {
			return
		}
		if errors.Is(err, errH264ResolutionChanged) {
			fmt.Printf("H265 screen restarting for %s after resolution change\n", sessionID)
			time.Sleep(150 * time.Millisecond)
			continue
		}
		if errors.Is(err, errH264EncoderOverloaded) && profileIndex < len(profiles)-1 {
			nextProfile := profiles[profileIndex+1]
			fmt.Printf("H265 adaptive downshift for %s: %s -> %s after encoder backlog\n", sessionID, profile.Name, nextProfile.Name)
			profileIndex++
			floorOverloads = 0
			emitProfileStatus("downshift", nextProfile, nil)
			time.Sleep(120 * time.Millisecond)
			continue
		}
		if errors.Is(err, errH264NetworkCongested) && profileIndex < len(profiles)-1 {
			nextProfile := profiles[profileIndex+1]
			profileIndex++
			floorOverloads = 0
			snapshot, _ := networkMonitor.congestionSince(0)
			fmt.Printf("H265 network downshift for %s: %s -> %s after loss %.1f%%, jitter %.0fms, nacks=%d, plis=%d\n", sessionID, profile.Name, nextProfile.Name, snapshot.LossPercent, snapshot.JitterMillis, snapshot.Nacks, snapshot.PLIs)
			emitProfileStatus("network-downshift", nextProfile, &snapshot)
			time.Sleep(120 * time.Millisecond)
			continue
		}
		if (errors.Is(err, errH264EncoderOverloaded) || errors.Is(err, errH264NetworkCongested)) && profileIndex == len(profiles)-1 && h264AdaptiveEnabled() {
			floorOverloads++
			if floorOverloads <= 3 {
				fmt.Printf("H265 adaptive floor retry for %s: profile=%s, overload=%d/3\n", sessionID, profile.Name, floorOverloads)
				time.Sleep(200 * time.Millisecond)
				continue
			}
		}
		if errors.Is(err, errH264StableUpgrade) && profileIndex > 0 {
			nextProfile := profiles[profileIndex-1]
			fmt.Printf("H265 adaptive upshift for %s: %s -> %s after stable stream\n", sessionID, profile.Name, nextProfile.Name)
			profileIndex--
			floorOverloads = 0
			emitProfileStatus("upshift", nextProfile, nil)
			time.Sleep(120 * time.Millisecond)
			continue
		}
		message := fmt.Sprintf("H.265 screen stream stopped: %v", err)
		fmt.Printf("H265 screen stopped for %s: %v\n", sessionID, err)
		emitError(message)
		return
	}
}

func runH265EncoderSession(ctx context.Context, done <-chan struct{}, sessionID, ffmpegPath, encoder string, pipeline *screenPipeline, track *webrtc.TrackLocalStaticRTP, networkMonitor *h264NetworkMonitor, profile h264AdaptiveProfile, emitStatus func(map[string]any), markFirstSample func()) error {
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

	args := h265FFmpegArgs(width, height, fps, encoder, profile)
	fmt.Printf("H265 encoder starting for %s: encoder=%s profile=%s, %dx%d @ %dfps, CRF %d, max frame age %s\n", sessionID, encoder, profile.Name, width, height, fps, profile.CRF, profile.MaxFrameAge)
	metrics := newRealtimeVideoMetrics("h265", sessionID, profile, emitStatus)
	metrics.emitLifecycle("capture-ready", map[string]any{
		"width":  width,
		"height": height,
	})
	metrics.emitLifecycle("encoder-starting", map[string]any{
		"encoder": encoder,
		"ffmpeg":  filepath.Base(ffmpegPath),
	})
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
		metrics.emitLifecycle("encoder-start-error", map[string]any{
			"encoder": encoder,
			"error":   err.Error(),
		})
		return err
	}
	metrics.emitLifecycle("encoder-started", map[string]any{
		"encoder": encoder,
	})

	componentCount := 3
	errCh := make(chan error, 5)
	go logVideoEncoderStderr("H265", sessionID, stderr)
	go func() {
		errCh <- writeH264RawFrames(encoderCtx, done, pipeline, stdin, firstFrame, width, height, frameDuration, sessionID, profile, metrics)
	}()
	go func() {
		errCh <- writeH265RTPPackets(stdout, track, frameDuration, sessionID, profile, metrics, markFirstSample)
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

func h265FFmpegArgs(width, height, fps int, encoder string, profile h264AdaptiveProfile) []string {
	gop := strconv.Itoa(maxInt(1, fps))
	maxRate := fmt.Sprintf("%dk", profile.MaxRateKbps)
	bufferSize := fmt.Sprintf("%dk", maxInt(250, profile.MaxRateKbps/8))
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
	case "hevc_nvenc":
		args = append(args,
			"-preset", "p1",
			"-tune", "ull",
			"-rc", "cbr",
			"-b:v", maxRate,
			"-maxrate", maxRate,
			"-bufsize", bufferSize,
			"-bf", "0",
			"-g", gop,
			"-forced-idr", "1",
			"-rc-lookahead", "0",
		)
	case "hevc_qsv":
		args = append(args,
			"-preset", "veryfast",
			"-look_ahead", "0",
			"-b:v", maxRate,
			"-maxrate", maxRate,
			"-bufsize", bufferSize,
			"-bf", "0",
			"-g", gop,
		)
	case "hevc_amf":
		args = append(args,
			"-quality", "speed",
			"-usage", "ultralowlatency",
			"-b:v", maxRate,
			"-maxrate", maxRate,
			"-bufsize", bufferSize,
			"-bf", "0",
			"-g", gop,
		)
	case "hevc_videotoolbox":
		args = append(args,
			"-realtime", "1",
			"-b:v", maxRate,
			"-g", gop,
		)
	default:
		x265Params := fmt.Sprintf("keyint=%s:min-keyint=%s:scenecut=0:open-gop=0:bframes=0:b-adapt=0:rc-lookahead=0:repeat-headers=1", gop, gop)
		args = append(args,
			"-preset", "ultrafast",
			"-tune", "zerolatency",
			"-crf", strconv.Itoa(profile.CRF),
			"-maxrate", maxRate,
			"-bufsize", bufferSize,
			"-threads", strconv.Itoa(h264EncoderThreads()),
			"-g", gop,
			"-keyint_min", gop,
			"-x265-params", x265Params,
		)
	}
	args = append(args,
		"-pix_fmt", "yuv420p",
		"-flush_packets", "1",
		"-muxdelay", "0",
		"-muxpreload", "0",
		"-f", "hevc",
		"pipe:1",
	)
	return args
}

func h265AdaptiveProfiles() []h264AdaptiveProfile {
	profiles := h264AdaptiveProfiles()
	maxRateCap := h264IntEnv("UNYDESK_H265_MAXRATE_KBPS", 10000, 750, 80000)
	for i := range profiles {
		profiles[i].CRF = clampInt(h264IntEnv("UNYDESK_H265_CRF", profiles[i].CRF+2, 14, 36), 14, 36)
		profiles[i].MaxRateKbps = minInt(profiles[i].MaxRateKbps, maxRateCap)
	}
	return profiles
}

func lookupH265Encoder(ffmpegPath string) (string, bool) {
	cacheKey := ffmpegPath + "\x00" + strings.TrimSpace(os.Getenv("UNYDESK_H265_ENCODER"))
	h265EncoderLookupMu.Lock()
	if cached, exists := h265EncoderLookupCache[cacheKey]; exists {
		h265EncoderLookupMu.Unlock()
		return cached.encoder, cached.ok
	}
	h265EncoderLookupMu.Unlock()

	candidates := []string{}
	if configured := strings.TrimSpace(os.Getenv("UNYDESK_H265_ENCODER")); configured != "" {
		candidates = append(candidates, configured)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := newBackgroundCommandContext(ctx, ffmpegPath, "-hide_banner", "-encoders").CombinedOutput()
	if err != nil && len(out) == 0 {
		return "", false
	}
	encoders := string(out)
	for _, candidate := range []string{"hevc_nvenc", "hevc_qsv", "hevc_amf", "hevc_videotoolbox", "libx265"} {
		if strings.Contains(encoders, candidate) {
			candidates = append(candidates, candidate)
		}
	}
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		if h265EncoderProbeOK(ffmpegPath, candidate) {
			storeH265EncoderLookup(cacheKey, candidate, true)
			return candidate, true
		}
	}
	storeH265EncoderLookup(cacheKey, "", false)
	return "", false
}

func storeH265EncoderLookup(cacheKey, encoder string, ok bool) {
	h265EncoderLookupMu.Lock()
	defer h265EncoderLookupMu.Unlock()
	h265EncoderLookupCache[cacheKey] = cachedEncoderLookup{encoder: encoder, ok: ok}
}

func h265EncoderProbeOK(ffmpegPath, encoder string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), h265ProbeTimeout)
	defer cancel()
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-f", "lavfi",
		"-i", "color=size=64x64:rate=1:color=black",
		"-frames:v", "1",
		"-an",
		"-c:v", encoder,
		"-pix_fmt", "yuv420p",
		"-f", "hevc",
		"-",
	}
	cmd := newBackgroundCommandContext(ctx, ffmpegPath, args...)
	cmd.Stdin = nil
	cmd.Stderr = io.Discard
	cmd.Stdout = io.Discard
	return cmd.Run() == nil
}

func writeH265RTPPackets(reader io.Reader, track *webrtc.TrackLocalStaticRTP, frameDuration time.Duration, sessionID string, profile h264AdaptiveProfile, metrics *realtimeVideoMetrics, markFirstSample func()) error {
	samples := make(chan h264EncodedSample, 1)
	encodedDropCh := make(chan int, 16)
	readErrCh := make(chan error, 1)
	go func() {
		readErrCh <- readH265AccessUnits(reader, samples, encodedDropCh)
		close(samples)
	}()

	packetizer := rtp.NewPacketizer(h265PayloadMTU, 0, 0, &h265Payloader{}, rtp.NewRandomSequencer(), h265RTPClockRate)
	samplesPerFrame := uint32(maxInt(1, int(frameDuration*time.Duration(h265RTPClockRate)/time.Second)))
	lastDropLogAt := time.Time{}
	firstSampleMarked := false
	overloadTracker := newH264OverloadTracker(profile.EncodedDropThreshold, profile.OverloadWindow)
	recordDrop := func(score int, message string) error {
		if score <= 0 {
			return nil
		}
		if lastDropLogAt.IsZero() || time.Since(lastDropLogAt) > 2*time.Second {
			lastDropLogAt = time.Now()
			fmt.Printf("H265 screen %s %s\n", sessionID, message)
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
			packets := packetizer.Packetize(sample.data, samplesPerFrame)
			if len(packets) == 0 {
				continue
			}
			for _, packet := range packets {
				if err := track.WriteRTP(packet); err != nil {
					return err
				}
			}
			metrics.observeEncodedSample(len(sample.data), sample.queuedAt, true)
			if !firstSampleMarked {
				firstSampleMarked = true
				if markFirstSample != nil {
					markFirstSample()
				}
				fmt.Printf("H265 first encoded frame sent for %s\n", sessionID)
			}
		case err := <-readErrCh:
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func readH265AccessUnits(reader io.Reader, samples chan h264EncodedSample, encodedDropCh chan<- int) error {
	var accessUnit [][]byte
	hasVCL := false
	emitAccessUnit := func() error {
		if !hasVCL || len(accessUnit) == 0 {
			accessUnit = accessUnit[:0]
			hasVCL = false
			return nil
		}
		sample := make([]byte, 0, h265AccessUnitSize(accessUnit))
		for _, nal := range accessUnit {
			sample = append(sample, 0x00, 0x00, 0x00, 0x01)
			sample = append(sample, nal...)
		}
		accessUnit = accessUnit[:0]
		hasVCL = false
		dropped := sendLatestH264Sample(samples, h264EncodedSample{data: sample, queuedAt: time.Now()})
		if dropped > 0 {
			notifyH264FrameDrops(encodedDropCh, dropped)
		}
		return nil
	}

	err := readAnnexBNALUnits(reader, func(nal []byte) error {
		if len(nal) < h265NALUHeaderSize {
			return nil
		}
		nalType := h265NALType(nal)
		if hasVCL && h265NALStartsNewAccessUnit(nalType) {
			if err := emitAccessUnit(); err != nil {
				return err
			}
		}
		if hasVCL && h265NALIsVCL(nalType) && h265NALFirstSlice(nal) {
			if err := emitAccessUnit(); err != nil {
				return err
			}
		}
		accessUnit = append(accessUnit, append([]byte(nil), nal...))
		if h265NALIsVCL(nalType) {
			hasVCL = true
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return emitAccessUnit()
}

func (p *h265Payloader) Payload(mtu uint16, payload []byte) [][]byte {
	payloads := [][]byte{}
	if len(payload) == 0 || mtu <= h265FUHeaderSize {
		return payloads
	}
	forEachAnnexBNAL(payload, func(nal []byte) {
		if len(nal) < h265NALUHeaderSize {
			return
		}
		nalType := h265NALType(nal)
		if nalType == h265NALTypeAUD || nalType == h265NALTypeFillerData {
			return
		}
		if len(nal) <= int(mtu) {
			out := append([]byte(nil), nal...)
			payloads = append(payloads, out)
			return
		}

		maxFragmentSize := int(mtu) - h265FUHeaderSize
		nalPayload := nal[h265NALUHeaderSize:]
		if maxFragmentSize <= 0 || len(nalPayload) == 0 {
			return
		}
		fuIndicator0 := (nal[0] & 0x81) | byte(h265NALTypeFU<<1)
		fuIndicator1 := nal[1]
		for offset := 0; offset < len(nalPayload); offset += maxFragmentSize {
			end := minInt(offset+maxFragmentSize, len(nalPayload))
			fuHeader := byte(nalType)
			if offset == 0 {
				fuHeader |= 0x80
			}
			if end == len(nalPayload) {
				fuHeader |= 0x40
			}
			out := make([]byte, h265FUHeaderSize+end-offset)
			out[0] = fuIndicator0
			out[1] = fuIndicator1
			out[2] = fuHeader
			copy(out[h265FUHeaderSize:], nalPayload[offset:end])
			payloads = append(payloads, out)
		}
	})
	return payloads
}

func readAnnexBNALUnits(reader io.Reader, emit func([]byte) error) error {
	buffer := make([]byte, 0, 256*1024)
	chunk := make([]byte, 32*1024)
	for {
		n, readErr := reader.Read(chunk)
		if n > 0 {
			buffer = append(buffer, chunk[:n]...)
			nextBuffer, err := emitCompleteAnnexBNALUnits(buffer, false, emit)
			if err != nil {
				return err
			}
			buffer = nextBuffer
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				_, err := emitCompleteAnnexBNALUnits(buffer, true, emit)
				return err
			}
			return readErr
		}
	}
}

func emitCompleteAnnexBNALUnits(buffer []byte, flush bool, emit func([]byte) error) ([]byte, error) {
	for {
		first, firstLen := findAnnexBStartCode(buffer, 0)
		if first < 0 {
			if len(buffer) > 4 {
				return append([]byte(nil), buffer[len(buffer)-4:]...), nil
			}
			return buffer, nil
		}
		if first > 0 {
			buffer = buffer[first:]
		}
		next, _ := findAnnexBStartCode(buffer, firstLen)
		if next < 0 {
			if flush {
				nal := trimAnnexBNAL(buffer[firstLen:])
				if len(nal) > 0 {
					if err := emit(append([]byte(nil), nal...)); err != nil {
						return nil, err
					}
				}
				return nil, nil
			}
			return buffer, nil
		}
		nal := trimAnnexBNAL(buffer[firstLen:next])
		if len(nal) > 0 {
			if err := emit(append([]byte(nil), nal...)); err != nil {
				return nil, err
			}
		}
		buffer = buffer[next:]
	}
}

func forEachAnnexBNAL(payload []byte, emit func([]byte)) {
	emitted := false
	_, _ = emitCompleteAnnexBNALUnits(payload, true, func(nal []byte) error {
		emitted = true
		emit(nal)
		return nil
	})
	if !emitted && len(payload) > 0 {
		emit(payload)
	}
}

func findAnnexBStartCode(data []byte, from int) (int, int) {
	if from < 0 {
		from = 0
	}
	for i := from; i+2 < len(data); i++ {
		if data[i] != 0x00 || data[i+1] != 0x00 {
			continue
		}
		if i+3 < len(data) && data[i+2] == 0x00 && data[i+3] == 0x01 {
			return i, 4
		}
		if data[i+2] == 0x01 {
			return i, 3
		}
	}
	return -1, 0
}

func trimAnnexBNAL(nal []byte) []byte {
	start := 0
	for start < len(nal) && nal[start] == 0x00 {
		start++
	}
	end := len(nal)
	for end > start && nal[end-1] == 0x00 {
		end--
	}
	return nal[start:end]
}

func h265AccessUnitSize(accessUnit [][]byte) int {
	size := 0
	for _, nal := range accessUnit {
		size += 4 + len(nal)
	}
	return size
}

func h265NALType(nal []byte) int {
	if len(nal) < h265NALUHeaderSize {
		return -1
	}
	return int((nal[0] >> 1) & 0x3f)
}

func h265NALIsVCL(nalType int) bool {
	return nalType >= 0 && nalType <= 31
}

func h265NALStartsNewAccessUnit(nalType int) bool {
	return nalType == h265NALTypeAUD || nalType == h265NALTypePrefixSEI || nalType == h265NALTypeSuffixSEI || (nalType >= 32 && nalType <= 34)
}

func h265NALFirstSlice(nal []byte) bool {
	return len(nal) > h265NALUHeaderSize && (nal[h265NALUHeaderSize]&0x80) != 0
}
