# Raspberry Pi Deployment

These notes assume Raspberry Pi OS Lite on a Raspberry Pi 3B+ that already runs
Pi-hole. They cover production-style deployment on the Pi, not day-to-day Linux
development. For a Debian/Ubuntu development setup, see
[DEVELOPMENT.md](DEVELOPMENT.md).

The service does not require Docker.

## Packages

Install the system packages needed for Go, SQLite CGO builds, Node/Vite builds,
and Bluetooth:

```sh
sudo apt update
sudo apt install -y git build-essential pkg-config sqlite3 libsqlite3-dev bluetooth bluez ca-certificates curl gnupg
```

## Go

This project expects Go 1.23 or newer.

First check whether Go is already installed and new enough:

```sh
go version
```

If Go is missing or too old, install the current stable Go release from the
official Go Linux tarball. These commands detect the common Raspberry Pi OS
architectures:

```sh
GO_VERSION="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n 1)"
case "$(uname -m)" in
  aarch64) GO_ARCH="arm64" ;;
  armv6l|armv7l) GO_ARCH="armv6l" ;;
  x86_64) GO_ARCH="amd64" ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

curl -fsSLo "/tmp/${GO_VERSION}.linux-${GO_ARCH}.tar.gz" \
  "https://go.dev/dl/${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "/tmp/${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
echo 'export PATH=/usr/local/go/bin:$PATH' | sudo tee /etc/profile.d/go.sh
. /etc/profile.d/go.sh
go version
```

Log out and back in, or source `/etc/profile.d/go.sh`, if your shell cannot find
`go` after installation.

## Node.js

Node.js and npm are only needed to build the React frontend. They are not needed
at runtime once `web/dist` has been built and copied to `/opt/airthings-monitor`.

Vite 7 expects Node.js 20.19 or newer, or Node.js 22.12 or newer. Check the
installed version first:

```sh
node --version
npm --version
```

If Raspberry Pi OS already provides a new enough Node.js package, this is enough:

```sh
sudo apt install -y nodejs npm
node --version
npm --version
```

If the distro package is too old, install Node.js 22 from the NodeSource Debian
repository:

```sh
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs
node --version
npm --version
```

If you prefer not to install Node.js on the Pi, build the frontend on another
machine with a compatible Node.js version and deploy the generated `web/dist`
directory to the Pi.

## Service User

Create a dedicated unprivileged service user before creating or chowning the
data directory.

```sh
sudo getent group airthings >/dev/null || sudo groupadd --system airthings
id -u airthings >/dev/null 2>&1 || sudo useradd --system --gid airthings --home /nonexistent --shell /usr/sbin/nologin airthings
```

## Data Directory

Create the database directory on the USB drive. Do this only after confirming
the USB drive is mounted, so the service does not write runtime data to the SD
card by accident.

```sh
mount | grep /mnt/pihole-usb
sudo mkdir -p /mnt/pihole-usb/airthings
sudo chown airthings:airthings /mnt/pihole-usb/airthings
sudo chmod 750 /mnt/pihole-usb/airthings
```

## Build

On Raspberry Pi, do not place `GOCACHE` or `GOMODCACHE` under `/tmp` because
`/tmp` is tmpfs/RAM-backed. Use the USB drive instead:

```sh
make CACHE_ROOT=/mnt/pihole-usb/airthings/cache test
make CACHE_ROOT=/mnt/pihole-usb/airthings/cache build
```

The web build is copied into `web/dist`, and the Go service serves that directory.

## Configuration

```sh
sudo mkdir -p /etc/airthings-monitor
sudo cp config.example.toml /etc/airthings-monitor/config.toml
sudo chown root:airthings /etc/airthings-monitor/config.toml
sudo chmod 640 /etc/airthings-monitor/config.toml
sudoedit /etc/airthings-monitor/config.toml
```

Keep `database_path` on the USB drive, for example:

```toml
database_path = "/mnt/pihole-usb/airthings/airthings.db"
```

## Install Files And Service

After the service user, data directory, and config file are ready, the local
deployment script can build and install the service on the Pi:

```sh
scripts/deploy-local.sh
```

The script does not overwrite `/etc/airthings-monitor/config.toml`. It builds
first, then stops the service, installs the binary and frontend, refreshes the
systemd unit, verifies it, and starts the service again.

Manual installation steps are:

```sh
sudo mkdir -p /opt/airthings-monitor
sudo mkdir -p /opt/airthings-monitor/bin
sudo mkdir -p /opt/airthings-monitor/web
sudo install -m 755 bin/airthings-server /opt/airthings-monitor/bin/airthings-server
sudo rm -rf /opt/airthings-monitor/web/dist
sudo cp -r web/dist /opt/airthings-monitor/web/dist
sudo cp -r README.md docs /opt/airthings-monitor/
sudo chown -R root:root /opt/airthings-monitor
sudo cp deploy/systemd/airthings.service /etc/systemd/system/airthings.service
```

Do not copy `web/node_modules` to `/opt`; only the built `web/dist` directory is
needed at runtime.

The default `frontend_dir = "web/dist"` is relative to
`WorkingDirectory=/opt/airthings-monitor` in the systemd unit.

The default service includes:

```ini
RequiresMountsFor=/mnt/pihole-usb/airthings
ReadWritePaths=/mnt/pihole-usb/airthings
```

This makes systemd wait for the USB-backed data path and keeps the service's
writable filesystem access scoped to that directory.

Bluetooth permissions vary by BlueZ/Raspberry Pi OS version. Start with the
capabilities documented in the service file:

```sh
sudo setcap 'cap_net_raw,cap_net_admin+eip' /opt/airthings-monitor/bin/airthings-server
```

Before enabling the service, check the paths and permissions:

```sh
sudo -u airthings test -r /etc/airthings-monitor/config.toml
sudo -u airthings test -w /mnt/pihole-usb/airthings
sudo test -x /opt/airthings-monitor/bin/airthings-server
sudo test -f /opt/airthings-monitor/web/dist/index.html
sudo systemd-analyze verify /etc/systemd/system/airthings.service
```

Then enable the service:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now airthings.service
sudo systemctl status airthings.service
```

Open `http://<pi-address>:8080`.

For later rebuilds after pulling repository updates, use the update workflow in
[OPERATIONS.md](OPERATIONS.md).
