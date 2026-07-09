//go:build !windows

package main

func notifyPendingHostApproval(session hostSessionDispatch) {}

func dismissPendingHostApprovalUI() {}

func notifyResolvedHostApproval(session hostSessionDispatch, allow bool) {}

func pendingHostApprovalUIVisible() bool { return false }
