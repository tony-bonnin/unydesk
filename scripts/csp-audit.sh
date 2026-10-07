#!/bin/sh

set -eu

ROOT_DIR=$(cd -- "$(dirname -- "$0")/.." && pwd)
FAILED=0
TMP="${TMPDIR:-/tmp}/unydesk-csp-audit.$$"

check_absent() {
  pattern=$1
  shift
  if grep -R -n -E --exclude-dir=vendor "$pattern" "$@" >"$TMP" 2>/dev/null; then
    cat "$TMP" >&2
    FAILED=1
  fi
}

check_absent "unsafe-|sha256-|cdn\\.jsdelivr|@import url\\(['\"]https?://" \
  "$ROOT_DIR/unydesk/backend/server" \
  "$ROOT_DIR/unydesk/frontend/public"

check_absent "document\\.write|outerHTML|insertAdjacentHTML|eval\\(" \
  "$ROOT_DIR/unydesk/frontend/public"

if grep -R -n "innerHTML" "$ROOT_DIR/unydesk/frontend/public" \
  --exclude-dir=vendor >"$TMP" 2>/dev/null; then
  if grep -v "frontend/public/account/account.js:" "$TMP" >&2; then
    FAILED=1
  fi
  count=$(wc -l < "$TMP" | tr -d '[:space:]')
  if [ "$count" -gt 6 ]; then
    cat "$TMP" >&2
    echo "UnyDesk CSP audit failed: unexpected new innerHTML usage" >&2
    FAILED=1
  fi
fi

rm -f "$TMP"

if [ "$FAILED" -ne 0 ]; then
  echo "UnyDesk CSP audit failed" >&2
  exit 1
fi

echo "UnyDesk CSP audit passed"
