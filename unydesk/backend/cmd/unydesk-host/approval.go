package main

import (
	"fmt"
	"strings"
	"sync"
)

type pendingHostApproval struct {
	session hostSessionDispatch
	decide  func(action string)
}

var hostApprovalState struct {
	mu      sync.Mutex
	pending *pendingHostApproval
}

func queuePendingHostApproval(session hostSessionDispatch, decide func(action string)) bool {
	hostApprovalState.mu.Lock()
	defer hostApprovalState.mu.Unlock()

	if hostApprovalState.pending != nil {
		return false
	}

	hostApprovalState.pending = &pendingHostApproval{
		session: session,
		decide:  decide,
	}
	localHostUI.noteApprovalRequested(session)
	notifyPendingHostApproval(session)
	return true
}

func hasPendingHostApproval() bool {
	hostApprovalState.mu.Lock()
	defer hostApprovalState.mu.Unlock()
	return hostApprovalState.pending != nil
}

func pendingHostApprovalSession() (hostSessionDispatch, bool) {
	hostApprovalState.mu.Lock()
	defer hostApprovalState.mu.Unlock()
	if hostApprovalState.pending == nil {
		return hostSessionDispatch{}, false
	}
	return hostApprovalState.pending.session, true
}

func resolvePendingHostApproval(allow bool) bool {
	hostApprovalState.mu.Lock()
	pending := hostApprovalState.pending
	if pending == nil {
		hostApprovalState.mu.Unlock()
		return false
	}
	hostApprovalState.pending = nil
	hostApprovalState.mu.Unlock()

	localHostUI.clearApprovalRequest()
	action := "deny"
	if allow {
		action = "accept"
	}
	notifyResolvedHostApproval(pending.session, allow)
	go pending.decide(action)
	return true
}

func pendingHostApprovalSummary() string {
	session, ok := pendingHostApprovalSession()
	if !ok {
		return ""
	}
	viewer := strings.TrimSpace(session.Viewer)
	target := strings.TrimSpace(session.Target)
	switch {
	case viewer != "" && target != "":
		return fmt.Sprintf("%s → %s", viewer, target)
	case viewer != "":
		return viewer
	case target != "":
		return target
	default:
		return strings.TrimSpace(session.ID)
	}
}
