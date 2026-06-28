//go:build windows

package main

import (
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	shell32DLL        = syscall.NewLazyDLL("shell32.dll")
	procIsUserAnAdmin = shell32DLL.NewProc("IsUserAnAdmin")
	procShellExecuteW = shell32DLL.NewProc("ShellExecuteW")
)

func processHasAdminRights() bool {
	ret, _, _ := procIsUserAnAdmin.Call()
	return ret != 0
}

func relaunchProcessElevated() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	args := make([]string, 0, len(os.Args)-1)
	for _, arg := range os.Args[1:] {
		if strings.EqualFold(arg, "--elevate") {
			continue
		}
		args = append(args, arg)
	}

	operation, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := syscall.UTF16PtrFromString(executable)
	if err != nil {
		return err
	}
	parameters, err := syscall.UTF16PtrFromString(windowsCommandLine(args))
	if err != nil {
		return err
	}
	directory, err := syscall.UTF16PtrFromString("")
	if err != nil {
		return err
	}

	ret, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(operation)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(parameters)),
		uintptr(unsafe.Pointer(directory)),
		1,
	)
	if ret <= 32 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return syscall.Errno(ret)
	}
	return nil
}

func windowsCommandLine(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, windowsQuoteArg(arg))
	}
	return strings.Join(quoted, " ")
}

func windowsQuoteArg(arg string) string {
	if arg == "" {
		return `""`
	}
	if !strings.ContainsAny(arg, " \t\n\v\"") {
		return arg
	}

	var b strings.Builder
	b.WriteByte('"')
	backslashes := 0
	for _, r := range arg {
		switch r {
		case '\\':
			backslashes++
		case '"':
			b.WriteString(strings.Repeat("\\", backslashes*2+1))
			b.WriteRune(r)
			backslashes = 0
		default:
			if backslashes > 0 {
				b.WriteString(strings.Repeat("\\", backslashes))
				backslashes = 0
			}
			b.WriteRune(r)
		}
	}
	if backslashes > 0 {
		b.WriteString(strings.Repeat("\\", backslashes*2))
	}
	b.WriteByte('"')
	return b.String()
}
