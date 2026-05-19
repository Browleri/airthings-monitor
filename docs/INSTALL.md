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
not already present.

## Data Directory

Create the database directory on the USB drive. Do this only after confirming
the USB drive is mounted, so the service does not write runtime data to the SD
card by accident.

```sh
mount | grep /mnt/pihole-usb
sudo mkdir -p /mnt/pihole-usb/airthings
sudo chown airthings:airthings /mnt/pihole-usb/airthings
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
sudoedit /etc/airthings-monitor/config.toml
```

Keep `database_path` on the USB drive, for example:

```toml
database_path = "/mnt/pihole-usb/airthings/airthings.db"
```

## User And Service

```sh
sudo useradd --system --home /nonexistent --shell /usr/sbin/nologin airthings
sudo chown -R airthings:airthings /mnt/pihole-usb/airthings
sudo mkdir -p /opt/airthings-monitor
sudo cp -r bin web config.example.toml README.md docs /opt/airthings-monitor/
sudo cp deploy/systemd/airthings.service /etc/systemd/system/airthings.service
```

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

Then enable the service:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now airthings.service
sudo systemctl status airthings.service
```

Open `http://<pi-address>:8080`.
