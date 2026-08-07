#!/usr/bin/env bash
# Builds Mangrove, starts it against a throwaway SQLite DB, runs the
# Playwright suite against it (real Docker, real Caddy -- nothing mocked),
# and tears everything down afterward regardless of outcome.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKDIR="$(mktemp -d /tmp/mangrove-e2e-XXXXXX)"
MANGROVE_BIN="$WORKDIR/mangrove"
DATA_DIR="$WORKDIR/data"
FIXTURE_DIR="$WORKDIR/fixtures/nginx-app"
LOG_FILE="$WORKDIR/mangrove.log"
PORT=7777

cleanup() {
  local code=$?
  echo "--- tearing down e2e run ---"
  [ -n "${MANGROVE_PID:-}" ] && kill -9 "$MANGROVE_PID" 2>/dev/null || true
  docker ps -a --filter "name=mangrove-" -q 2>/dev/null | xargs -r docker rm -f >/dev/null 2>&1 || true
  docker network ls --filter name=mangrove -q 2>/dev/null | xargs -r docker network rm >/dev/null 2>&1 || true
  rm -rf "$WORKDIR"
  exit $code
}
trap cleanup EXIT

echo "--- building mangrove ---"
cd "$ROOT"
go build -o "$MANGROVE_BIN" ./cmd/mangrove

echo "--- setting up fixture repo ---"
mkdir -p "$FIXTURE_DIR" "$DATA_DIR"
cat > "$FIXTURE_DIR/Dockerfile" <<'EOF'
FROM nginx:alpine
EOF
git -C "$FIXTURE_DIR" init -q
git -C "$FIXTURE_DIR" -c user.email=e2e@test.local -c user.name=e2e add Dockerfile
git -C "$FIXTURE_DIR" -c user.email=e2e@test.local -c user.name=e2e commit -q -m "e2e fixture"
git -C "$FIXTURE_DIR" branch -m main

echo "--- starting mangrove ---"
MANGROVE_DATA_DIR="$DATA_DIR" MANGROVE_PORT="$PORT" "$MANGROVE_BIN" > "$LOG_FILE" 2>&1 &
MANGROVE_PID=$!

for i in $(seq 1 30); do
  if curl -fs "http://127.0.0.1:$PORT/healthz" > /dev/null 2>&1; then
    break
  fi
  sleep 0.5
done
if ! curl -fs "http://127.0.0.1:$PORT/healthz" > /dev/null 2>&1; then
  echo "mangrove did not become healthy in time; log:"
  cat "$LOG_FILE"
  exit 1
fi

echo "--- running playwright ---"
cd "$ROOT/e2e"
MANGROVE_FIXTURE_DIR="$FIXTURE_DIR" MANGROVE_BASE_URL="http://127.0.0.1:$PORT" npx playwright test "$@"
