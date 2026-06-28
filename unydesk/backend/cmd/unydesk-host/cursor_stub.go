//go:build !windows

package main

func detectHostCursorKind() string {
	return "default"
}

func normalizeRemoteCursorKind(kind string) string {
	return "default"
}
