//go:build windows

package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

const (
	trayCallbackMessage = win.WM_APP + 1
	trayRefreshMessage  = win.WM_APP + 2

	trayMenuApproveAccess = 1000
	trayMenuDenyAccess    = 1001
	trayMenuCopyAccessID  = 1002
	trayMenuToggleAccess  = 1003
	trayMenuOpenAccount   = 1004
	trayMenuTestServer    = 1005
	trayMenuQuit          = 1006
)

var (
	startLocalHostTrayOnce sync.Once
	trayWindowClassName    = syscall.StringToUTF16Ptr("UnyDeskHostTrayWindow")
	trayWindowTitle        = syscall.StringToUTF16Ptr("UnyDeskHostTrayWindow")
	trayWindowProc         = syscall.NewCallback(handleLocalHostTrayWindowMessage)
	trayWindowHandle       win.HWND
	trayShutdown           context.CancelFunc
	trayIconData           win.NOTIFYICONDATA
)

func startLocalHostTray(ctx context.Context, shutdown context.CancelFunc) {
	startLocalHostTrayOnce.Do(func() {
		trayShutdown = shutdown
		go runLocalHostTray(ctx)
	})
}

func hideConsoleWindow() {
	console := win.GetConsoleWindow()
	if console != 0 {
		win.ShowWindow(console, win.SW_HIDE)
	}
}

func runLocalHostTray(ctx context.Context) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInstance := win.GetModuleHandle(nil)
	class := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   trayWindowProc,
		HInstance:     hInstance,
		HIcon:         loadLocalHostTrayIcon(hInstance),
		HCursor:       0,
		HbrBackground: 0,
		LpszClassName: trayWindowClassName,
		HIconSm:       loadLocalHostTrayIcon(hInstance),
	}
	if win.RegisterClassEx(&class) == 0 {
		return
	}

	hwnd := win.CreateWindowEx(
		0,
		trayWindowClassName,
		trayWindowTitle,
		0,
		0, 0, 0, 0,
		0,
		0,
		hInstance,
		nil,
	)
	if hwnd == 0 {
		return
	}
	trayWindowHandle = hwnd
	defer func() {
		trayWindowHandle = 0
	}()
	_ = initLocalHostPanel(hInstance)

	if !addLocalHostTrayIcon(hwnd, class.HIcon) {
		_ = win.DestroyWindow(hwnd)
		return
	}
	defer removeLocalHostTrayIcon()

	go func() {
		<-ctx.Done()
		if trayWindowHandle != 0 {
			win.PostMessage(trayWindowHandle, win.WM_CLOSE, 0, 0)
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if trayWindowHandle != 0 {
				win.PostMessage(trayWindowHandle, trayRefreshMessage, 0, 0)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
		}
	}()

	updateLocalHostTrayTip()

	var msg win.MSG
	for {
		ret := win.GetMessage(&msg, 0, 0, 0)
		if ret == 0 || ret == -1 {
			break
		}
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
}

func handleLocalHostTrayWindowMessage(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case trayCallbackMessage:
		switch localHostTrayEventCode(lParam) {
		case win.NIN_BALLOONUSERCLICK:
			if hasPendingHostApproval() && showLocalHostPanel() {
				return 0
			}
			showLocalHostTrayMenu(hwnd)
			return 0
		case win.WM_RBUTTONUP, win.WM_CONTEXTMENU, win.NIN_KEYSELECT:
			showLocalHostTrayMenu(hwnd)
			return 0
		}
	case trayRefreshMessage:
		updateLocalHostTrayTip()
		return 0
	case win.WM_CLOSE:
		win.DestroyWindow(hwnd)
		return 0
	case win.WM_DESTROY:
		removeLocalHostTrayIcon()
		destroyLocalHostPanel()
		win.PostQuitMessage(0)
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func localHostTrayEventCode(lParam uintptr) uint32 {
	return uint32(uint16(lParam & 0xffff))
}

func loadLocalHostTrayIcon(hInstance win.HINSTANCE) win.HICON {
	if icon := win.LoadIcon(hInstance, win.MAKEINTRESOURCE(1)); icon != 0 {
		return icon
	}
	return win.LoadIcon(0, win.MAKEINTRESOURCE(win.IDI_APPLICATION))
}

func addLocalHostTrayIcon(hwnd win.HWND, icon win.HICON) bool {
	trayIconData = win.NOTIFYICONDATA{}
	trayIconData.CbSize = uint32(unsafe.Sizeof(trayIconData))
	trayIconData.HWnd = hwnd
	trayIconData.UID = 1
	trayIconData.UFlags = win.NIF_MESSAGE | win.NIF_ICON | win.NIF_TIP | win.NIF_SHOWTIP
	trayIconData.UCallbackMessage = trayCallbackMessage
	trayIconData.HIcon = icon
	setLocalHostTrayString(trayIconData.SzTip[:], localHostTrayBaseLabel(localHostUI.snapshot()))
	if !win.Shell_NotifyIcon(win.NIM_ADD, &trayIconData) {
		return false
	}
	trayIconData.UVersion = win.NOTIFYICON_VERSION_4
	_ = win.Shell_NotifyIcon(win.NIM_SETVERSION, &trayIconData)
	return true
}

func showLocalHostTrayNotification(title, message string) {
	if trayIconData.HWnd == 0 {
		return
	}
	data := trayIconData
	data.UFlags = win.NIF_INFO
	data.DwInfoFlags = win.NIIF_INFO
	setLocalHostTrayString(data.SzInfoTitle[:], title)
	setLocalHostTrayString(data.SzInfo[:], message)
	_ = win.Shell_NotifyIcon(win.NIM_MODIFY, &data)
}

func removeLocalHostTrayIcon() {
	if trayIconData.HWnd != 0 {
		_ = win.Shell_NotifyIcon(win.NIM_DELETE, &trayIconData)
	}
}

func updateLocalHostTrayTip() {
	if trayIconData.HWnd == 0 {
		return
	}
	snapshot := localHostUI.snapshot()
	tip := localHostTrayBaseLabel(snapshot)
	switch {
	case strings.TrimSpace(snapshot.PublicID) != "":
		tip = fmt.Sprintf("%s  %s • %s", localHostTrayBaseLabel(snapshot), strings.TrimSpace(snapshot.ConnectionState), strings.TrimSpace(snapshot.PublicID))
	case strings.TrimSpace(snapshot.ConnectionState) != "":
		tip = fmt.Sprintf("%s  %s", localHostTrayBaseLabel(snapshot), strings.TrimSpace(snapshot.ConnectionState))
	}
	trayIconData.UFlags = win.NIF_TIP | win.NIF_SHOWTIP
	setLocalHostTrayString(trayIconData.SzTip[:], tip)
	_ = win.Shell_NotifyIcon(win.NIM_MODIFY, &trayIconData)
}

func localHostTrayBaseLabel(snapshot hostUIStatus) string {
	hostname := strings.TrimSpace(snapshot.Hostname)
	if hostname == "" {
		return "UnyDesk"
	}
	return hostname
}

func setLocalHostTrayString(dst []uint16, value string) {
	for index := range dst {
		dst[index] = 0
	}
	text := syscall.StringToUTF16(value)
	if len(text) > len(dst) {
		text = text[:len(dst)]
		text[len(dst)-1] = 0
	}
	copy(dst, text)
}

func showLocalHostTrayMenu(hwnd win.HWND) {
	snapshot := localHostUI.snapshot()

	menu := win.CreatePopupMenu()
	if menu == 0 {
		return
	}
	defer win.DestroyMenu(menu)

	index := uint32(0)
	statusLabel := buildLocalHostTrayStatusLabel(snapshot)
	localHostTrayInsertItem(menu, index, 0, statusLabel, false)
	index++
	if summary := pendingHostApprovalSummary(); summary != "" {
		localHostTrayInsertItem(menu, index, 0, summary, false)
		index++
		localHostTrayInsertSeparator(menu, index)
		index++
		localHostTrayInsertItem(menu, index, trayMenuApproveAccess, hostText("tray.allow_request"), true)
		index++
		localHostTrayInsertItem(menu, index, trayMenuDenyAccess, hostText("tray.deny_request"), true)
		index++
	}
	localHostTrayInsertSeparator(menu, index)
	index++
	localHostTrayInsertItem(menu, index, trayMenuCopyAccessID, hostText("tray.copy_access_id"), strings.TrimSpace(snapshot.PublicID) != "")
	index++
	toggleLabel := hostText("tray.stop_remote_access")
	if !snapshot.AccessEnabled {
		toggleLabel = hostText("tray.resume_remote_access")
	}
	localHostTrayInsertItem(menu, index, trayMenuToggleAccess, toggleLabel, true)
	index++
	localHostTrayInsertItem(menu, index, trayMenuOpenAccount, hostText("tray.open_dashboard"), strings.TrimSpace(snapshot.AccountURL) != "")
	index++
	localHostTrayInsertItem(menu, index, trayMenuTestServer, hostText("tray.test_server"), strings.TrimSpace(snapshot.ServerURL) != "")
	index++
	localHostTrayInsertSeparator(menu, index)
	index++
	localHostTrayInsertItem(menu, index, trayMenuQuit, hostText("tray.quit_host"), true)

	var point win.POINT
	if !win.GetCursorPos(&point) {
		return
	}
	win.SetForegroundWindow(hwnd)
	command := win.TrackPopupMenu(
		menu,
		win.TPM_LEFTALIGN|win.TPM_BOTTOMALIGN|win.TPM_RETURNCMD|win.TPM_RIGHTBUTTON,
		point.X,
		point.Y,
		0,
		hwnd,
		nil,
	)
	_ = win.PostMessage(hwnd, win.WM_NULL, 0, 0)
	if command == 0 {
		return
	}
	handleLocalHostTrayCommand(uint32(command))
}

func buildLocalHostTrayStatusLabel(snapshot hostUIStatus) string {
	state := localHostTrayStateLabel(snapshot)
	publicID := strings.TrimSpace(snapshot.PublicID)
	if publicID == "" {
		return state
	}
	return fmt.Sprintf("%s · %s", state, publicID)
}

func localHostTrayStateLabel(snapshot hostUIStatus) string {
	switch {
	case snapshot.PendingApproval:
		return "Approval needed"
	case strings.EqualFold(strings.TrimSpace(snapshot.ConnectionState), "Link required"):
		return "Link required"
	case strings.EqualFold(strings.TrimSpace(snapshot.ConnectionState), "Provisioning required"):
		return "Provisioning required"
	case !snapshot.AccessEnabled:
		return "Remote access stopped"
	case snapshot.Connected:
		return "Online"
	case strings.EqualFold(strings.TrimSpace(snapshot.ConnectionState), "Reconnecting"):
		return "Reconnecting"
	default:
		return "Starting"
	}
}

func localHostTrayInsertItem(menu win.HMENU, position uint32, id uint32, label string, enabled bool) {
	text := syscall.StringToUTF16Ptr(label)
	state := uint32(win.MFS_ENABLED)
	if !enabled {
		state = win.MFS_DISABLED
	}
	item := win.MENUITEMINFO{
		CbSize:     uint32(unsafe.Sizeof(win.MENUITEMINFO{})),
		FMask:      win.MIIM_ID | win.MIIM_STRING | win.MIIM_STATE | win.MIIM_FTYPE,
		FType:      win.MFT_STRING,
		FState:     state,
		WID:        id,
		DwTypeData: text,
	}
	_ = win.InsertMenuItem(menu, position, true, &item)
}

func localHostTrayInsertSeparator(menu win.HMENU, position uint32) {
	item := win.MENUITEMINFO{
		CbSize: uint32(unsafe.Sizeof(win.MENUITEMINFO{})),
		FMask:  win.MIIM_FTYPE,
		FType:  win.MFT_SEPARATOR,
	}
	_ = win.InsertMenuItem(menu, position, true, &item)
}

func handleLocalHostTrayCommand(command uint32) {
	switch command {
	case trayMenuApproveAccess:
		resolvePendingHostApproval(true)
	case trayMenuDenyAccess:
		resolvePendingHostApproval(false)
	case trayMenuCopyAccessID:
		snapshot := localHostUI.snapshot()
		if strings.TrimSpace(snapshot.PublicID) != "" {
			_ = writeClipboardText(snapshot.PublicID)
		}
	case trayMenuToggleAccess:
		snapshot := localHostUI.snapshot()
		setHostAccessEnabled(!snapshot.AccessEnabled)
		updateLocalHostTrayTip()
	case trayMenuOpenAccount:
		openLocalHostAccount()
	case trayMenuTestServer:
		go testLocalHostServer()
	case trayMenuQuit:
		if trayShutdown != nil {
			trayShutdown()
		}
		if trayWindowHandle != 0 {
			win.PostMessage(trayWindowHandle, win.WM_CLOSE, 0, 0)
		}
	}
}

func openLocalHostAccount() {
	snapshot := localHostUI.snapshot()
	if strings.TrimSpace(snapshot.AccountURL) == "" {
		return
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", snapshot.AccountURL).Start()
}

func testLocalHostServer() {
	snapshot := localHostUI.snapshot()
	serverURL := strings.TrimSpace(snapshot.ServerURL)
	if serverURL == "" {
		showLocalHostTrayNotification("UnyDesk", "No server route is configured yet.")
		return
	}

	if _, err := fetchRuntimeConfig(serverURL, currentRuntimeServerCredential()); err != nil {
		localHostUI.setDisconnected(err, 0)
		showLocalHostTrayNotification("UnyDesk", "Server test failed. Reconnecting.")
		triggerBootstrapWake()
		return
	}

	showLocalHostTrayNotification("UnyDesk", "Server reachable. Registration refresh requested.")
	triggerBootstrapWake()
}
