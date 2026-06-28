//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

const (
	hostPanelShowMessage = win.WM_APP + 40
	hostPanelHideMessage = win.WM_APP + 41
	hostPanelRefreshTick = 1

	hostPanelCommandCopyID      = 2001
	hostPanelCommandToggle      = 2002
	hostPanelCommandOpenAccount = 2003
	hostPanelCommandHide        = 2004
)

var (
	hostPanelWindowClassName = syscall.StringToUTF16Ptr("UnyDeskHostPanelWindow")
	hostPanelWindowTitle     = syscall.StringToUTF16Ptr("UnyDesk Host")
	hostPanelWindowProc      = syscall.NewCallback(handleLocalHostPanelWindowMessage)
	hostPanelWindowHandle    win.HWND
	hostPanelWindowFont      win.HFONT
	hostPanelWindowMonoFont  win.HFONT
	hostPanelWindowTitleFont win.HFONT
	hostPanelControls        hostPanelControlSet
)

type hostPanelControlSet struct {
	eyebrow        win.HWND
	title          win.HWND
	subtitle       win.HWND
	accessIDLabel  win.HWND
	accessIDValue  win.HWND
	connectionText win.HWND
	accessText     win.HWND
	serverText     win.HWND
	heartbeatText  win.HWND
	sessionText    win.HWND
	copyButton     win.HWND
	toggleButton   win.HWND
	accountButton  win.HWND
	hideButton     win.HWND
}

func initLocalHostPanel(hInstance win.HINSTANCE) bool {
	if hostPanelWindowHandle != 0 {
		return true
	}

	class := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   hostPanelWindowProc,
		HInstance:     hInstance,
		HIcon:         loadLocalHostTrayIcon(hInstance),
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW)),
		HbrBackground: win.HBRUSH(win.GetStockObject(win.WHITE_BRUSH)),
		LpszClassName: hostPanelWindowClassName,
		HIconSm:       loadLocalHostTrayIcon(hInstance),
	}
	if win.RegisterClassEx(&class) == 0 {
		return false
	}

	style := uint32(win.WS_OVERLAPPEDWINDOW &^ (win.WS_MAXIMIZEBOX | win.WS_THICKFRAME))
	hwnd := win.CreateWindowEx(
		0,
		hostPanelWindowClassName,
		hostPanelWindowTitle,
		style,
		win.CW_USEDEFAULT,
		win.CW_USEDEFAULT,
		540,
		370,
		0,
		0,
		hInstance,
		nil,
	)
	if hwnd == 0 {
		return false
	}

	hostPanelWindowHandle = hwnd
	win.ShowWindow(hwnd, win.SW_HIDE)
	win.UpdateWindow(hwnd)
	return true
}

func localHostPanelAvailable() bool {
	return hostPanelWindowHandle != 0
}

func showLocalHostPanel() bool {
	if hostPanelWindowHandle == 0 {
		return false
	}
	refreshLocalHostPanel()
	if win.IsIconic(hostPanelWindowHandle) {
		win.ShowWindow(hostPanelWindowHandle, win.SW_RESTORE)
	} else {
		win.ShowWindow(hostPanelWindowHandle, win.SW_SHOW)
	}
	win.SetForegroundWindow(hostPanelWindowHandle)
	win.BringWindowToTop(hostPanelWindowHandle)
	win.SetWindowPos(hostPanelWindowHandle, win.HWND_TOP, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE)
	if hostPanelControls.copyButton != 0 {
		win.SetFocus(hostPanelControls.copyButton)
	}
	return true
}

func destroyLocalHostPanel() {
	if hostPanelWindowHandle != 0 {
		_ = win.DestroyWindow(hostPanelWindowHandle)
		hostPanelWindowHandle = 0
	}
}

func handleLocalHostPanelWindowMessage(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_CREATE:
		createLocalHostPanelFonts()
		createLocalHostPanelControls(hwnd)
		refreshLocalHostPanel()
		win.SetTimer(hwnd, hostPanelRefreshTick, 1000, 0)
		return 0
	case win.WM_TIMER:
		if uintptr(hostPanelRefreshTick) == wParam {
			refreshLocalHostPanel()
			return 0
		}
	case win.WM_COMMAND:
		handleLocalHostPanelCommand(uint16(wParam & 0xffff))
		return 0
	case hostPanelHideMessage:
		win.ShowWindow(hwnd, win.SW_HIDE)
		return 0
	case win.WM_CLOSE:
		win.ShowWindow(hwnd, win.SW_HIDE)
		return 0
	case win.WM_DESTROY:
		win.KillTimer(hwnd, hostPanelRefreshTick)
		hostPanelWindowHandle = 0
		clearLocalHostPanelControls()
		destroyLocalHostPanelFonts()
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func createLocalHostPanelFonts() {
	if hostPanelWindowFont == 0 {
		hostPanelWindowFont = win.HFONT(win.GetStockObject(win.DEFAULT_GUI_FONT))
	}
	if hostPanelWindowTitleFont == 0 {
		hostPanelWindowTitleFont = createHostPanelFont(-26, win.FW_BOLD, "Segoe UI")
	}
	if hostPanelWindowMonoFont == 0 {
		hostPanelWindowMonoFont = createHostPanelFont(-18, win.FW_BOLD, "Consolas")
	}
}

func createHostPanelFont(height int32, weight int32, face string) win.HFONT {
	font := win.LOGFONT{
		LfHeight:         height,
		LfWeight:         weight,
		LfCharSet:        win.DEFAULT_CHARSET,
		LfQuality:        win.CLEARTYPE_QUALITY,
		LfPitchAndFamily: win.VARIABLE_PITCH | win.FF_SWISS,
	}
	if strings.EqualFold(face, "Consolas") {
		font.LfPitchAndFamily = win.FIXED_PITCH
	}
	copy(font.LfFaceName[:], syscall.StringToUTF16(face))
	return win.HFONT(win.CreateFontIndirect(&font))
}

func destroyLocalHostPanelFonts() {
	if hostPanelWindowTitleFont != 0 {
		win.DeleteObject(win.HGDIOBJ(hostPanelWindowTitleFont))
		hostPanelWindowTitleFont = 0
	}
	if hostPanelWindowMonoFont != 0 {
		win.DeleteObject(win.HGDIOBJ(hostPanelWindowMonoFont))
		hostPanelWindowMonoFont = 0
	}
}

func createLocalHostPanelControls(hwnd win.HWND) {
	hostPanelControls.eyebrow = createLocalHostStatic(hwnd, "TRINITY LABS REMOTE ACCESS", 24, 18, 280, 20, hostPanelWindowFont, false)
	hostPanelControls.title = createLocalHostStatic(hwnd, "UnyDesk Host", 24, 42, 300, 34, hostPanelWindowTitleFont, false)
	hostPanelControls.subtitle = createLocalHostStatic(hwnd, "Lightweight host control in the Windows tray.", 24, 76, 360, 18, hostPanelWindowFont, false)
	hostPanelControls.accessIDLabel = createLocalHostStatic(hwnd, "Access ID", 24, 114, 120, 18, hostPanelWindowFont, false)
	hostPanelControls.accessIDValue = createLocalHostEdit(hwnd, "", 24, 136, 336, 34, hostPanelWindowMonoFont)
	hostPanelControls.copyButton = createLocalHostButton(hwnd, "Copy ID", hostPanelCommandCopyID, 374, 136, 132, 34)
	hostPanelControls.connectionText = createLocalHostStatic(hwnd, "", 24, 186, 482, 20, hostPanelWindowFont, true)
	hostPanelControls.accessText = createLocalHostStatic(hwnd, "", 24, 210, 482, 20, hostPanelWindowFont, true)
	hostPanelControls.serverText = createLocalHostStatic(hwnd, "", 24, 234, 482, 20, hostPanelWindowFont, true)
	hostPanelControls.heartbeatText = createLocalHostStatic(hwnd, "", 24, 258, 482, 20, hostPanelWindowFont, true)
	hostPanelControls.sessionText = createLocalHostStatic(hwnd, "", 24, 282, 482, 20, hostPanelWindowFont, true)
	hostPanelControls.toggleButton = createLocalHostButton(hwnd, "Pause Host Access", hostPanelCommandToggle, 24, 314, 160, 34)
	hostPanelControls.accountButton = createLocalHostButton(hwnd, "Open Dashboard", hostPanelCommandOpenAccount, 194, 314, 150, 34)
	hostPanelControls.hideButton = createLocalHostButton(hwnd, "Hide", hostPanelCommandHide, 356, 314, 150, 34)
}

func clearLocalHostPanelControls() {
	hostPanelControls = hostPanelControlSet{}
}

func createLocalHostStatic(hwnd win.HWND, text string, x, y, width, height int32, font win.HFONT, ellipsis bool) win.HWND {
	style := uint32(win.WS_CHILD | win.WS_VISIBLE | win.SS_LEFT)
	if ellipsis {
		style |= win.SS_ENDELLIPSIS
	}
	control := win.CreateWindowEx(
		0,
		syscall.StringToUTF16Ptr("STATIC"),
		syscall.StringToUTF16Ptr(text),
		style,
		x, y, width, height,
		hwnd,
		0,
		win.GetModuleHandle(nil),
		nil,
	)
	if control != 0 && font != 0 {
		win.SendMessage(control, win.WM_SETFONT, uintptr(font), 1)
	}
	return control
}

func createLocalHostEdit(hwnd win.HWND, text string, x, y, width, height int32, font win.HFONT) win.HWND {
	control := win.CreateWindowEx(
		win.WS_EX_CLIENTEDGE,
		syscall.StringToUTF16Ptr("EDIT"),
		syscall.StringToUTF16Ptr(text),
		win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP|win.ES_AUTOHSCROLL|win.ES_READONLY,
		x, y, width, height,
		hwnd,
		0,
		win.GetModuleHandle(nil),
		nil,
	)
	if control != 0 {
		win.SendMessage(control, win.WM_SETFONT, uintptr(font), 1)
		win.SendMessage(control, win.EM_SETREADONLY, 1, 0)
	}
	return control
}

func createLocalHostButton(hwnd win.HWND, text string, commandID uint16, x, y, width, height int32) win.HWND {
	control := win.CreateWindowEx(
		0,
		syscall.StringToUTF16Ptr("BUTTON"),
		syscall.StringToUTF16Ptr(text),
		win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP|win.BS_PUSHBUTTON,
		x, y, width, height,
		hwnd,
		win.HMENU(uintptr(commandID)),
		win.GetModuleHandle(nil),
		nil,
	)
	if control != 0 && hostPanelWindowFont != 0 {
		win.SendMessage(control, win.WM_SETFONT, uintptr(hostPanelWindowFont), 1)
	}
	return control
}

func refreshLocalHostPanel() {
	snapshot := localHostUI.snapshot()
	setLocalHostWindowText(hostPanelControls.accessIDValue, strings.TrimSpace(snapshot.PublicID))
	setLocalHostWindowText(hostPanelControls.connectionText, "Connection  "+formatLocalHostPanelValue(snapshot.ConnectionState, snapshot.ConnectionNote))
	setLocalHostWindowText(hostPanelControls.accessText, "Access      "+formatLocalHostPanelValue(snapshot.AccessState, accessHintText(snapshot.AccessEnabled)+" · "+adminHintText(snapshot.Admin)))
	setLocalHostWindowText(hostPanelControls.serverText, "Server      "+formatLocalHostPanelValue(snapshot.ServerURL, snapshot.Hostname))
	setLocalHostWindowText(hostPanelControls.heartbeatText, "Heartbeat   "+fallbackText(strings.TrimSpace(snapshot.LastHeartbeatAt), "Waiting for first acknowledgement"))
	setLocalHostWindowText(hostPanelControls.sessionText, "Session     "+fallbackText(strings.TrimSpace(snapshot.LastSessionMeta), "No recent session"))
	setLocalHostWindowText(hostPanelControls.toggleButton, toggleLocalHostButtonText(snapshot.AccessEnabled))

	if hostPanelControls.accountButton != 0 {
		win.EnableWindow(hostPanelControls.accountButton, strings.TrimSpace(snapshot.AccountURL) != "")
	}
	if hostPanelControls.copyButton != 0 {
		win.EnableWindow(hostPanelControls.copyButton, strings.TrimSpace(snapshot.PublicID) != "")
	}
}

func formatLocalHostPanelValue(primary string, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return fallbackText(fallback, "Unavailable")
}

func fallbackText(value string, empty string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return empty
	}
	return value
}

func accessHintText(enabled bool) string {
	if enabled {
		return "New client sessions are allowed"
	}
	return "New client sessions are paused"
}

func adminHintText(admin bool) string {
	if admin {
		return "Administrator"
	}
	return "Standard user"
}

func toggleLocalHostButtonText(enabled bool) string {
	if enabled {
		return "Pause Host Access"
	}
	return "Enable Host Access"
}

func setLocalHostWindowText(hwnd win.HWND, text string) {
	if hwnd == 0 {
		return
	}
	win.SendMessage(hwnd, win.WM_SETTEXT, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))))
}

func handleLocalHostPanelCommand(command uint16) {
	switch command {
	case hostPanelCommandCopyID:
		snapshot := localHostUI.snapshot()
		if strings.TrimSpace(snapshot.PublicID) != "" {
			_ = writeClipboardText(snapshot.PublicID)
		}
	case hostPanelCommandToggle:
		snapshot := localHostUI.snapshot()
		setHostAccessEnabled(!snapshot.AccessEnabled)
		refreshLocalHostPanel()
		updateLocalHostTrayTip()
	case hostPanelCommandOpenAccount:
		openLocalHostAccount()
	case hostPanelCommandHide:
		if hostPanelWindowHandle != 0 {
			win.ShowWindow(hostPanelWindowHandle, win.SW_HIDE)
		}
	}
}
