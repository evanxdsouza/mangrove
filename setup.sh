#!/usr/bin/env bash
# setup.sh -- bootstrap Mangrove on a fresh Debian/Ubuntu box: installs
# Docker, Caddy, Go, and Node if missing; builds the frontend + binary;
# installs the systemd units/slices from deploy/systemd/ exactly as
# documented in docs/deployment.md; starts the service; and creates the
# one-time admin account via the API so you don't have to do it through the
# browser's first-run screen.
#
# Usage:
#   sudo ./setup.sh
#
# Admin account: by default this prompts interactively for the admin email
# and password (the password entry is hidden, and confirmed once). To run
# unattended (e.g. from another provisioning script), set
# MANGROVE_ADMIN_EMAIL and MANGROVE_ADMIN_PASSWORD in the environment
# instead -- but don't commit real credentials into a script or repo; pass
# them at invocation time, e.g.:
#   sudo MANGROVE_ADMIN_EMAIL=me@example.com MANGROVE_ADMIN_PASSWORD='...' ./setup.sh
#
# Re-running is safe: every step either checks for existing state first or
# is naturally idempotent (useradd guarded, mkdir -p, systemd unit files
# overwritten with the repo's copy and reloaded, admin bootstrap skipped if
# an account already exists).
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANGROVE_SYSTEM_USER="mangrove"
MANGROVE_DATA_DIR="/var/lib/mangrove"
MANGROVE_STATIC_DIR="/var/lib/mangrove-static"
MANGROVE_ENV_DIR="/etc/mangrove"
MANGROVE_ENV_FILE="${MANGROVE_ENV_DIR}/mangrove.env"
MANGROVE_PORT="${MANGROVE_PORT:-7777}"
GO_MIN_VERSION="1.25.0"
GO_INSTALL_VERSION="1.25.0"
NODE_MIN_MAJOR=20

log()  { printf '\n\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Preflight
# ---------------------------------------------------------------------------

if [ "$(id -u)" -ne 0 ]; then
  die "must run as root (systemd units, apt, /etc, /var/lib, and a system user all need it) -- try: sudo ./setup.sh"
fi

if ! command -v apt-get >/dev/null 2>&1; then
  die "this script targets Debian/Ubuntu (apt-get not found). See docs/deployment.md and README.md's Quickstart to set up manually on another distro."
fi

if [ ! -f "${REPO_DIR}/go.mod" ] || [ ! -d "${REPO_DIR}/deploy/systemd" ]; then
  die "run this from inside the mangrove repo checkout (expected go.mod and deploy/systemd/ next to setup.sh)"
fi

version_ge() {
  # version_ge A B -- true if version A >= version B
  [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]
}

# ---------------------------------------------------------------------------
# System dependencies
# ---------------------------------------------------------------------------

install_docker() {
  if command -v docker >/dev/null 2>&1; then
    log "Docker already installed ($(docker --version))"
  else
    log "Installing Docker (official get.docker.com script)"
    curl -fsSL https://get.docker.com | sh
  fi
  systemctl enable --now docker
}

install_caddy() {
  if command -v caddy >/dev/null 2>&1; then
    log "Caddy already installed ($(caddy version))"
  else
    log "Installing Caddy (official apt repo)"
    apt-get update -qq
    apt-get install -y -qq debian-keyring debian-archive-keyring apt-transport-https curl gnupg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
      | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
      > /etc/apt/sources.list.d/caddy-stable.list
    chmod o+r /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    chmod o+r /etc/apt/sources.list.d/caddy-stable.list
    apt-get update -qq
    apt-get install -y -qq caddy
  fi
  systemctl enable --now caddy
}

install_go() {
  if command -v go >/dev/null 2>&1; then
    local have
    have="$(go version | awk '{print $3}' | sed 's/go//')"
    if version_ge "$have" "$GO_MIN_VERSION"; then
      log "Go already installed (go${have})"
      return
    fi
    warn "installed Go (go${have}) is older than go${GO_MIN_VERSION}; installing go${GO_INSTALL_VERSION} to /usr/local/go"
  else
    log "Installing Go ${GO_INSTALL_VERSION}"
  fi
  local arch tarball
  case "$(uname -m)" in
    x86_64)  arch=amd64 ;;
    aarch64) arch=arm64 ;;
    *) die "unsupported architecture $(uname -m) for the Go auto-install; install Go ${GO_MIN_VERSION}+ manually and re-run" ;;
  esac
  tarball="go${GO_INSTALL_VERSION}.linux-${arch}.tar.gz"
  curl -fsSL "https://go.dev/dl/${tarball}" -o "/tmp/${tarball}"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "/tmp/${tarball}"
  rm -f "/tmp/${tarball}"
  ln -sf /usr/local/go/bin/go /usr/local/bin/go
  ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
}

install_node() {
  if command -v node >/dev/null 2>&1; then
    local major
    major="$(node --version | sed 's/^v//' | cut -d. -f1)"
    if [ "$major" -ge "$NODE_MIN_MAJOR" ]; then
      log "Node already installed ($(node --version))"
      return
    fi
    warn "installed Node ($(node --version)) is older than v${NODE_MIN_MAJOR}; installing current LTS via NodeSource"
  else
    log "Installing Node.js (NodeSource, current LTS)"
  fi
  curl -fsSL https://deb.nodesource.com/setup_lts.x | bash -
  apt-get install -y -qq nodejs
}

install_docker
install_caddy
install_go
install_node
hash -r

# ---------------------------------------------------------------------------
# System user + directories
# ---------------------------------------------------------------------------

log "Creating ${MANGROVE_SYSTEM_USER} system user and data directories"
if ! id "$MANGROVE_SYSTEM_USER" >/dev/null 2>&1; then
  useradd --system --home-dir "$MANGROVE_DATA_DIR" --create-home --shell /usr/sbin/nologin "$MANGROVE_SYSTEM_USER"
fi
usermod -aG docker "$MANGROVE_SYSTEM_USER"

mkdir -p "$MANGROVE_DATA_DIR"
chown "$MANGROVE_SYSTEM_USER:$MANGROVE_SYSTEM_USER" "$MANGROVE_DATA_DIR"
chmod 700 "$MANGROVE_DATA_DIR"

mkdir -p "$MANGROVE_STATIC_DIR"
chown "$MANGROVE_SYSTEM_USER:$MANGROVE_SYSTEM_USER" "$MANGROVE_STATIC_DIR"
chmod 755 "$MANGROVE_STATIC_DIR"

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

log "Building the frontend (internal/webui/dist, embedded into the binary)"
(cd "${REPO_DIR}/web" && npm install --no-fund --no-audit && npm run build)

log "Building the mangrove binary"
export PATH="/usr/local/go/bin:${PATH}"
(cd "$REPO_DIR" && go build -o /tmp/mangrove-new ./cmd/mangrove)
# cp-then-mv, not a direct overwrite, so a running binary's inode stays
# valid for any process still executing it -- see docs/deployment.md.
cp /tmp/mangrove-new /usr/local/bin/mangrove-staged
mv /usr/local/bin/mangrove-staged /usr/local/bin/mangrove
rm -f /tmp/mangrove-new

# ---------------------------------------------------------------------------
# systemd units + resource slices
# ---------------------------------------------------------------------------

log "Installing systemd slices and the mangrove.service unit"
cp "${REPO_DIR}/deploy/systemd/mangrove.slice" "${REPO_DIR}/deploy/systemd/mangrove-deployments.slice" /etc/systemd/system/
mkdir -p /etc/systemd/system/caddy.service.d
cp "${REPO_DIR}/deploy/systemd/caddy-mangrove-slice.conf" /etc/systemd/system/caddy.service.d/mangrove-slice.conf
cp "${REPO_DIR}/deploy/systemd/mangrove.service" /etc/systemd/system/mangrove.service

mkdir -p "$MANGROVE_ENV_DIR"
if [ ! -f "$MANGROVE_ENV_FILE" ]; then
  touch "$MANGROVE_ENV_FILE"
fi
chown root:root "$MANGROVE_ENV_FILE"
chmod 600 "$MANGROVE_ENV_FILE"

# MANGROVE_CGROUP_PARENT ties deployment containers to
# mangrove-deployments.slice's hard memory ceiling -- left unset by
# config.go's default (dev-only), so production needs it set explicitly.
if ! grep -q '^MANGROVE_CGROUP_PARENT=' "$MANGROVE_ENV_FILE" 2>/dev/null; then
  echo "MANGROVE_CGROUP_PARENT=mangrove-deployments.slice" >> "$MANGROVE_ENV_FILE"
fi

# MANGROVE_BASE_DOMAIN is cosmetic only (used in deploy-success email copy,
# see internal/config/config.go) -- ask, but don't block setup on it.
if ! grep -q '^MANGROVE_BASE_DOMAIN=' "$MANGROVE_ENV_FILE" 2>/dev/null; then
  if [ -n "${MANGROVE_BASE_DOMAIN:-}" ]; then
    base_domain="$MANGROVE_BASE_DOMAIN"
  elif [ -t 0 ]; then
    read -r -p "Base domain for deployed apps to be suggested under (cosmetic only, blank to skip): " base_domain || true
  fi
  if [ -n "${base_domain:-}" ]; then
    echo "MANGROVE_BASE_DOMAIN=${base_domain}" >> "$MANGROVE_ENV_FILE"
  fi
fi

systemctl daemon-reload

log "Resource isolation: mangrove.slice + mangrove-deployments.slice + caddy under mangrove.slice"
systemctl restart caddy

# ---------------------------------------------------------------------------
# Start mangrove
# ---------------------------------------------------------------------------

log "Starting mangrove.service"
systemctl enable --now mangrove
systemctl restart mangrove

log "Waiting for mangrove to become healthy on 127.0.0.1:${MANGROVE_PORT}"
for _ in $(seq 1 30); do
  if curl -sf "http://127.0.0.1:${MANGROVE_PORT}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if ! curl -sf "http://127.0.0.1:${MANGROVE_PORT}/healthz" >/dev/null 2>&1; then
  die "mangrove did not become healthy within 30s -- check: systemctl status mangrove && journalctl -u mangrove -n 100"
fi
log "mangrove is up"

# ---------------------------------------------------------------------------
# One-time admin account
# ---------------------------------------------------------------------------

api="http://127.0.0.1:${MANGROVE_PORT}"
setup_required="$(curl -sf "${api}/api/auth/status" | grep -o '"setup_required":[a-z]*' | cut -d: -f2)"

if [ "$setup_required" = "true" ]; then
  log "Creating the admin account"
  admin_email="${MANGROVE_ADMIN_EMAIL:-}"
  admin_password="${MANGROVE_ADMIN_PASSWORD:-}"

  if [ -z "$admin_email" ] || [ -z "$admin_password" ]; then
    if [ ! -t 0 ]; then
      die "no admin account exists yet and this isn't an interactive terminal -- set MANGROVE_ADMIN_EMAIL and MANGROVE_ADMIN_PASSWORD and re-run"
    fi
    [ -z "$admin_email" ] && read -r -p "Admin email: " admin_email
    while [ -z "$admin_password" ]; do
      read -r -s -p "Admin password (min 8 characters): " admin_password; echo
      if [ "${#admin_password}" -lt 8 ]; then
        warn "password must be at least 8 characters"
        admin_password=""
        continue
      fi
      read -r -s -p "Confirm admin password: " admin_password_confirm; echo
      if [ "$admin_password" != "$admin_password_confirm" ]; then
        warn "passwords didn't match, try again"
        admin_password=""
      fi
    done
  fi

  resp="$(curl -sf -X POST "${api}/api/auth/setup" \
    -H 'Content-Type: application/json' \
    -d "$(printf '{"email":%s,"password":%s}' \
          "$(printf '%s' "$admin_email" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')" \
          "$(printf '%s' "$admin_password" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')")")" \
    || die "failed to create the admin account -- mangrove is running, but you'll need to complete first-run setup in the browser instead"
  unset admin_password admin_password_confirm MANGROVE_ADMIN_PASSWORD
  log "Admin account created for ${admin_email}"
else
  log "Admin account already exists -- skipping bootstrap"
fi

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------

log "Setup complete"
cat <<EOF

  Mangrove is running at:      http://127.0.0.1:${MANGROVE_PORT}
  Service status:               systemctl status mangrove
  Logs:                          journalctl -u mangrove -f
  Secrets (Resend key, etc.):    ${MANGROVE_ENV_FILE} (then: systemctl restart mangrove)
  Update the binary later:       see docs/deployment.md "Updating the binary"

  Point Caddy/DNS at this box on 80/443 to serve deployed apps publicly --
  see docs/deployment.md for anything beyond this first-boot setup.
EOF
