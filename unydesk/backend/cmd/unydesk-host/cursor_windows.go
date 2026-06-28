//go:build windows

package main

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/lxn/win"
)

var (
	procGetCursorInfo   = user32DLL.NewProc("GetCursorInfo")
	systemCursorHandles map[string]win.HCURSOR
	systemCursorOnce    sync.Once
)

const cursorShowing = 0x00000001

type localCursorInfo struct {
	CbSize  uint32
	Flags   uint32
	HCursor win.HCURSOR
	Pt      win.POINT
}

func detectHostCursorKind() string {
	info := localCursorInfo{CbSize: uint32(unsafe.Sizeof(localCursorInfo{}))}
	ret, _, _ := procGetCursorInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return "default"
	}
	if info.Flags&cursorShowing == 0 {
		return "default"
	}

	cursors := loadSystemCursorHandles()
	for kind, handle := range cursors {
		if handle != 0 && info.HCursor == handle {
			return kind
		}
	}
	return "default"
}

func loadSystemCursorHandles() map[string]win.HCURSOR {
	systemCursorOnce.Do(func() {
		systemCursorHandles = map[string]win.HCURSOR{
			"default":     win.LoadCursor(0, makeIntResource(win.IDC_ARROW)),
			"text":        win.LoadCursor(0, makeIntResource(win.IDC_IBEAM)),
			"wait":        win.LoadCursor(0, makeIntResource(win.IDC_WAIT)),
			"crosshair":   win.LoadCursor(0, makeIntResource(win.IDC_CROSS)),
			"move":        win.LoadCursor(0, makeIntResource(win.IDC_SIZEALL)),
			"pointer":     win.LoadCursor(0, makeIntResource(win.IDC_HAND)),
			"progress":    win.LoadCursor(0, makeIntResource(win.IDC_APPSTARTING)),
			"help":        win.LoadCursor(0, makeIntResource(win.IDC_HELP)),
			"not-allowed": win.LoadCursor(0, makeIntResource(win.IDC_NO)),
			"ew-resize":   win.LoadCursor(0, makeIntResource(win.IDC_SIZEWE)),
			"ns-resize":   win.LoadCursor(0, makeIntResource(win.IDC_SIZENS)),
			"nwse-resize": win.LoadCursor(0, makeIntResource(win.IDC_SIZENWSE)),
			"nesw-resize": win.LoadCursor(0, makeIntResource(win.IDC_SIZENESW)),
		}
	})
	return systemCursorHandles
}

func makeIntResource(id int) *uint16 {
	return (*uint16)(unsafe.Pointer(uintptr(id)))
}

func normalizeRemoteCursorKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "pointer", "text", "wait", "progress", "crosshair", "move", "help",
		"not-allowed", "ew-resize", "ns-resize", "nwse-resize", "nesw-resize":
		return kind
	default:
		return "default"
	}
}
