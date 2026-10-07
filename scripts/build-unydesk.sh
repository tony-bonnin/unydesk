#!/bin/sh

set -eu

ROOT_DIR=$(cd -- "$(dirname -- "$0")/.." && pwd)
BACKEND_DIR="$ROOT_DIR/unydesk/backend"
DIST_DIR=${UNYDESK_DIST_DIR:-"$ROOT_DIR/dist"}

mkdir -p "$DIST_DIR"

cd "$BACKEND_DIR"
go test ./...

GO_BUILD_FLAGS="${GO_BUILD_FLAGS:--trimpath -buildvcs=false}"
LDFLAGS="${LDFLAGS:--s -w -buildid=}"

# shellcheck disable=SC2086
CGO_ENABLED="${CGO_ENABLED:-0}" go build $GO_BUILD_FLAGS -ldflags "$LDFLAGS" -o "$DIST_DIR/unydesk" ./cmd/unydesk
# shellcheck disable=SC2086
CGO_ENABLED="${CGO_ENABLED:-0}" GOOS="${GOOS:-linux}" GOARCH="${GOARCH:-amd64}" go build $GO_BUILD_FLAGS -ldflags "$LDFLAGS" -o "$DIST_DIR/unydesk-host-linux-amd64" ./cmd/unydesk-host

sha256sum "$DIST_DIR/unydesk" "$DIST_DIR/unydesk-host-linux-amd64" > "$DIST_DIR/SHA256SUMS"
printf '[unydesk-build] built %s\n' "$DIST_DIR/unydesk"
