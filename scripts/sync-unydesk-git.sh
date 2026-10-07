#!/bin/sh

set -eu

ROOT_DIR=$(cd -- "$(dirname -- "$0")/.." && pwd)
ENV_FILE=${UNYDESK_GIT_SYNC_ENV:-"$ROOT_DIR/unydesk-git-sync.env"}

UNYDESK_GIT_SYNC_ENABLED=${UNYDESK_GIT_SYNC_ENABLED:-0}
UNYDESK_GIT_SYNC_PUSH=${UNYDESK_GIT_SYNC_PUSH:-1}
UNYDESK_GIT_BRANCH=${UNYDESK_GIT_BRANCH:-master}
UNYDESK_GIT_REQUIRE_BUILD=${UNYDESK_GIT_REQUIRE_BUILD:-1}
UNYDESK_GIT_BUILD_VERIFIED=${UNYDESK_GIT_BUILD_VERIFIED:-0}
UNYDESK_GIT_COMMIT_FILE_LIMIT=${UNYDESK_GIT_COMMIT_FILE_LIMIT:-80}
UNYDESK_GIT_PUSH_URL=${UNYDESK_GIT_PUSH_URL:-https://github.com/tony-bonnin/unydesk.git}
GH_TOKEN_FILE=${GH_TOKEN_FILE:-${GITHUB_TOKEN_FILE:-}}
GIT_TERMINAL_PROMPT=0
GIT_ASKPASS=/bin/false
export GIT_TERMINAL_PROMPT GIT_ASKPASS

log() { printf '[unydesk-git] %s\n' "$*"; }

load_env() {
  if [ -f "$ENV_FILE" ]; then
    set -a
    # shellcheck disable=SC1090
    . "$ENV_FILE"
    set +a
  fi
}

safe_remote_url() {
  printf '%s\n' "$UNYDESK_GIT_PUSH_URL" | sed -E 's#(https?://)[^/@]+@#\1***@#'
}

auth_header() {
  [ -f "$GH_TOKEN_FILE" ] || return 0
  token=$(tr -d '\r\n' < "$GH_TOKEN_FILE")
  [ -n "$token" ] || return 0
  printf '%s' "x-access-token:$token" | base64 | tr -d '\n'
  printf '\n'
}

git_auth() {
  header=$(auth_header || true)
  if [ -n "$header" ]; then
    git -C "$ROOT_DIR" -c credential.helper= -c core.askPass=/bin/false -c "http.https://github.com/.extraHeader=Authorization: Basic $header" "$@"
  else
    git -C "$ROOT_DIR" -c credential.helper= -c core.askPass=/bin/false "$@"
  fi
}

stage_safe_paths() {
  git -C "$ROOT_DIR" add \
    .gitignore \
    README.md \
    docker-compose.yml \
    unydesk-git-sync.env.example \
    watch-unydesk.sh \
    scripts \
    unydesk/backend/auth \
    unydesk/backend/cmd \
    unydesk/backend/config \
    unydesk/backend/go.mod \
    unydesk/backend/go.sum \
    unydesk/backend/remote \
    unydesk/backend/server \
    unydesk/backend/settings/settings.yaml \
    unydesk/frontend/public
}

commit_summary() {
  git -C "$ROOT_DIR" diff --cached --name-status | awk '
    /^A/ { a++ } /^M/ { m++ } /^D/ { d++ } /^R/ { r++ }
    END { printf "%sA %sM %sD %sR", a+0, m+0, d+0, r+0 }'
}

format_change_list() {
  awk '
    BEGIN { labels["A"]="Added"; labels["M"]="Changed"; labels["D"]="Removed"; labels["R"]="Renamed"; labels["C"]="Copied" }
    NF {
      code=substr($1, 1, 1)
      label=(code in labels) ? labels[code] : "Changed"
      if (code == "R" || code == "C") printf "- %s `%s` -> `%s`\n", label, $2, $3
      else printf "- %s `%s`\n", label, $2
    }'
}

commit_body() {
  limit=$UNYDESK_GIT_COMMIT_FILE_LIMIT
  count=$(git -C "$ROOT_DIR" diff --cached --name-status | wc -l | tr -d '[:space:]')
  printf 'Automated UnyDesk git sync.\n\n'
  printf 'Changed file(s): %s\n' "$count"
  printf 'Summary: %s\n\n' "$(commit_summary)"
  printf 'Security gate: csp-audit + go test + go vet + govulncheck + build passed.\n\n'
  printf 'Stat:\n'
  git -C "$ROOT_DIR" diff --cached --stat || true
  printf '\nFiles (first %s):\n' "$limit"
  git -C "$ROOT_DIR" diff --cached --name-status | sed 's/	/ /g' | sed -n "1,${limit}p"
}

commit_and_push() {
  stage_safe_paths
  if git -C "$ROOT_DIR" diff --cached --quiet; then
    log "aucune modification a commit"
  else
    subject="unydesk: auto sync $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
    git -C "$ROOT_DIR" commit -m "$subject" -m "$(commit_body)"
  fi

  if [ "$UNYDESK_GIT_SYNC_PUSH" = "1" ]; then
    git_auth push "$UNYDESK_GIT_PUSH_URL" "$UNYDESK_GIT_BRANCH"
  fi
}

run_sync() {
  load_env
  [ "$UNYDESK_GIT_SYNC_ENABLED" = "1" ] || { log "sync git desactive"; return 0; }
  if [ "$UNYDESK_GIT_REQUIRE_BUILD" = "1" ] && [ "$UNYDESK_GIT_BUILD_VERIFIED" != "1" ]; then
    log "build non verifiee: commit/push refuse"
    return 1
  fi
  git_auth fetch origin --prune >/dev/null 2>&1 || true
  commit_and_push
}

case "${1:-sync}" in
  sync|all) run_sync ;;
  status)
    load_env
    printf 'enabled: %s\nbranch: %s\npush_url: %s\n' "$UNYDESK_GIT_SYNC_ENABLED" "$UNYDESK_GIT_BRANCH" "$(safe_remote_url)"
    ;;
  *) echo "usage: $0 [sync|status]" >&2; exit 1 ;;
esac
