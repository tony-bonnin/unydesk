//go:build windows

package main

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"os/exec"
	"strings"
	"syscall"

	"github.com/kbinani/screenshot"
)

var (
	user32DLL      = syscall.NewLazyDLL("user32.dll")
	procSetCursor  = user32DLL.NewProc("SetCursorPos")
	procMouseEvent = user32DLL.NewProc("mouse_event")
	procKeybdEvent = user32DLL.NewProc("keybd_event")
)

const (
	mouseeventfLeftDown   = 0x0002
	mouseeventfLeftUp     = 0x0004
	mouseeventfRightDown  = 0x0008
	mouseeventfRightUp    = 0x0010
	mouseeventfMiddleDown = 0x0020
	mouseeventfMiddleUp   = 0x0040
	mouseeventfWheel      = 0x0800
	keyeventfKeyUp        = 0x0002
	wheelDelta            = 120
)

func injectMouseMove(normalizedX, normalizedY float64) error {
	bounds, err := primaryDisplayBounds()
	if err != nil {
		return err
	}

	x := bounds.Min.X + int(math.Round(clamp(normalizedX)*float64(max(1, bounds.Dx()-1))))
	y := bounds.Min.Y + int(math.Round(clamp(normalizedY)*float64(max(1, bounds.Dy()-1))))
	if err := setCursorPos(x, y); err != nil {
		return err
	}
	return nil
}

func injectMouseClick(button int, normalizedX, normalizedY float64) error {
	if err := injectMouseButton(button, normalizedX, normalizedY, true); err != nil {
		return err
	}
	return injectMouseButton(button, normalizedX, normalizedY, false)
}

func injectMouseButton(button int, normalizedX, normalizedY float64, down bool) error {
	if err := injectMouseMove(normalizedX, normalizedY); err != nil {
		return err
	}

	flag := mouseButtonFlag(button, down)
	if _, _, err := procMouseEvent.Call(flag, 0, 0, 0, 0); err != syscall.Errno(0) {
		return err
	}
	return nil
}

func injectMouseWheel(normalizedX, normalizedY float64, deltaY int) error {
	if deltaY == 0 {
		return nil
	}
	if err := injectMouseMove(normalizedX, normalizedY); err != nil {
		return err
	}

	step := 1
	if deltaY > 0 {
		step = -1
	}
	data := uintptr(uint32(int32(step * wheelDelta)))
	if _, _, err := procMouseEvent.Call(mouseeventfWheel, 0, 0, data, 0); err != syscall.Errno(0) {
		return err
	}
	return nil
}

func readClipboardText() (string, error) {
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Get-Clipboard -Raw")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("clipboard read failed: %s", message)
	}
	return strings.TrimRight(stdout.String(), "\r\n"), nil
}

func writeClipboardText(text string) error {
	command := exec.Command("cmd.exe", "/c", "clip")
	command.Stdin = strings.NewReader(text)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("clipboard write failed: %s", message)
	}
	return nil
}

func injectKeyEvent(key, code string, down bool) error {
	vk, ok := virtualKeyCode(key, code)
	if !ok {
		return fmt.Errorf("unsupported key code %q key %q", code, key)
	}

	flags := uintptr(0)
	if !down {
		flags = keyeventfKeyUp
	}
	if _, _, err := procKeybdEvent.Call(uintptr(vk), 0, flags, 0); err != syscall.Errno(0) {
		return err
	}
	return nil
}

func setCursorPos(x, y int) error {
	ret, _, err := procSetCursor.Call(uintptr(x), uintptr(y))
	if ret == 0 && err != syscall.Errno(0) {
		return err
	}
	if ret == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}
	return nil
}

func primaryDisplayBounds() (image.Rectangle, error) {
	rect := screenshot.GetDisplayBounds(0)
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		return rect, fmt.Errorf("primary display bounds unavailable")
	}
	return rect, nil
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func virtualKeyCode(key, code string) (byte, bool) {
	switch strings.TrimSpace(code) {
	case "Enter":
		return 0x0D, true
	case "Escape":
		return 0x1B, true
	case "Backspace":
		return 0x08, true
	case "Tab":
		return 0x09, true
	case "Space":
		return 0x20, true
	case "ArrowLeft":
		return 0x25, true
	case "ArrowUp":
		return 0x26, true
	case "ArrowRight":
		return 0x27, true
	case "ArrowDown":
		return 0x28, true
	case "ShiftLeft", "ShiftRight":
		return 0x10, true
	case "ControlLeft", "ControlRight":
		return 0x11, true
	case "AltLeft", "AltRight":
		return 0x12, true
	case "MetaLeft", "MetaRight":
		return 0x5B, true
	case "Delete":
		return 0x2E, true
	case "Home":
		return 0x24, true
	case "End":
		return 0x23, true
	case "PageUp":
		return 0x21, true
	case "PageDown":
		return 0x22, true
	case "Insert":
		return 0x2D, true
	case "Minus":
		return 0xBD, true
	case "Equal":
		return 0xBB, true
	case "BracketLeft":
		return 0xDB, true
	case "BracketRight":
		return 0xDD, true
	case "Backslash":
		return 0xDC, true
	case "Semicolon":
		return 0xBA, true
	case "Quote":
		return 0xDE, true
	case "Comma":
		return 0xBC, true
	case "Period":
		return 0xBE, true
	case "Slash":
		return 0xBF, true
	case "Backquote":
		return 0xC0, true
	}

	if strings.HasPrefix(code, "Key") && len(code) == 4 {
		ch := code[3]
		if ch >= 'A' && ch <= 'Z' {
			return ch, true
		}
	}
	if strings.HasPrefix(code, "Digit") && len(code) == 6 {
		ch := code[5]
		if ch >= '0' && ch <= '9' {
			return ch, true
		}
	}
	if len(key) == 1 {
		ch := key[0]
		if ch >= 'a' && ch <= 'z' {
			return ch - 32, true
		}
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			return ch, true
		}
	}
	return 0, false
}

func mouseButtonFlag(button int, down bool) uintptr {
	switch button {
	case 1:
		if down {
			return mouseeventfMiddleDown
		}
		return mouseeventfMiddleUp
	case 2:
		if down {
			return mouseeventfRightDown
		}
		return mouseeventfRightUp
	default:
		if down {
			return mouseeventfLeftDown
		}
		return mouseeventfLeftUp
	}
}
