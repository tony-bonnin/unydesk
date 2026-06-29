//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

const windowsCreateNoWindow = 0x08000000

func configureBackgroundCommand(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windowsCreateNoWindow,
	}
}
