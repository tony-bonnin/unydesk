package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

func machineFingerprint() string {
	switch runtime.GOOS {
	case "linux":
		return normalizeMachineFingerprintParts(
			readTrimmed("/etc/machine-id"),
			readTrimmed("/var/lib/dbus/machine-id"),
			readTrimmed("/sys/class/dmi/id/product_uuid"),
			readTrimmed("/sys/class/dmi/id/product_serial"),
		)
	case "windows":
		return normalizeMachineFingerprintParts(
			parseWindowsRegistryValue(readCommand("reg", "query", `HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid"), "MachineGuid"),
			parseWindowsColumnValue(readCommand("wmic", "csproduct", "get", "uuid"), "UUID"),
			parseWindowsColumnValue(readCommand("wmic", "bios", "get", "serialnumber"), "SerialNumber"),
			parseWindowsColumnValue(readCommand("wmic", "baseboard", "get", "serialnumber"), "SerialNumber"),
		)
	case "darwin":
		return normalizeMachineFingerprintParts(
			parseDarwinPlatformUUID(readCommand("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")),
			readCommand("system_profiler", "SPHardwareDataType"),
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

func parseWindowsRegistryValue(raw, key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if strings.ToLower(fields[0]) != key {
			continue
		}
		return strings.TrimSpace(fields[len(fields)-1])
	}
	return ""
}

func parseWindowsColumnValue(raw, column string) string {
	column = strings.TrimSpace(strings.ToLower(column))
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, column) {
			continue
		}
		return line
	}
	return ""
}

func normalizeMachineFingerprintParts(values ...string) string {
	parts := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.Trim(value, `"`)
		if value == "" || value == "default string" || value == "to be filled by o.e.m." {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		parts = append(parts, value)
	}
	if len(parts) == 0 {
		return ""
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return base64.RawURLEncoding.EncodeToString(sum[:24])
}
