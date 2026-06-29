//go:build ffmpegembed && !(windows && amd64)

package main

func embeddedFFmpegPayloadForRuntime() (embeddedFFmpegPayload, bool) {
	return embeddedFFmpegPayload{}, false
}
