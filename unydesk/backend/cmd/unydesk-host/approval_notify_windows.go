//go:build windows

package main

import (
	"fmt"
	"strings"
)

func notifyPendingHostApproval(session hostSessionDispatch) {
	viewer := strings.TrimSpace(session.Viewer)
	if viewer == "" {
		viewer = "a client"
	}
	message := fmt.Sprintf("Remote access request from %s. Right-click the UnyDesk tray icon to Allow or Deny.", viewer)
	showLocalHostTrayNotification("Remote access request", message)
	updateLocalHostTrayTip()
}

func notifyResolvedHostApproval(session hostSessionDispatch, allow bool) {
	viewer := strings.TrimSpace(session.Viewer)
	if viewer == "" {
		viewer = "Remote client"
	}
	if allow {
		showLocalHostTrayNotification("Remote access allowed", viewer+" can now control this host.")
	} else {
		showLocalHostTrayNotification("Remote access denied", viewer+" was denied for this host.")
	}
	updateLocalHostTrayTip()
}
