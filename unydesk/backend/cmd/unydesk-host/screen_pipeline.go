package main

import (
	"bytes"
	"image"
	"math"
	"strings"
	"time"
)

type screenFrame struct {
	ContentType string
	Data        []byte
	X           int
	Y           int
	Width       int
	Height      int
	FullWidth   int
	FullHeight  int
	Codec       byte
	Kind        byte
}

type screenEncoderState struct {
	lastSignature           uint64
	lastUploadAt            time.Time
	lastKeyframeAt          time.Time
	lastErrorReportedAt     time.Time
	lastErrorText           string
	previousFrame           *image.RGBA
	hasSignature            bool
	consecutiveFailures     int
	lastSuccessfulCaptureAt time.Time
}

type screenPipeline struct {
	profile screenPipelineProfile
	capture captureProvider
	encoder screenImageEncoder
	state   screenEncoderState
}

func newScreenPipeline(profile screenPipelineProfile) *screenPipeline {
	return newScreenPipelineWithCapture(profile, defaultCaptureProvider())
}

func newScreenPipelineWithCapture(profile screenPipelineProfile, capture captureProvider) *screenPipeline {
	if capture == nil {
		capture = defaultCaptureProvider()
	}
	return &screenPipeline{
		profile: profile,
		capture: capture,
		encoder: newScreenImageEncoder(profile.Codec),
	}
}

func (p *screenPipeline) capturePrimaryDisplay() (*image.RGBA, error) {
	return capturePrimaryDisplayWithProvider(p.capture)
}

func (p *screenPipeline) nextFrame() (screenFrame, bool, error) {
	var (
		captured *image.RGBA
		err      error
	)
	for attempt := 0; attempt < 3; attempt++ {
		captured, err = p.capturePrimaryDisplay()
		if err == nil {
			break
		}
		if attempt < 2 {
			time.Sleep(p.profile.CaptureRetryDelay)
		}
	}
	if err != nil {
		p.state.consecutiveFailures++
		return screenFrame{}, false, err
	}

	scaled := p.normalizeCapture(captured)
	fullBounds := scaled.Bounds()
	fullWidth := fullBounds.Dx()
	fullHeight := fullBounds.Dy()
	signature := p.frameSignature(scaled)
	if p.state.hasSignature && p.state.lastSignature == signature && time.Since(p.state.lastUploadAt) < p.profile.RefreshUnchangedAfter {
		return screenFrame{}, false, nil
	}

	frameRect := fullBounds
	frameKind := byte(screenWireKindKeyframe)
	if p.state.previousFrame != nil &&
		p.state.previousFrame.Bounds().Dx() == fullWidth &&
		p.state.previousFrame.Bounds().Dy() == fullHeight &&
		time.Since(p.state.lastKeyframeAt) < p.profile.KeyframeInterval {
		if dirtyRect, ok := p.detectDirtyRect(p.state.previousFrame, scaled); ok {
			dirtyArea := dirtyRect.Dx() * dirtyRect.Dy()
			totalArea := maxInt(1, fullWidth*fullHeight)
			if float64(dirtyArea)/float64(totalArea) < p.profile.PatchAreaThreshold {
				frameRect = dirtyRect
				frameKind = screenWireKindPatch
			}
		}
	}

	region := copyRGBARegion(scaled, frameRect)
	encoded, contentType, codec, err := p.encoder.encode(region, frameKind)
	if err != nil {
		return screenFrame{}, false, err
	}

	p.state.lastSignature = signature
	p.state.lastUploadAt = time.Now()
	if frameKind == screenWireKindKeyframe {
		p.state.lastKeyframeAt = p.state.lastUploadAt
	}
	p.state.lastSuccessfulCaptureAt = p.state.lastUploadAt
	p.state.lastErrorText = ""
	p.state.consecutiveFailures = 0
	p.state.previousFrame = scaled
	p.state.hasSignature = true

	return screenFrame{
		ContentType: contentType,
		Data:        encoded,
		X:           frameRect.Min.X,
		Y:           frameRect.Min.Y,
		Width:       frameRect.Dx(),
		Height:      frameRect.Dy(),
		FullWidth:   fullWidth,
		FullHeight:  fullHeight,
		Codec:       codec,
		Kind:        frameKind,
	}, true, nil
}

func (p *screenPipeline) captureErrorMessage(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	friendly := presentableCaptureError(err)
	if friendly == p.state.lastErrorText && time.Since(p.state.lastErrorReportedAt) < p.profile.CaptureErrorCooldown {
		return friendly, false
	}
	p.state.lastErrorReportedAt = time.Now()
	p.state.lastErrorText = friendly
	return friendly, true
}

func presentableCaptureError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "Screen capture is temporarily unavailable. Retrying..."
	}
	if strings.Contains(strings.ToLower(message), "getdibits failed") {
		return "Windows screen capture is temporarily unavailable. Retrying..."
	}
	return message
}

func (p *screenPipeline) normalizeCapture(src *image.RGBA) *image.RGBA {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return src
	}
	if width <= p.profile.StreamMaxEdge && height <= p.profile.StreamMaxEdge {
		return src
	}

	targetWidth := width
	targetHeight := height
	if width >= height {
		targetWidth = p.profile.StreamMaxEdge
		targetHeight = maxInt(1, height*p.profile.StreamMaxEdge/width)
	} else {
		targetHeight = p.profile.StreamMaxEdge
		targetWidth = maxInt(1, width*p.profile.StreamMaxEdge/height)
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	scaleX := float64(width) / float64(targetWidth)
	scaleY := float64(height) / float64(targetHeight)
	for y := 0; y < targetHeight; y++ {
		sourceY := (float64(y)+0.5)*scaleY - 0.5
		if sourceY < 0 {
			sourceY = 0
		}
		y0 := clampInt(int(math.Floor(sourceY)), 0, height-1)
		y1 := clampInt(y0+1, 0, height-1)
		wy := sourceY - float64(y0)
		targetRow := dst.Pix[y*dst.Stride : y*dst.Stride+targetWidth*4]
		for x := 0; x < targetWidth; x++ {
			targetOffset := x * 4
			sourceX := (float64(x)+0.5)*scaleX - 0.5
			if sourceX < 0 {
				sourceX = 0
			}
			x0 := clampInt(int(math.Floor(sourceX)), 0, width-1)
			x1 := clampInt(x0+1, 0, width-1)
			wx := sourceX - float64(x0)

			offset00 := y0*src.Stride + x0*4
			offset10 := y0*src.Stride + x1*4
			offset01 := y1*src.Stride + x0*4
			offset11 := y1*src.Stride + x1*4

			for channel := 0; channel < 4; channel++ {
				top := float64(src.Pix[offset00+channel])*(1-wx) + float64(src.Pix[offset10+channel])*wx
				bottom := float64(src.Pix[offset01+channel])*(1-wx) + float64(src.Pix[offset11+channel])*wx
				value := top*(1-wy) + bottom*wy
				targetRow[targetOffset+channel] = uint8(clampInt(int(math.Round(value)), 0, 255))
			}
		}
	}
	return dst
}

func (p *screenPipeline) detectDirtyRect(previous, current *image.RGBA) (image.Rectangle, bool) {
	if previous == nil || current == nil {
		return image.Rectangle{}, false
	}
	if previous.Bounds().Dx() != current.Bounds().Dx() || previous.Bounds().Dy() != current.Bounds().Dy() {
		return current.Bounds(), true
	}

	width := current.Bounds().Dx()
	height := current.Bounds().Dy()
	minX, minY := width, height
	maxX, maxY := 0, 0
	changed := false

	for tileY := 0; tileY < height; tileY += p.profile.DiffTileSize {
		tileHeight := minInt(p.profile.DiffTileSize, height-tileY)
		for tileX := 0; tileX < width; tileX += p.profile.DiffTileSize {
			tileWidth := minInt(p.profile.DiffTileSize, width-tileX)
			if tilePixelsEqual(previous, current, tileX, tileY, tileWidth, tileHeight) {
				continue
			}
			changed = true
			minX = minInt(minX, tileX)
			minY = minInt(minY, tileY)
			maxX = maxInt(maxX, tileX+tileWidth)
			maxY = maxInt(maxY, tileY+tileHeight)
		}
	}

	if !changed {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX, maxY), true
}

func tilePixelsEqual(previous, current *image.RGBA, x, y, width, height int) bool {
	for row := 0; row < height; row++ {
		offset := (y+row)*previous.Stride + x*4
		length := width * 4
		if !bytes.Equal(previous.Pix[offset:offset+length], current.Pix[offset:offset+length]) {
			return false
		}
	}
	return true
}

func copyRGBARegion(src *image.RGBA, rect image.Rectangle) *image.RGBA {
	rect = rect.Intersect(src.Bounds())
	dst := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	for row := 0; row < rect.Dy(); row++ {
		srcOffset := (rect.Min.Y+row)*src.Stride + rect.Min.X*4
		dstOffset := row * dst.Stride
		copy(dst.Pix[dstOffset:dstOffset+rect.Dx()*4], src.Pix[srcOffset:srcOffset+rect.Dx()*4])
	}
	return dst
}

func (p *screenPipeline) frameSignature(img *image.RGBA) uint64 {
	bounds := img.Bounds()
	width := maxInt(1, bounds.Dx())
	height := maxInt(1, bounds.Dy())
	var hash uint64 = 1469598103934665603

	for row := 0; row < p.profile.SignatureRows; row++ {
		y := row * (height - 1) / maxInt(1, p.profile.SignatureRows-1)
		sourceRow := img.Pix[y*img.Stride:]
		for column := 0; column < p.profile.SignatureColumns; column++ {
			x := column * (width - 1) / maxInt(1, p.profile.SignatureColumns-1)
			offset := x * 4
			red := sourceRow[offset] >> 3
			green := sourceRow[offset+1] >> 2
			blue := sourceRow[offset+2] >> 3
			value := uint64(red)<<11 | uint64(green)<<5 | uint64(blue)
			hash ^= value + uint64(column+1)<<24 + uint64(row+1)<<32
			hash *= 1099511628211
		}
	}

	hash ^= uint64(width)<<1 ^ uint64(height)<<17
	hash *= 1099511628211
	return hash
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampInt(value, lower, upper int) int {
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}
