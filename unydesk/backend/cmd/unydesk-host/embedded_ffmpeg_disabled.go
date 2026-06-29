//go:build !ffmpegembed

package main

func embeddedFFmpegPayloadForRuntime() (embeddedFFmpegPayload, bool) {
	return embeddedFFmpegPayload{}, false
}
