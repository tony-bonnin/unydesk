//go:build windows

package main

import (
	"strings"

	"github.com/lxn/win"
)

func pendingHostApprovalUIVisible() bool {
	return hostPanelWindowHandle != 0 && win.IsWindowVisible(hostPanelWindowHandle)
}

func notifyPendingHostApproval(session hostSessionDispatch) {
	if !showLocalHostPanel() {
		viewer := strings.TrimSpace(session.Viewer)
		if viewer == "" {
			viewer = hostText("viewer.a_client")
		}
		showLocalHostTrayNotification(hostText("notif.request_title"), hostTextf("notif.request_message", viewer))
	}
	updateLocalHostTrayTip()
}

func dismissPendingHostApprovalUI() {
	if hostPanelWindowHandle != 0 {
		win.ShowWindow(hostPanelWindowHandle, win.SW_HIDE)
	}
}

func notifyResolvedHostApproval(session hostSessionDispatch, allow bool) {
	viewer := strings.TrimSpace(session.Viewer)
	if viewer == "" {
		viewer = hostText("viewer.remote_client")
	}
	if allow {
		showLocalHostTrayNotification(hostText("notif.allowed_title"), hostTextf("notif.allowed_message", viewer))
	} else {
		showLocalHostTrayNotification(hostText("notif.denied_title"), hostTextf("notif.denied_message", viewer))
	}
	updateLocalHostTrayTip()
}
