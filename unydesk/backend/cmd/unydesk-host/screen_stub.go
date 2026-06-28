//go:build !windows

package main

import (
	"image"

	"github.com/kbinani/screenshot"
)

func capturePrimaryDisplayImage() (*image.RGBA, error) {
	return screenshot.CaptureDisplay(0)
}
