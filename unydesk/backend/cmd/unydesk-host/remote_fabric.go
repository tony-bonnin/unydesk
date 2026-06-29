package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type providerStatus string

const (
	providerStatusActive      providerStatus = "active"
	providerStatusAvailable   providerStatus = "available"
	providerStatusUnavailable providerStatus = "unavailable"
	providerStatusPlanned     providerStatus = "planned"
)

type providerCapability struct {
	Name   string
	Kind   string
	Status providerStatus
	Reason string
}

func configuredProvider(envName string) string {
	return strings.ToLower(strings.TrimSpace(os.Getenv(envName)))
}

func providerNames(capabilities []providerCapability) string {
	names := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		names = append(names, capability.Name+":"+string(capability.Status))
	}
	return strings.Join(names, ",")
}

func primaryProviderStatus(capabilities []providerCapability) string {
	if len(capabilities) == 0 {
		return string(providerStatusUnavailable)
	}
	return string(capabilities[0].Status)
}

type captureProvider interface {
	Name() string
	Kind() string
	CapturePrimaryDisplay() (*image.RGBA, error)
}

type screenshotCaptureProvider struct{}

func (screenshotCaptureProvider) Name() string {
	switch runtime.GOOS {
	case "windows":
		return "windows-gdi-screenshot"
	case "darwin":
		return "macos-screenshot-fallback"
	case "linux":
		return "linux-screenshot-fallback"
	default:
		return runtime.GOOS + "-screenshot-fallback"
	}
}

func (screenshotCaptureProvider) Kind() string {
	return "software-capture"
}

func (screenshotCaptureProvider) CapturePrimaryDisplay() (*image.RGBA, error) {
	return capturePrimaryDisplayImage()
}

func defaultCaptureProvider() captureProvider {
	configured := configuredProvider("UNYDESK_CAPTURE_PROVIDER")
	switch configured {
	case "", "auto", "screenshot", "fallback":
		return screenshotCaptureProvider{}
	default:
		fmt.Printf("Remote fabric capture provider %q is not available yet; falling back to screenshot capture\n", configured)
		return screenshotCaptureProvider{}
	}
}

func captureCapabilities(active captureProvider) []providerCapability {
	if active == nil {
		active = defaultCaptureProvider()
	}
	capabilities := []providerCapability{
		{
			Name:   active.Name(),
			Kind:   active.Kind(),
			Status: providerStatusActive,
			Reason: "portable fallback capture provider",
		},
	}
	configured := configuredProvider("UNYDESK_CAPTURE_PROVIDER")
	switch configured {
	case "", "auto", "screenshot", "fallback":
	default:
		capabilities = append(capabilities, providerCapability{
			Name:   configured,
			Kind:   "capture-provider",
			Status: providerStatusUnavailable,
			Reason: "configured capture provider is not implemented in this build",
		})
	}
	switch runtime.GOOS {
	case "windows":
		capabilities = append(capabilities, providerCapability{
			Name:   "windows-graphics-capture",
			Kind:   "gpu-capture",
			Status: providerStatusPlanned,
			Reason: "native provider hook reserved for Windows.Graphics.Capture/DXGI",
		})
	case "linux":
		capabilities = append(capabilities, providerCapability{
			Name:   "linux-pipewire",
			Kind:   "gpu-capture",
			Status: providerStatusPlanned,
			Reason: "native provider hook reserved for PipeWire/Wayland",
		})
	case "darwin":
		capabilities = append(capabilities, providerCapability{
			Name:   "macos-screencapturekit",
			Kind:   "gpu-capture",
			Status: providerStatusPlanned,
			Reason: "native provider hook reserved for ScreenCaptureKit",
		})
	}
	return capabilities
}

func captureProviderNames(capabilities []providerCapability) string {
	return providerNames(capabilities)
}

func capturePrimaryDisplayWithProvider(provider captureProvider) (*image.RGBA, error) {
	if provider == nil {
		provider = defaultCaptureProvider()
	}
	return provider.CapturePrimaryDisplay()
}

type encoderProvider interface {
	Name() string
	Codec() string
	Mode() string
	Capabilities() []providerCapability
}

type ffmpegH264EncoderProvider struct {
	ffmpegPath string
	profile    h264AdaptiveProfile
	available  bool
}

func (provider ffmpegH264EncoderProvider) Name() string {
	if provider.ffmpegPath == "" {
		return "ffmpeg-h264"
	}
	return "ffmpeg-h264:" + filepath.Base(provider.ffmpegPath)
}

func (provider ffmpegH264EncoderProvider) Codec() string {
	return "h264"
}

func (provider ffmpegH264EncoderProvider) Mode() string {
	return provider.profile.Name
}

func (provider ffmpegH264EncoderProvider) Capabilities() []providerCapability {
	status := providerStatusUnavailable
	reason := "ffmpeg binary not found; H.264 video track will fall back to data channel"
	if provider.available {
		status = providerStatusActive
		reason = "current low-latency WebRTC encoder"
	}
	capabilities := []providerCapability{
		{
			Name:   provider.Name(),
			Kind:   "software-h264",
			Status: status,
			Reason: reason,
		},
	}
	configured := configuredProvider("UNYDESK_VIDEO_ENCODER")
	switch configured {
	case "", "auto", "h264", "ffmpeg-h264", "libx264":
	default:
		capabilities = append(capabilities, providerCapability{
			Name:   configured,
			Kind:   "video-encoder",
			Status: providerStatusUnavailable,
			Reason: "configured encoder provider is not implemented in this build",
		})
	}
	for _, candidate := range plannedEncoderCapabilities() {
		capabilities = append(capabilities, candidate)
	}
	return capabilities
}

func plannedEncoderCapabilities() []providerCapability {
	capabilities := []providerCapability{
		{Name: "ffmpeg-av1-opportunistic", Kind: "adaptive-av1", Status: providerStatusAvailable, Reason: "enabled when the viewer offers AV1 and FFmpeg exposes av1_nvenc/qsv/amf/libsvtav1/libaom"},
		{Name: "ffmpeg-h264-hardware", Kind: "hardware-h264", Status: providerStatusPlanned, Reason: "reserved for NVENC/QSV/AMF/VideoToolbox H.264 provider"},
	}
	switch runtime.GOOS {
	case "linux":
		capabilities = append(capabilities, providerCapability{Name: "vaapi-h264-av1", Kind: "hardware-video", Status: providerStatusPlanned, Reason: "reserved for VAAPI provider"})
	case "darwin":
		capabilities = append(capabilities, providerCapability{Name: "videotoolbox-h264", Kind: "hardware-h264", Status: providerStatusPlanned, Reason: "reserved for VideoToolbox provider"})
	case "windows":
		capabilities = append(capabilities, providerCapability{Name: "d3d11-hardware-video", Kind: "hardware-video", Status: providerStatusPlanned, Reason: "reserved for Windows GPU video provider"})
	}
	return capabilities
}

func defaultEncoderProvider(ffmpegPath string, profile h264AdaptiveProfile) encoderProvider {
	configured := configuredProvider("UNYDESK_VIDEO_ENCODER")
	provider := ffmpegH264EncoderProvider{
		ffmpegPath: ffmpegPath,
		profile:    profile,
		available:  strings.TrimSpace(ffmpegPath) != "",
	}
	switch configured {
	case "", "auto", "h264", "ffmpeg-h264", "libx264":
		return provider
	default:
		fmt.Printf("Remote fabric encoder provider %q is not available yet; falling back to ffmpeg-h264\n", configured)
		return provider
	}
}

func encoderProviderNames(capabilities []providerCapability) string {
	return providerNames(capabilities)
}

type transportProvider interface {
	Name() string
	Fallbacks() []string
	Capabilities() []providerCapability
}

type webRTCTransportProvider struct{}

func (webRTCTransportProvider) Name() string {
	return "webrtc-direct"
}

func (webRTCTransportProvider) Fallbacks() []string {
	return []string{"webrtc-datachannel", "websocket"}
}

func (webRTCTransportProvider) Capabilities() []providerCapability {
	capabilities := []providerCapability{
		{Name: "webrtc-direct", Kind: "p2p-udp", Status: providerStatusActive, Reason: "current premium direct transport"},
		{Name: "webrtc-datachannel", Kind: "p2p-sctp", Status: providerStatusAvailable, Reason: "current screen/input fallback transport"},
		{Name: "websocket", Kind: "tcp-fallback", Status: providerStatusAvailable, Reason: "legacy relay fallback"},
		{Name: "webtransport-quic", Kind: "quic-relay", Status: providerStatusPlanned, Reason: "reserved for the monolithic HTTP/3/WebTransport relay path"},
	}
	configured := configuredProvider("UNYDESK_TRANSPORT_PROVIDER")
	switch configured {
	case "", "auto", "webrtc", "webrtc-direct":
	default:
		capabilities = append(capabilities, providerCapability{
			Name:   configured,
			Kind:   "transport-provider",
			Status: providerStatusUnavailable,
			Reason: "configured transport provider is not implemented in this build",
		})
	}
	return capabilities
}

func defaultTransportProvider() transportProvider {
	configured := configuredProvider("UNYDESK_TRANSPORT_PROVIDER")
	switch configured {
	case "", "auto", "webrtc", "webrtc-direct":
		return webRTCTransportProvider{}
	default:
		fmt.Printf("Remote fabric transport provider %q is not available yet; falling back to webrtc-direct\n", configured)
		return webRTCTransportProvider{}
	}
}

func transportProviderNames(capabilities []providerCapability) string {
	return providerNames(capabilities)
}

type remoteFabricPolicy struct {
	CaptureProvider       string
	CaptureKind           string
	CaptureStatus         string
	CaptureCapabilities   string
	EncoderProvider       string
	EncoderCodec          string
	EncoderMode           string
	EncoderStatus         string
	EncoderCapabilities   string
	TransportPrimary      string
	TransportStatus       string
	TransportFallback     string
	TransportCapabilities string
}

func currentRemoteFabricPolicy(ffmpegPath string, profile h264AdaptiveProfile) remoteFabricPolicy {
	return remoteFabricPolicyFor(ffmpegPath, profile, defaultCaptureProvider())
}

func remoteFabricPolicyFor(ffmpegPath string, profile h264AdaptiveProfile, capture captureProvider) remoteFabricPolicy {
	if capture == nil {
		capture = defaultCaptureProvider()
	}
	encoder := defaultEncoderProvider(ffmpegPath, profile)
	transport := defaultTransportProvider()
	captureCaps := captureCapabilities(capture)
	encoderCaps := encoder.Capabilities()
	transportCaps := transport.Capabilities()
	return remoteFabricPolicy{
		CaptureProvider:       capture.Name(),
		CaptureKind:           capture.Kind(),
		CaptureStatus:         primaryProviderStatus(captureCaps),
		CaptureCapabilities:   captureProviderNames(captureCaps),
		EncoderProvider:       encoder.Name(),
		EncoderCodec:          encoder.Codec(),
		EncoderMode:           encoder.Mode(),
		EncoderStatus:         primaryProviderStatus(encoderCaps),
		EncoderCapabilities:   encoderProviderNames(encoderCaps),
		TransportPrimary:      transport.Name(),
		TransportStatus:       primaryProviderStatus(transportCaps),
		TransportFallback:     strings.Join(transport.Fallbacks(), ","),
		TransportCapabilities: transportProviderNames(transportCaps),
	}
}

func (policy remoteFabricPolicy) statusFields() map[string]any {
	return map[string]any{
		"capture_provider":   policy.CaptureProvider,
		"capture_kind":       policy.CaptureKind,
		"capture_status":     policy.CaptureStatus,
		"capture_caps":       policy.CaptureCapabilities,
		"encoder_provider":   policy.EncoderProvider,
		"encoder_codec":      policy.EncoderCodec,
		"encoder_mode":       policy.EncoderMode,
		"encoder_status":     policy.EncoderStatus,
		"encoder_caps":       policy.EncoderCapabilities,
		"transport_primary":  policy.TransportPrimary,
		"transport_status":   policy.TransportStatus,
		"transport_fallback": policy.TransportFallback,
		"transport_caps":     policy.TransportCapabilities,
	}
}
