# Install

These notes assume Raspberry Pi OS Lite on a Raspberry Pi 3B+ that already runs
Pi-hole. The service does not require Docker.

## Packages

Install the system packages needed for Go, SQLite CGO builds, Node/Vite builds,
and Bluetooth:

```sh
sudo apt update
sudo apt install -y git build-essential pkg-config sqlite3 libsqlite3-dev bluetooth bluez
```

Install Go and Node.js using your preferred Raspberry Pi OS method if they are
not already present. This project expects Go 1.23 or newer. Vite 7 expects a
modern Node.js runtime; use Node.js 20.19 or newer, or Node.js 22.12 or newer.

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

```sh
make test
make build
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
