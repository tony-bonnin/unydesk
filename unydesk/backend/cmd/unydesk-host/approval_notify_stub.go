//go:build !windows

package main

func notifyPendingHostApproval(session hostSessionDispatch) {}

func notifyResolvedHostApproval(session hostSessionDispatch, allow bool) {}
