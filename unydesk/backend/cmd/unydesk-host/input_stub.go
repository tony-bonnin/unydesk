//go:build !windows

package main

import "fmt"

func injectMouseMove(normalizedX, normalizedY float64) error {
	return fmt.Errorf("native input injection is currently implemented for Windows hosts only")
}

func injectMouseClick(button int, normalizedX, normalizedY float64) error {
	return fmt.Errorf("native input injection is currently implemented for Windows hosts only")
}

func injectMouseButton(button int, normalizedX, normalizedY float64, down bool) error {
	return fmt.Errorf("native input injection is currently implemented for Windows hosts only")
}

func injectMouseWheel(normalizedX, normalizedY float64, deltaY int) error {
	return fmt.Errorf("native input injection is currently implemented for Windows hosts only")
}

func injectKeyEvent(key, code string, down bool) error {
	return fmt.Errorf("native input injection is currently implemented for Windows hosts only")
}

func readClipboardText() (string, error) {
	return "", fmt.Errorf("clipboard integration is currently implemented for Windows hosts only")
}

func writeClipboardText(text string) error {
	return fmt.Errorf("clipboard integration is currently implemented for Windows hosts only")
}
