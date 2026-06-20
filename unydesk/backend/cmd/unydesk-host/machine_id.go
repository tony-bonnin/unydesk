package main

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func machineFingerprint() string {
	switch runtime.GOOS {
	case "linux":
		return firstNonEmpty(
			readTrimmed("/etc/machine-id"),
			readTrimmed("/var/lib/dbus/machine-id"),
		)
	case "windows":
		return firstNonEmpty(
			readCommand("reg", "query", `HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid"),
			readCommand("wmic", "csproduct", "get", "uuid"),
		)
	case "darwin":
		return firstNonEmpty(
			parseDarwinPlatformUUID(readCommand("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")),
			readTrimmed("/Library/Preferences/SystemConfiguration/com.apple.platform.uuid.plist"),
		)
	default:
		return ""
	}
}

func readTrimmed(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readCommand(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}

func parseDarwinPlatformUUID(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "IOPlatformUUID") {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) < 2 {
			continue
		}
		value := strings.TrimSpace(parts[len(parts)-1])
		value = strings.Trim(value, `"`)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if runtime.GOOS == "windows" && strings.Contains(strings.ToLower(value), "machineguid") {
			fields := strings.Fields(value)
			if len(fields) > 0 {
				value = fields[len(fields)-1]
			}
		}
		if runtime.GOOS == "windows" && strings.Contains(strings.ToLower(value), "uuid") {
			lines := strings.Split(value, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.EqualFold(line, "UUID") {
					continue
				}
				return line
			}
		}
		return value
	}
	return ""
}
