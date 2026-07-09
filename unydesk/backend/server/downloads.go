package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type hostDownloadSpec struct {
	goos     string
	goarch   string
	filename string
}

func (s *Server) handleHostDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	w.Header().Set("Cache-Control", "no-store, max-age=0")

	target := strings.TrimPrefix(r.URL.Path, "/download/host/")
	spec, ok := hostTargetSpec(target)
	if !ok {
		writeError(w, http.StatusNotFound, "host download not found")
		return
	}

	path := filepath.Join(s.cfg.Paths.HostDownloadsDir, spec.filename)
	if _, err := os.Stat(path); err != nil {
		writeError(w, http.StatusNotFound, "host binary not available yet")
		return
	}

	if spec.filename == "SHA256SUMS" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.ServeFile(w, r, path)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", spec.filename))
	if strings.HasSuffix(spec.filename, ".zip") {
		w.Header().Set("Content-Type", "application/zip")
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	http.ServeFile(w, r, path)
}

func hostTargetSpec(target string) (hostDownloadSpec, bool) {
	switch target {
	case "linux-amd64":
		return hostDownloadSpec{goos: "linux", goarch: "amd64", filename: "unydesk-host-linux-amd64"}, true
	case "linux-amd64.zip":
		return hostDownloadSpec{filename: "unydesk-host-linux-amd64.zip"}, true
	case "linux-arm64":
		return hostDownloadSpec{goos: "linux", goarch: "arm64", filename: "unydesk-host-linux-arm64"}, true
	case "linux-arm64.zip":
		return hostDownloadSpec{filename: "unydesk-host-linux-arm64.zip"}, true
	case "windows-amd64":
		return hostDownloadSpec{goos: "windows", goarch: "amd64", filename: "unydesk-host-windows-amd64.exe"}, true
	case "windows-amd64.zip":
		return hostDownloadSpec{filename: "unydesk-host-windows-amd64.zip"}, true
	case "windows-arm64":
		return hostDownloadSpec{goos: "windows", goarch: "arm64", filename: "unydesk-host-windows-arm64.exe"}, true
	case "windows-arm64.zip":
		return hostDownloadSpec{filename: "unydesk-host-windows-arm64.zip"}, true
	case "macos-amd64":
		return hostDownloadSpec{goos: "darwin", goarch: "amd64", filename: "unydesk-host-darwin-amd64"}, true
	case "macos-amd64.zip":
		return hostDownloadSpec{filename: "unydesk-host-macos-amd64.zip"}, true
	case "macos-arm64":
		return hostDownloadSpec{goos: "darwin", goarch: "arm64", filename: "unydesk-host-darwin-arm64"}, true
	case "macos-arm64.zip":
		return hostDownloadSpec{filename: "unydesk-host-macos-arm64.zip"}, true
	case "checksums":
		return hostDownloadSpec{filename: "SHA256SUMS"}, true
	}
	return hostDownloadSpec{}, false
}
