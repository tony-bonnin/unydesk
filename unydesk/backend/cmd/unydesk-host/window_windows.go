//go:build windows

package main

import (
	"net/url"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

const (
	hostPanelShowMessage = win.WM_APP + 40
	hostPanelHideMessage = win.WM_APP + 41
	hostPanelRefreshTick = 1

	hostPanelCommandCopyID       = 2001
	hostPanelCommandToggle       = 2002
	hostPanelCommandOpenAccount  = 2003
	hostPanelCommandHide         = 2004
	hostPanelCommandApprove      = 2005
	hostPanelCommandDeny         = 2006
	hostPanelCommandCopyPassword = 2007
)

var (
	hostPanelWindowClassName = syscall.StringToUTF16Ptr("UnyDeskHostPanelWindow")
	hostPanelWindowTitle     = syscall.StringToUTF16Ptr("UnyDesk Host")
	hostPanelWindowProc      = syscall.NewCallback(handleLocalHostPanelWindowMessage)
	hostPanelButtonProc      = syscall.NewCallback(handleLocalHostPanelButtonMessage)
	hostPanelWindowHandle    win.HWND

	hostPanelWindowSmallFont  win.HFONT
	hostPanelWindowBodyFont   win.HFONT
	hostPanelWindowMonoFont   win.HFONT
	hostPanelWindowTitleFont  win.HFONT
	hostPanelWindowButtonFont win.HFONT

	hostPanelBackgroundBrush win.HBRUSH
	hostPanelCardBrush       win.HBRUSH
	hostPanelCardSoftBrush   win.HBRUSH
	hostPanelAccentBrush     win.HBRUSH
	hostPanelAccentSoftBrush win.HBRUSH
	hostPanelBorderBrush     win.HBRUSH

	hostPanelControls        hostPanelControlSet
	hostPanelButtonPrevProcs = map[win.HWND]uintptr{}
)

var (
	hostPanelColorBackground = win.RGB(239, 243, 252)
	hostPanelColorCard       = win.RGB(255, 255, 255)
	hostPanelColorCardSoft   = win.RGB(247, 249, 255)
	hostPanelColorAccent     = win.RGB(39, 34, 214)
	hostPanelColorAccentSoft = win.RGB(228, 235, 255)
	hostPanelColorText       = win.RGB(28, 33, 64)
	hostPanelColorMuted      = win.RGB(103, 112, 155)
	hostPanelColorBorder     = win.RGB(232, 237, 250)
	hostPanelColorDanger     = win.RGB(204, 85, 85)
	hostPanelColorWhite      = win.RGB(255, 255, 255)
	hostPanelColorDisabled   = win.RGB(170, 177, 204)
)

type hostPanelControlSet struct {
	eyebrow            win.HWND
	title              win.HWND
	titleHostname      win.HWND
	subtitle           win.HWND
	accessIDLabel      win.HWND
	accessIDValue      win.HWND
	passwordLabel      win.HWND
	passwordValue      win.HWND
	connectionText     win.HWND
	accessText         win.HWND
	serverText         win.HWND
	heartbeatText      win.HWND
	sessionText        win.HWND
	copyButton         win.HWND
	copyPasswordButton win.HWND
	approveButton      win.HWND
	denyButton         win.HWND
	toggleButton       win.HWND
	accountButton      win.HWND
	hideButton         win.HWND
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
		HbrBackground: 0,
		LpszClassName: hostPanelWindowClassName,
		HIconSm:       loadLocalHostTrayIcon(hInstance),
		Style:         win.CS_HREDRAW | win.CS_VREDRAW,
	}
	if win.RegisterClassEx(&class) == 0 {
		return false
	}

	style := uint32(win.WS_CAPTION | win.WS_SYSMENU | win.WS_MINIMIZEBOX)
	style = uint32(win.WS_POPUP)
	hwnd := win.CreateWindowEx(
		win.WS_EX_TOOLWINDOW,
		hostPanelWindowClassName,
		hostPanelWindowTitle,
		style,
		win.CW_USEDEFAULT,
		win.CW_USEDEFAULT,
		548,
		560,
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
	positionLocalHostPanel()
	wasVisible := win.IsWindowVisible(hostPanelWindowHandle)
	if win.IsIconic(hostPanelWindowHandle) {
		win.ShowWindow(hostPanelWindowHandle, win.SW_RESTORE)
	} else if !wasVisible {
		win.ShowWindow(hostPanelWindowHandle, win.SW_SHOW)
		win.UpdateWindow(hostPanelWindowHandle)
	} else {
		win.ShowWindow(hostPanelWindowHandle, win.SW_SHOWNA)
	}
	win.SetForegroundWindow(hostPanelWindowHandle)
	win.BringWindowToTop(hostPanelWindowHandle)
	win.SetWindowPos(hostPanelWindowHandle, win.HWND_TOPMOST, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE)
	snapshot := localHostUI.snapshot()
	if snapshot.PendingApproval && hostPanelControls.approveButton != 0 {
		win.SetFocus(hostPanelControls.approveButton)
	} else if hostPanelControls.copyButton != 0 {
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
		createLocalHostPanelTheme()
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
	case win.WM_SETCURSOR:
		if applyLocalHostPanelPointerCursor() {
			return 1
		}
	case win.WM_DRAWITEM:
		if lParam != 0 {
			drawLocalHostPanelButton((*win.DRAWITEMSTRUCT)(unsafe.Pointer(lParam)))
			return 1
		}
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_PAINT:
		paintLocalHostPanel(hwnd)
		return 0
	case win.WM_CTLCOLORSTATIC:
		return handleLocalHostPanelStaticColor(win.HDC(wParam), win.HWND(lParam))
	case hostPanelHideMessage:
		win.ShowWindow(hwnd, win.SW_HIDE)
		return 0
	case win.WM_CLOSE:
		dismissPendingHostApprovalPresentation()
		win.ShowWindow(hwnd, win.SW_HIDE)
		return 0
	case win.WM_DESTROY:
		win.KillTimer(hwnd, hostPanelRefreshTick)
		hostPanelWindowHandle = 0
		clearLocalHostPanelControls()
		destroyLocalHostPanelTheme()
		destroyLocalHostPanelFonts()
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func createLocalHostPanelFonts() {
	if hostPanelWindowSmallFont == 0 {
		hostPanelWindowSmallFont = createHostPanelFont(-13, win.FW_SEMIBOLD, "Segoe UI")
	}
	if hostPanelWindowBodyFont == 0 {
		hostPanelWindowBodyFont = createHostPanelFont(-15, win.FW_NORMAL, "Segoe UI")
	}
	if hostPanelWindowTitleFont == 0 {
		hostPanelWindowTitleFont = createHostPanelFont(-24, win.FW_BOLD, "Segoe UI Semibold")
	}
	if hostPanelWindowMonoFont == 0 {
		hostPanelWindowMonoFont = createHostPanelFont(-20, win.FW_BOLD, "Consolas")
	}
	if hostPanelWindowButtonFont == 0 {
		hostPanelWindowButtonFont = createHostPanelFont(-15, win.FW_SEMIBOLD, "Segoe UI Semibold")
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
		font.LfPitchAndFamily = win.FIXED_PITCH | win.FF_MODERN
	}
	copy(font.LfFaceName[:], syscall.StringToUTF16(face))
	return win.HFONT(win.CreateFontIndirect(&font))
}

func destroyLocalHostPanelFonts() {
	if hostPanelWindowButtonFont != 0 {
		win.DeleteObject(win.HGDIOBJ(hostPanelWindowButtonFont))
		hostPanelWindowButtonFont = 0
	}
	if hostPanelWindowTitleFont != 0 {
		win.DeleteObject(win.HGDIOBJ(hostPanelWindowTitleFont))
		hostPanelWindowTitleFont = 0
	}
	if hostPanelWindowMonoFont != 0 {
		win.DeleteObject(win.HGDIOBJ(hostPanelWindowMonoFont))
		hostPanelWindowMonoFont = 0
	}
	if hostPanelWindowBodyFont != 0 {
		win.DeleteObject(win.HGDIOBJ(hostPanelWindowBodyFont))
		hostPanelWindowBodyFont = 0
	}
	if hostPanelWindowSmallFont != 0 {
		win.DeleteObject(win.HGDIOBJ(hostPanelWindowSmallFont))
		hostPanelWindowSmallFont = 0
	}
}

func createLocalHostPanelTheme() {
	if hostPanelBackgroundBrush == 0 {
		hostPanelBackgroundBrush = createHostPanelBrush(hostPanelColorBackground)
	}
	if hostPanelCardBrush == 0 {
		hostPanelCardBrush = createHostPanelBrush(hostPanelColorCard)
	}
	if hostPanelCardSoftBrush == 0 {
		hostPanelCardSoftBrush = createHostPanelBrush(hostPanelColorCardSoft)
	}
	if hostPanelAccentBrush == 0 {
		hostPanelAccentBrush = createHostPanelBrush(hostPanelColorAccent)
	}
	if hostPanelAccentSoftBrush == 0 {
		hostPanelAccentSoftBrush = createHostPanelBrush(hostPanelColorAccentSoft)
	}
	if hostPanelBorderBrush == 0 {
		hostPanelBorderBrush = createHostPanelBrush(hostPanelColorBorder)
	}
}

func destroyLocalHostPanelTheme() {
	deleteHostPanelBrush(&hostPanelBorderBrush)
	deleteHostPanelBrush(&hostPanelAccentSoftBrush)
	deleteHostPanelBrush(&hostPanelAccentBrush)
	deleteHostPanelBrush(&hostPanelCardSoftBrush)
	deleteHostPanelBrush(&hostPanelCardBrush)
	deleteHostPanelBrush(&hostPanelBackgroundBrush)
}

func createHostPanelBrush(color win.COLORREF) win.HBRUSH {
	brush := win.LOGBRUSH{
		LbStyle: win.BS_SOLID,
		LbColor: color,
	}
	return win.CreateBrushIndirect(&brush)
}

func deleteHostPanelBrush(brush *win.HBRUSH) {
	if brush == nil || *brush == 0 {
		return
	}
	win.DeleteObject(win.HGDIOBJ(*brush))
	*brush = 0
}

func createLocalHostPanelControls(hwnd win.HWND) {
	hostPanelControls.eyebrow = createLocalHostStatic(hwnd, "", 44, 28, 420, 18, hostPanelWindowSmallFont, false)
	hostPanelControls.title = createLocalHostStatic(hwnd, hostText("window.host_prefix"), 44, 52, 164, 30, hostPanelWindowTitleFont, false)
	hostPanelControls.titleHostname = createLocalHostStatic(hwnd, "", 212, 52, 230, 30, hostPanelWindowTitleFont, true)
	hostPanelControls.subtitle = createLocalHostStatic(hwnd, hostText("window.subtitle"), 44, 92, 430, 48, hostPanelWindowBodyFont, false)
	hostPanelControls.accessIDLabel = createLocalHostStatic(hwnd, hostText("label.access_id"), 44, 174, 124, 22, hostPanelWindowSmallFont, false)
	hostPanelControls.accessIDValue = createLocalHostStatic(hwnd, "", 178, 173, 244, 30, hostPanelWindowMonoFont, true)
	hostPanelControls.copyButton = createLocalHostButton(hwnd, "", hostPanelCommandCopyID, 458, 164, 36, 36)
	hostPanelControls.passwordLabel = createLocalHostStatic(hwnd, hostText("label.password"), 44, 212, 124, 22, hostPanelWindowSmallFont, false)
	hostPanelControls.passwordValue = createLocalHostStatic(hwnd, "", 178, 211, 244, 26, hostPanelWindowBodyFont, true)
	hostPanelControls.copyPasswordButton = createLocalHostButton(hwnd, "", hostPanelCommandCopyPassword, 458, 202, 36, 36)
	hostPanelControls.connectionText = createLocalHostStatic(hwnd, "", 44, 286, 450, 22, hostPanelWindowBodyFont, true)
	hostPanelControls.accessText = createLocalHostStatic(hwnd, "", 44, 314, 450, 22, hostPanelWindowBodyFont, true)
	hostPanelControls.serverText = createLocalHostStatic(hwnd, "", 44, 342, 450, 22, hostPanelWindowBodyFont, true)
	hostPanelControls.heartbeatText = createLocalHostStatic(hwnd, "", 44, 370, 450, 22, hostPanelWindowBodyFont, true)
	hostPanelControls.sessionText = createLocalHostStatic(hwnd, "", 44, 370, 450, 22, hostPanelWindowBodyFont, true)
	hostPanelControls.approveButton = createLocalHostButton(hwnd, hostText("btn.allow"), hostPanelCommandApprove, 106, 478, 150, 40)
	hostPanelControls.denyButton = createLocalHostButton(hwnd, hostText("btn.deny"), hostPanelCommandDeny, 286, 478, 150, 40)
	hostPanelControls.toggleButton = createLocalHostButton(hwnd, toggleLocalHostButtonText(true), hostPanelCommandToggle, 106, 478, 150, 40)
	hostPanelControls.accountButton = createLocalHostButton(hwnd, hostText("btn.open_dashboard"), hostPanelCommandOpenAccount, 286, 478, 150, 40)
	hostPanelControls.hideButton = createLocalHostButton(hwnd, "", hostPanelCommandHide, 492, 16, 30, 30)
}

func clearLocalHostPanelControls() {
	hostPanelControls = hostPanelControlSet{}
	hostPanelButtonPrevProcs = map[win.HWND]uintptr{}
}

func createLocalHostStatic(hwnd win.HWND, text string, x, y, width, height int32, font win.HFONT, ellipsis bool) win.HWND {
	return createLocalHostStaticAligned(hwnd, text, x, y, width, height, font, ellipsis, win.SS_LEFT)
}

func createLocalHostStaticAligned(hwnd win.HWND, text string, x, y, width, height int32, font win.HFONT, ellipsis bool, alignment uint32) win.HWND {
	style := uint32(win.WS_CHILD | win.WS_VISIBLE | alignment)
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

func createLocalHostButton(hwnd win.HWND, text string, commandID uint16, x, y, width, height int32) win.HWND {
	control := win.CreateWindowEx(
		0,
		syscall.StringToUTF16Ptr("BUTTON"),
		syscall.StringToUTF16Ptr(text),
		win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP|win.BS_OWNERDRAW,
		x, y, width, height,
		hwnd,
		win.HMENU(uintptr(commandID)),
		win.GetModuleHandle(nil),
		nil,
	)
	if control != 0 && hostPanelWindowButtonFont != 0 {
		win.SendMessage(control, win.WM_SETFONT, uintptr(hostPanelWindowButtonFont), 1)
	}
	if control != 0 {
		installLocalHostButtonSubclass(control)
	}
	return control
}

func installLocalHostButtonSubclass(hwnd win.HWND) {
	if hwnd == 0 {
		return
	}
	if _, ok := hostPanelButtonPrevProcs[hwnd]; ok {
		return
	}
	previous := win.SetWindowLongPtr(hwnd, win.GWLP_WNDPROC, hostPanelButtonProc)
	if previous != 0 {
		hostPanelButtonPrevProcs[hwnd] = previous
	}
}

func handleLocalHostPanelButtonMessage(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == win.WM_SETCURSOR && win.IsWindowEnabled(hwnd) && win.IsWindowVisible(hwnd) {
		cursor := loadSystemCursorHandles()["pointer"]
		if cursor != 0 {
			win.SetCursor(cursor)
			return 1
		}
	}
	if previous, ok := hostPanelButtonPrevProcs[hwnd]; ok && previous != 0 {
		return win.CallWindowProc(previous, hwnd, msg, wParam, lParam)
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func refreshLocalHostPanel() {
	snapshot := localHostUI.snapshot()
	connectionState := localizedLocalHostConnectionState(snapshot.ConnectionState)
	accessState := localizedLocalHostAccessState(snapshot.AccessState, snapshot.AccessEnabled)
	setLocalHostWindowText(hostPanelWindowHandle, hostText("window.title"))
	setLocalHostWindowText(hostPanelControls.eyebrow, "")
	setLocalHostWindowText(hostPanelControls.title, hostText("window.host_prefix"))
	titleHostname := localHostPanelSourceName(snapshot)
	setLocalHostWindowText(hostPanelControls.titleHostname, titleHostname)
	subtitle := hostText("window.subtitle")
	if snapshot.PendingApproval {
		viewer := fallbackText(strings.TrimSpace(snapshot.PendingViewer), hostText("viewer.a_client"))
		subtitle = hostTextf("window.subtitle_approval", viewer)
	}
	setLocalHostWindowText(hostPanelControls.subtitle, subtitle)
	setLocalHostWindowText(hostPanelControls.accessIDLabel, hostText("label.access_id"))
	setLocalHostWindowText(hostPanelControls.accessIDValue, fallbackText(strings.TrimSpace(snapshot.PublicID), "—"))
	setLocalHostWindowText(hostPanelControls.copyButton, "")
	setLocalHostWindowText(hostPanelControls.copyPasswordButton, "")
	setLocalHostWindowText(hostPanelControls.passwordLabel, hostText("label.password"))
	setLocalHostWindowText(hostPanelControls.passwordValue, fallbackText(strings.TrimSpace(snapshot.AccessPassword), hostText("status.waiting_local_password")))
	setLocalHostWindowText(hostPanelControls.connectionText, hostText("label.connection")+"  "+formatLocalHostPanelValue(connectionState, snapshot.ConnectionNote))
	setLocalHostWindowText(hostPanelControls.accessText, hostText("label.access")+"  "+formatLocalHostPanelValue(accessState, accessHintText(snapshot.AccessEnabled)+" · "+adminHintText(snapshot.Admin)))
	setLocalHostWindowText(hostPanelControls.serverText, hostText("label.server")+"  "+formatLocalHostPanelValue(snapshot.ServerURL, snapshot.Hostname))
	setLocalHostWindowText(hostPanelControls.heartbeatText, "")
	setLocalHostWindowText(hostPanelControls.sessionText, hostText("label.session")+"  "+fallbackText(strings.TrimSpace(snapshot.LastSessionMeta), hostText("status.no_recent_session")))
	setLocalHostWindowText(hostPanelControls.approveButton, hostText("btn.allow"))
	setLocalHostWindowText(hostPanelControls.denyButton, hostText("btn.deny"))
	setLocalHostWindowText(hostPanelControls.toggleButton, toggleLocalHostButtonText(snapshot.AccessEnabled))
	setLocalHostWindowText(hostPanelControls.accountButton, hostText("btn.open_dashboard"))
	setLocalHostWindowText(hostPanelControls.hideButton, "")

	pending := snapshot.PendingApproval
	setLocalHostButtonVisibility(hostPanelControls.approveButton, pending)
	setLocalHostButtonVisibility(hostPanelControls.denyButton, pending)
	setLocalHostButtonVisibility(hostPanelControls.toggleButton, !pending)
	setLocalHostButtonVisibility(hostPanelControls.accountButton, !pending)
	setLocalHostButtonVisibility(hostPanelControls.hideButton, true)
	setLocalHostButtonVisibility(hostPanelControls.heartbeatText, false)
	win.EnableWindow(hostPanelControls.approveButton, pending)
	win.EnableWindow(hostPanelControls.denyButton, pending)
	win.EnableWindow(hostPanelControls.toggleButton, true)
	win.EnableWindow(hostPanelControls.accountButton, strings.TrimSpace(snapshot.AccountURL) != "")
	win.EnableWindow(hostPanelControls.hideButton, true)
	win.EnableWindow(hostPanelControls.copyButton, strings.TrimSpace(snapshot.PublicID) != "")
	win.EnableWindow(hostPanelControls.copyPasswordButton, strings.TrimSpace(snapshot.AccessPassword) != "")

	if hostPanelWindowHandle != 0 {
		win.InvalidateRect(hostPanelWindowHandle, nil, true)
	}
}

func formatLocalHostPanelValue(primary string, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return fallbackText(fallback, hostText("status.unavailable"))
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
		return hostText("access.allowed")
	}
	return hostText("access.paused_hint")
}

func localizedLocalHostConnectionState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "starting":
		return hostText("status.starting")
	case "link required":
		return hostText("status.link_required")
	case "provisioning required":
		return hostText("status.provisioning_required")
	case "connecting":
		return hostText("status.connecting")
	case "connected":
		return hostText("status.connected")
	case "approval needed":
		return hostText("status.approval_needed")
	case "reconnecting":
		return hostText("status.reconnecting")
	default:
		return strings.TrimSpace(value)
	}
}

func localizedLocalHostAccessState(value string, enabled bool) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "enabled", "active":
		return hostText("access.enabled")
	case "paused":
		return hostText("access.paused")
	}
	if enabled {
		return hostText("access.enabled")
	}
	return hostText("access.paused")
}

func adminHintText(admin bool) string {
	if admin {
		return hostText("admin.administrator")
	}
	return hostText("admin.standard")
}

func toggleLocalHostButtonText(enabled bool) string {
	if enabled {
		return hostText("btn.pause_host_access")
	}
	return hostText("btn.enable_host_access")
}

func setLocalHostWindowText(hwnd win.HWND, text string) {
	if hwnd == 0 {
		return
	}
	win.SendMessage(hwnd, win.WM_SETTEXT, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))))
}

func localHostWindowText(hwnd win.HWND) string {
	if hwnd == 0 {
		return ""
	}
	buffer := make([]uint16, 256)
	win.SendMessage(hwnd, win.WM_GETTEXT, uintptr(len(buffer)), uintptr(unsafe.Pointer(&buffer[0])))
	return strings.TrimSpace(syscall.UTF16ToString(buffer))
}

func setLocalHostButtonVisibility(hwnd win.HWND, visible bool) {
	if hwnd == 0 {
		return
	}
	if visible {
		win.ShowWindow(hwnd, win.SW_SHOW)
		return
	}
	win.ShowWindow(hwnd, win.SW_HIDE)
}

func paintLocalHostPanel(hwnd win.HWND) {
	var ps win.PAINTSTRUCT
	hdc := win.BeginPaint(hwnd, &ps)
	if hdc == 0 {
		return
	}
	defer win.EndPaint(hwnd, &ps)

	var client win.RECT
	win.GetClientRect(hwnd, &client)
	drawHostPanelRoundRect(hdc, &client, hostPanelBackgroundBrush, hostPanelBackgroundBrush, 1)

	headerCard := win.RECT{Left: 24, Top: 150, Right: 514, Bottom: 258}
	statusCard := win.RECT{Left: 24, Top: 274, Right: 514, Bottom: 446}
	actionCard := win.RECT{Left: 24, Top: 466, Right: 514, Bottom: 544}

	drawHostPanelRoundRect(hdc, &headerCard, hostPanelCardBrush, 0, 16)
	drawHostPanelRoundRect(hdc, &statusCard, hostPanelAccentSoftBrush, 0, 16)
	drawHostPanelRoundRect(hdc, &actionCard, hostPanelBackgroundBrush, 0, 16)

}

func handleLocalHostPanelStaticColor(hdc win.HDC, control win.HWND) uintptr {
	if hdc == 0 {
		return uintptr(win.GetStockObject(win.NULL_BRUSH))
	}
	win.SetBkMode(hdc, win.TRANSPARENT)
	textColor := hostPanelColorText
	switch control {
	case hostPanelControls.eyebrow:
		textColor = hostPanelColorAccent
	case hostPanelControls.title:
		textColor = hostPanelColorText
	case hostPanelControls.titleHostname:
		textColor = hostPanelColorAccent
	case hostPanelControls.subtitle:
		textColor = hostPanelColorMuted
	case hostPanelControls.accessIDLabel:
		textColor = hostPanelColorMuted
	case hostPanelControls.accessIDValue:
		textColor = hostPanelColorAccent
	case hostPanelControls.passwordLabel:
		textColor = hostPanelColorMuted
	case hostPanelControls.passwordValue:
		textColor = hostPanelColorText
	default:
		textColor = hostPanelColorMuted
	}
	win.SetTextColor(hdc, textColor)
	return uintptr(win.GetStockObject(win.NULL_BRUSH))
}

func drawLocalHostPanelButton(item *win.DRAWITEMSTRUCT) {
	if item == nil || item.HwndItem == 0 || item.HDC == 0 {
		return
	}
	rect := item.RcItem
	commandID := uint16(item.CtlID)
	pressed := item.ItemState&win.ODS_SELECTED != 0
	disabled := item.ItemState&win.ODS_DISABLED != 0
	focused := item.ItemState&win.ODS_FOCUS != 0

	background := hostPanelColorCard
	textColor := hostPanelColorText
	radius := int32(12)

	switch commandID {
	case hostPanelCommandApprove:
		background = hostPanelColorAccent
		textColor = hostPanelColorWhite
	case hostPanelCommandCopyID, hostPanelCommandCopyPassword, hostPanelCommandToggle:
		background = hostPanelColorAccentSoft
		textColor = hostPanelColorAccent
	case hostPanelCommandOpenAccount:
		background = hostPanelColorAccentSoft
		textColor = hostPanelColorAccent
	case hostPanelCommandDeny:
		background = blendHostPanelColor(hostPanelColorDanger, hostPanelColorWhite, 0.84)
		textColor = hostPanelColorDanger
	case hostPanelCommandHide:
		background = hostPanelColorDanger
		textColor = hostPanelColorWhite
		radius = 10
	}

	if disabled {
		background = hostPanelColorCardSoft
		textColor = hostPanelColorDisabled
	} else if pressed {
		background = blendHostPanelColor(background, hostPanelColorText, 0.08)
	}

	fillBrush := createHostPanelBrush(background)
	defer deleteHostPanelBrush(&fillBrush)

	drawRect := rect
	if commandID == hostPanelCommandHide {
		drawRect.Left += 1
		drawRect.Top += 1
		drawRect.Right -= 1
		drawRect.Bottom -= 1
	}
	drawHostPanelRoundRect(item.HDC, &drawRect, fillBrush, 0, radius)

	label := localHostWindowText(item.HwndItem)
	if commandID == hostPanelCommandHide {
		penColor := textColor
		if disabled {
			penColor = hostPanelColorDisabled
		}
		pen := createHostPanelPen(penColor)
		oldPen := win.SelectObject(item.HDC, win.HGDIOBJ(pen))
		oldBrush := win.SelectObject(item.HDC, win.GetStockObject(win.NULL_BRUSH))
		centerX := ((drawRect.Left + drawRect.Right) / 2) - 1
		centerY := ((drawRect.Top + drawRect.Bottom) / 2) - 2
		half := int32(4)
		win.MoveToEx(item.HDC, int(centerX-half), int(centerY-half), nil)
		win.LineTo(item.HDC, centerX+half+1, centerY+half+1)
		win.MoveToEx(item.HDC, int(centerX+half), int(centerY-half), nil)
		win.LineTo(item.HDC, centerX-half-1, centerY+half+1)
		if oldBrush != 0 {
			win.SelectObject(item.HDC, oldBrush)
		}
		if oldPen != 0 {
			current := win.SelectObject(item.HDC, oldPen)
			if current != 0 {
				win.DeleteObject(current)
			}
		}
		return
	}
	if commandID == hostPanelCommandCopyID || commandID == hostPanelCommandCopyPassword {
		drawLocalHostPanelCopyGlyph(item.HDC, rect, textColor, disabled)
		return
	}
	if label == "" {
		return
	}

	textRect := rect
	textRect.Left += 12
	textRect.Right -= 12
	win.SetBkMode(item.HDC, win.TRANSPARENT)
	win.SetTextColor(item.HDC, textColor)
	font := hostPanelWindowButtonFont
	if font == 0 {
		font = hostPanelWindowBodyFont
	}
	oldFont := win.SelectObject(item.HDC, win.HGDIOBJ(font))
	defer win.SelectObject(item.HDC, oldFont)
	if focused && !pressed {
		win.SetTextColor(item.HDC, blendHostPanelColor(textColor, hostPanelColorWhite, 0.18))
	}
	win.DrawTextEx(
		item.HDC,
		syscall.StringToUTF16Ptr(label),
		-1,
		&textRect,
		win.DT_CENTER|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_END_ELLIPSIS,
		nil,
	)
}

func drawLocalHostPanelCopyGlyph(hdc win.HDC, rect win.RECT, color win.COLORREF, disabled bool) {
	penColor := color
	if disabled {
		penColor = hostPanelColorDisabled
	}
	pen := createHostPanelPen(penColor)
	oldPen := win.SelectObject(hdc, win.HGDIOBJ(pen))
	oldBrush := win.SelectObject(hdc, win.GetStockObject(win.NULL_BRUSH))

	back := win.RECT{
		Left:   rect.Left + 11,
		Top:    rect.Top + 9,
		Right:  rect.Left + 23,
		Bottom: rect.Top + 22,
	}
	front := win.RECT{
		Left:   rect.Left + 14,
		Top:    rect.Top + 13,
		Right:  rect.Left + 26,
		Bottom: rect.Top + 26,
	}
	win.RoundRect(hdc, back.Left, back.Top, back.Right, back.Bottom, 3, 3)
	win.RoundRect(hdc, front.Left, front.Top, front.Right, front.Bottom, 3, 3)

	if oldBrush != 0 {
		win.SelectObject(hdc, oldBrush)
	}
	if oldPen != 0 {
		current := win.SelectObject(hdc, oldPen)
		if current != 0 {
			win.DeleteObject(current)
		}
	}
}

func applyLocalHostPanelPointerCursor() bool {
	button := hoveredLocalHostPanelButton()
	if button == 0 {
		return false
	}
	if !win.IsWindowVisible(button) || !win.IsWindowEnabled(button) {
		return false
	}
	cursor := loadSystemCursorHandles()["pointer"]
	if cursor == 0 {
		return false
	}
	win.SetCursor(cursor)
	return true
}

func hoveredLocalHostPanelButton() win.HWND {
	var point win.POINT
	if !win.GetCursorPos(&point) {
		return 0
	}
	target := win.WindowFromPoint(point)
	switch target {
	case hostPanelControls.copyButton,
		hostPanelControls.copyPasswordButton,
		hostPanelControls.approveButton,
		hostPanelControls.denyButton,
		hostPanelControls.toggleButton,
		hostPanelControls.accountButton,
		hostPanelControls.hideButton:
		return target
	default:
		return 0
	}
}

func localHostPanelSourceName(snapshot hostUIStatus) string {
	viewer := strings.TrimSpace(snapshot.PendingViewer)
	if viewer != "" {
		return strings.ToUpper(viewer)
	}
	serverHost := strings.TrimSpace(localHostPanelServerHostname(snapshot.ServerURL))
	if serverHost != "" {
		return strings.ToUpper(serverHost)
	}
	return hostText("window.host_name_fallback")
}

func localHostPanelServerHostname(serverURL string) string {
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		return ""
	}
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Hostname())
}

func drawHostPanelRoundRect(hdc win.HDC, rect *win.RECT, fill win.HBRUSH, border win.HBRUSH, radius int32) {
	if hdc == 0 || rect == nil {
		return
	}
	var oldBrush, oldPen win.HGDIOBJ
	if fill != 0 {
		oldBrush = win.SelectObject(hdc, win.HGDIOBJ(fill))
	} else {
		oldBrush = win.SelectObject(hdc, win.GetStockObject(win.NULL_BRUSH))
	}
	if border != 0 {
		oldPen = win.SelectObject(hdc, win.HGDIOBJ(createHostPanelPen(brushColor(border))))
	} else {
		oldPen = win.SelectObject(hdc, win.GetStockObject(win.NULL_PEN))
	}
	win.RoundRect(hdc, rect.Left, rect.Top, rect.Right, rect.Bottom, radius, radius)
	if oldPen != 0 {
		current := win.SelectObject(hdc, oldPen)
		if current != 0 && current != win.GetStockObject(win.NULL_PEN) {
			win.DeleteObject(current)
		}
	}
	if oldBrush != 0 {
		win.SelectObject(hdc, oldBrush)
	}
}

func createHostPanelPen(color win.COLORREF) win.HPEN {
	brush := win.LOGBRUSH{
		LbStyle: win.BS_SOLID,
		LbColor: color,
	}
	return win.ExtCreatePen(win.PS_GEOMETRIC|win.PS_SOLID|win.PS_ENDCAP_ROUND|win.PS_JOIN_ROUND, 1, &brush, 0, nil)
}

func brushColor(brush win.HBRUSH) win.COLORREF {
	switch brush {
	case hostPanelBackgroundBrush:
		return hostPanelColorBackground
	case hostPanelCardBrush:
		return hostPanelColorCard
	case hostPanelCardSoftBrush:
		return hostPanelColorCardSoft
	case hostPanelAccentBrush:
		return hostPanelColorAccent
	case hostPanelAccentSoftBrush:
		return hostPanelColorAccentSoft
	case hostPanelBorderBrush:
		return hostPanelColorBorder
	default:
		return hostPanelColorBorder
	}
}

func blendHostPanelColor(base win.COLORREF, tint win.COLORREF, ratio float64) win.COLORREF {
	clamp := func(value float64) byte {
		if value < 0 {
			value = 0
		}
		if value > 255 {
			value = 255
		}
		return byte(value)
	}
	baseR := float64(byte(base & 0xff))
	baseG := float64(byte((base >> 8) & 0xff))
	baseB := float64(byte((base >> 16) & 0xff))
	tintR := float64(byte(tint & 0xff))
	tintG := float64(byte((tint >> 8) & 0xff))
	tintB := float64(byte((tint >> 16) & 0xff))
	return win.RGB(
		clamp(baseR+(tintR-baseR)*ratio),
		clamp(baseG+(tintG-baseG)*ratio),
		clamp(baseB+(tintB-baseB)*ratio),
	)
}

func positionLocalHostPanel() {
	if hostPanelWindowHandle == 0 {
		return
	}
	var rect win.RECT
	if !win.GetWindowRect(hostPanelWindowHandle, &rect) {
		return
	}
	width := rect.Right - rect.Left
	height := rect.Bottom - rect.Top
	screenWidth := win.GetSystemMetrics(win.SM_CXSCREEN)
	screenHeight := win.GetSystemMetrics(win.SM_CYSCREEN)
	x := screenWidth - width - 32
	y := screenHeight - height - 80
	if x < 16 {
		x = 16
	}
	if y < 16 {
		y = 16
	}
	win.SetWindowPos(hostPanelWindowHandle, win.HWND_TOPMOST, x, y, 0, 0, win.SWP_NOSIZE)
}

func handleLocalHostPanelCommand(command uint16) {
	switch command {
	case hostPanelCommandCopyID:
		snapshot := localHostUI.snapshot()
		if strings.TrimSpace(snapshot.PublicID) != "" {
			_ = writeClipboardText(snapshot.PublicID)
		}
	case hostPanelCommandCopyPassword:
		snapshot := localHostUI.snapshot()
		if strings.TrimSpace(snapshot.AccessPassword) != "" {
			_ = writeClipboardText(snapshot.AccessPassword)
		}
	case hostPanelCommandApprove:
		if resolvePendingHostApproval(true) && hostPanelWindowHandle != 0 {
			win.ShowWindow(hostPanelWindowHandle, win.SW_HIDE)
		}
	case hostPanelCommandDeny:
		if resolvePendingHostApproval(false) && hostPanelWindowHandle != 0 {
			win.ShowWindow(hostPanelWindowHandle, win.SW_HIDE)
		}
	case hostPanelCommandToggle:
		snapshot := localHostUI.snapshot()
		setHostAccessEnabled(!snapshot.AccessEnabled)
		refreshLocalHostPanel()
		updateLocalHostTrayTip()
	case hostPanelCommandOpenAccount:
		openLocalHostAccount()
	case hostPanelCommandHide:
		dismissPendingHostApprovalPresentation()
		if hostPanelWindowHandle != 0 {
			win.ShowWindow(hostPanelWindowHandle, win.SW_HIDE)
		}
	}
}
