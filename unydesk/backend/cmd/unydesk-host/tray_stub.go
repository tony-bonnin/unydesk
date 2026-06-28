//go:build !windows

package main

import "context"

func startLocalHostTray(ctx context.Context, shutdown context.CancelFunc) {}

func hideConsoleWindow() {}
