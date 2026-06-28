//go:build windows

package main

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"

	"github.com/kbinani/screenshot"
	"github.com/lxn/win"
)

func init() {
	// Ask Windows for real display coordinates before we start capturing.
	user32 := syscall.NewLazyDLL("user32.dll")
	_, _, _ = user32.NewProc("SetProcessDPIAware").Call()
}

func capturePrimaryDisplayImage() (*image.RGBA, error) {
	if screenshot.NumActiveDisplays() == 0 {
		return nil, fmt.Errorf("no active display detected")
	}

	bounds := screenshot.GetDisplayBounds(0)
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid primary display bounds")
	}

	desktop := win.GetDesktopWindow()
	sourceDC := win.GetDC(desktop)
	if sourceDC == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer win.ReleaseDC(desktop, sourceDC)

	memoryDC := win.CreateCompatibleDC(sourceDC)
	if memoryDC == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer win.DeleteDC(memoryDC)

	var header win.BITMAPINFOHEADER
	header.BiSize = uint32(unsafe.Sizeof(header))
	header.BiWidth = int32(width)
	header.BiHeight = int32(-height)
	header.BiPlanes = 1
	header.BiBitCount = 32
	header.BiCompression = win.BI_RGB

	var bits unsafe.Pointer
	bitmap := win.CreateDIBSection(sourceDC, &header, win.DIB_RGB_COLORS, &bits, 0, 0)
	if bitmap == 0 || bits == nil {
		return nil, fmt.Errorf("CreateDIBSection failed")
	}
	defer win.DeleteObject(win.HGDIOBJ(bitmap))

	previous := win.SelectObject(memoryDC, win.HGDIOBJ(bitmap))
	if previous == 0 {
		return nil, fmt.Errorf("SelectObject failed")
	}
	defer win.SelectObject(memoryDC, previous)

	if !win.BitBlt(
		memoryDC,
		0,
		0,
		int32(width),
		int32(height),
		sourceDC,
		int32(bounds.Min.X),
		int32(bounds.Min.Y),
		win.SRCCOPY|win.CAPTUREBLT,
	) {
		return nil, fmt.Errorf("BitBlt failed")
	}

	raw := unsafe.Slice((*byte)(bits), width*height*4)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for offset := 0; offset < len(raw); offset += 4 {
		img.Pix[offset] = raw[offset+2]
		img.Pix[offset+1] = raw[offset+1]
		img.Pix[offset+2] = raw[offset]
		img.Pix[offset+3] = 255
	}

	return img, nil
}
