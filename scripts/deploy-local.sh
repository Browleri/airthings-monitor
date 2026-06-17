#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

SERVICE_NAME="${SERVICE_NAME:-airthings.service}"
SERVICE_USER="${SERVICE_USER:-airthings}"
INSTALL_ROOT="${INSTALL_ROOT:-/opt/airthings-monitor}"
CONFIG_PATH="${CONFIG_PATH:-/etc/airthings-monitor/config.toml}"
DATA_DIR="${DATA_DIR:-/mnt/pihole-usb/airthings}"
CACHE_ROOT="${CACHE_ROOT:-/mnt/pihole-usb/airthings/cache}"
RUN_TESTS="${RUN_TESTS:-1}"
RESTART_SERVICE="${RESTART_SERVICE:-1}"

require_file() {
  if [ ! -f "$1" ]; then
    echo "missing required file: $1" >&2
    exit 1
  fi
}

require_dir() {
  if [ ! -d "$1" ]; then
    echo "missing required directory: $1" >&2
    exit 1
  fi
}

require_file "$CONFIG_PATH"
require_dir "$DATA_DIR"

if ! id "$SERVICE_USER" >/dev/null 2>&1; then
  echo "service user does not exist: $SERVICE_USER" >&2
  exit 1
fi

sudo -u "$SERVICE_USER" test -r "$CONFIG_PATH"
sudo -u "$SERVICE_USER" test -w "$DATA_DIR"

if [ "$RUN_TESTS" = "1" ]; then
  make CACHE_ROOT="$CACHE_ROOT" test
fi

make CACHE_ROOT="$CACHE_ROOT" build

require_file "bin/airthings-server"
require_file "web/dist/index.html"

if [ "$RESTART_SERVICE" = "1" ]; then
  sudo systemctl stop "$SERVICE_NAME" || true
fi

sudo mkdir -p "$INSTALL_ROOT/bin" "$INSTALL_ROOT/web"
sudo install -m 755 bin/airthings-server "$INSTALL_ROOT/bin/airthings-server"
sudo rm -rf "$INSTALL_ROOT/web/dist"
sudo cp -R web/dist "$INSTALL_ROOT/web/dist"
sudo cp -R README.md docs "$INSTALL_ROOT/"
sudo chown -R root:root "$INSTALL_ROOT"
sudo cp deploy/systemd/airthings.service "/etc/systemd/system/$SERVICE_NAME"

if command -v setcap >/dev/null 2>&1; then
  sudo setcap 'cap_net_raw,cap_net_admin+eip' "$INSTALL_ROOT/bin/airthings-server"
fi

sudo systemctl daemon-reload
sudo systemd-analyze verify "/etc/systemd/system/$SERVICE_NAME"

if [ "$RESTART_SERVICE" = "1" ]; then
  sudo systemctl start "$SERVICE_NAME"
  sudo systemctl status "$SERVICE_NAME" --no-pager
else
  echo "deployment complete; service restart skipped"
fi
