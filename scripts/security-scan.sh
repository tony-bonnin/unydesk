#!/bin/sh

set -eu

ROOT_DIR=$(cd -- "$(dirname -- "$0")/.." && pwd)
BACKEND_DIR="$ROOT_DIR/unydesk/backend"

"$ROOT_DIR/scripts/csp-audit.sh"

cd "$BACKEND_DIR"
go test ./...
go vet ./...

run_govulncheck() {
  if command -v govulncheck >/dev/null 2>&1; then
    govulncheck ./...
    return $?
  fi
  if [ -x "$(go env GOPATH)/bin/govulncheck" ]; then
    "$(go env GOPATH)/bin/govulncheck" ./...
    return $?
  fi
  echo "govulncheck not found; install with:" >&2
  echo "go install golang.org/x/vuln/cmd/govulncheck@latest" >&2
  return 127
}

attempt=1
while ! run_govulncheck; do
  if [ "$attempt" -ge 3 ]; then
    exit 1
  fi
  sleep $((attempt * 5))
  attempt=$((attempt + 1))
done
