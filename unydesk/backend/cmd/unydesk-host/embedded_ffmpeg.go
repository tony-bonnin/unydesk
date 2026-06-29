package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type embeddedFFmpegPayload struct {
	GOOS     string
	GOARCH   string
	Filename string
	Gzip     bool
	Data     []byte
}

var (
	embeddedFFmpegOnce sync.Once
	embeddedFFmpegPath string
	embeddedFFmpegOK   bool
)

func embeddedFFmpegBinaryPath() (string, bool) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("UNYDESK_DISABLE_EMBEDDED_FFMPEG")), "1") {
		return "", false
	}
	embeddedFFmpegOnce.Do(func() {
		payload, ok := embeddedFFmpegPayloadForRuntime()
		if !ok || len(payload.Data) == 0 {
			return
		}
		if payload.GOOS != runtime.GOOS || payload.GOARCH != runtime.GOARCH {
			return
		}
		path, err := materializeEmbeddedFFmpeg(payload)
		if err != nil {
			fmt.Printf("Embedded FFmpeg unavailable: %v\n", err)
			return
		}
		embeddedFFmpegPath = path
		embeddedFFmpegOK = true
	})
	return embeddedFFmpegPath, embeddedFFmpegOK
}

func materializeEmbeddedFFmpeg(payload embeddedFFmpegPayload) (string, error) {
	raw, err := embeddedFFmpegBytes(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	shortHash := hex.EncodeToString(sum[:])[:12]
	dir, err := embeddedFFmpegCacheDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	filename := strings.TrimSpace(payload.Filename)
	if filename == "" {
		filename = "ffmpeg"
		if runtime.GOOS == "windows" {
			filename += ".exe"
		}
	}
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	outPath := filepath.Join(dir, fmt.Sprintf("%s-%s%s", base, shortHash, ext))
	if embeddedFFmpegFileMatches(outPath, sum[:]) {
		return outPath, nil
	}
	tmpPath := outPath + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o700); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	_ = os.Chmod(outPath, 0o700)
	fmt.Printf("Embedded FFmpeg extracted to %s\n", outPath)
	return outPath, nil
}

func embeddedFFmpegBytes(payload embeddedFFmpegPayload) ([]byte, error) {
	if !payload.Gzip {
		return payload.Data, nil
	}
	reader, err := gzip.NewReader(bytes.NewReader(payload.Data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func embeddedFFmpegCacheDir() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("UNYDESK_FFMPEG_CACHE_DIR")); configured != "" {
		return configured, nil
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(cacheDir) == "" {
		executable, exeErr := os.Executable()
		if exeErr != nil {
			return "", err
		}
		cacheDir = filepath.Dir(executable)
	}
	return filepath.Join(cacheDir, "UnyDesk", "bin"), nil
}

func embeddedFFmpegFileMatches(path string, want []byte) bool {
	current, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(current)
	return bytes.Equal(sum[:], want)
}
