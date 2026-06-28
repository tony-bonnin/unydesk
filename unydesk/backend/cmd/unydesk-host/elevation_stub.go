//go:build !windows

package main

import "fmt"

func processHasAdminRights() bool {
	return false
}

func relaunchProcessElevated() error {
	return fmt.Errorf("admin elevation is only supported on Windows")
}
