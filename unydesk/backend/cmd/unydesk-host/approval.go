package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type pendingHostApproval struct {
	session hostSessionDispatch
	decide  func(action string)
}

var hostApprovalState struct {
	mu             sync.Mutex
	pending        *pendingHostApproval
	recentApproved map[string]time.Time
	dismissedKey   string
}

const hostApprovalReuseWindow = 45 * time.Second

func recentHostApprovalKey(session hostSessionDispatch) string {
	parts := []string{
		strings.TrimSpace(session.ID),
		strings.TrimSpace(session.Viewer),
		strings.TrimSpace(session.Target),
		strings.TrimSpace(session.RoutedHostID),
		strings.TrimSpace(session.RoutedHostPublicID),
	}
	return strings.Join(parts, "\x00")
}

func hasCachedApprovedHostAccess(session hostSessionDispatch) bool {
	hostApprovalState.mu.Lock()
	defer hostApprovalState.mu.Unlock()

	now := time.Now().UTC()
	for key, approvedAt := range hostApprovalState.recentApproved {
		if now.Sub(approvedAt) > hostApprovalReuseWindow {
			delete(hostApprovalState.recentApproved, key)
		}
	}

	approvedAt, ok := hostApprovalState.recentApproved[recentHostApprovalKey(session)]
	if !ok {
		return false
	}
	return now.Sub(approvedAt) <= hostApprovalReuseWindow
}

func rememberApprovedHostAccess(session hostSessionDispatch) {
	key := recentHostApprovalKey(session)
	if key == "" {
		return
	}
	hostApprovalState.mu.Lock()
	defer hostApprovalState.mu.Unlock()
	if hostApprovalState.recentApproved == nil {
		hostApprovalState.recentApproved = make(map[string]time.Time)
	}
	now := time.Now().UTC()
	hostApprovalState.recentApproved[key] = now
	for existingKey, approvedAt := range hostApprovalState.recentApproved {
		if now.Sub(approvedAt) > hostApprovalReuseWindow {
			delete(hostApprovalState.recentApproved, existingKey)
		}
	}
}

func queuePendingHostApproval(session hostSessionDispatch, decide func(action string)) bool {
	hostApprovalState.mu.Lock()

	if hostApprovalState.pending != nil {
		currentKey := recentHostApprovalKey(hostApprovalState.pending.session)
		incomingKey := recentHostApprovalKey(session)
		if currentKey != "" && currentKey == incomingKey {
			hostApprovalState.pending.session = session
			hostApprovalState.pending.decide = decide
			hostApprovalState.mu.Unlock()
			return true
		}
		if !pendingHostApprovalUIVisible() {
			hostApprovalState.pending = &pendingHostApproval{
				session: session,
				decide:  decide,
			}
			hostApprovalState.dismissedKey = ""
			hostApprovalState.mu.Unlock()
			localHostUI.noteApprovalRequested(session)
			notifyPendingHostApproval(session)
			return true
		}
		hostApprovalState.mu.Unlock()
		return false
	}

	hostApprovalState.pending = &pendingHostApproval{
		session: session,
		decide:  decide,
	}
	hostApprovalState.dismissedKey = ""
	hostApprovalState.mu.Unlock()
	localHostUI.noteApprovalRequested(session)
	notifyPendingHostApproval(session)
	return true
}

func repromptPendingHostApproval(session hostSessionDispatch, decide func(action string)) bool {
	hostApprovalState.mu.Lock()
	currentKey := recentHostApprovalKey(session)
	if hostApprovalState.pending != nil {
		pendingKey := recentHostApprovalKey(hostApprovalState.pending.session)
		if pendingKey != "" && currentKey != "" && pendingKey != currentKey {
			hostApprovalState.mu.Unlock()
			return false
		}
	}
	hostApprovalState.pending = &pendingHostApproval{
		session: session,
		decide:  decide,
	}
	hostApprovalState.dismissedKey = ""
	hostApprovalState.mu.Unlock()

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
	hostApprovalState.dismissedKey = ""
	hostApprovalState.mu.Unlock()

	localHostUI.clearApprovalRequest()
	action := "deny"
	if allow {
		action = "accept"
		rememberApprovedHostAccess(pending.session)
	}
	dismissPendingHostApprovalUI()
	notifyResolvedHostApproval(pending.session, allow)
	go pending.decide(action)
	return true
}

func dismissPendingHostApprovalPresentation() {
	hostApprovalState.mu.Lock()
	defer hostApprovalState.mu.Unlock()
	if hostApprovalState.pending == nil {
		return
	}
	hostApprovalState.dismissedKey = recentHostApprovalKey(hostApprovalState.pending.session)
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
