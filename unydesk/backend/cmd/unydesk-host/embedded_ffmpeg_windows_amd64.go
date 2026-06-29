//go:build ffmpegembed && windows && amd64

package main

import _ "embed"

//go:embed embedded/ffmpeg/windows-amd64/ffmpeg.exe.gz
var embeddedFFmpegWindowsAMD64 []byte

func embeddedFFmpegPayloadForRuntime() (embeddedFFmpegPayload, bool) {
	return embeddedFFmpegPayload{
		GOOS:     "windows",
		GOARCH:   "amd64",
		Filename: "ffmpeg.exe",
		Gzip:     true,
		Data:     embeddedFFmpegWindowsAMD64,
	}, true
}
