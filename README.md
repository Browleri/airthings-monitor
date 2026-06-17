# Airthings Monitor

Lightweight self-hosted indoor air monitoring for an Airthings Wave Plus on a
Raspberry Pi.

The service is designed for a Pi 3B+ that already runs Pi-hole. It keeps runtime
writes off the microSD card by default, stores measurements in SQLite on a
configurable USB-drive path, serves a small React frontend, and logs to
stdout/stderr for systemd/journald.

## Architecture

- Go service in `cmd/airthings-server`
- BLE reading and Airthings payload decoding in `internal/airthings`
- SQLite migrations and queries in `internal/db`
- Sampling cadence and polling in `internal/scheduler`
- REST API in `internal/httpapi`
- React + TypeScript + Vite frontend in `web`
- systemd deployment example in `deploy/systemd`

The service performs one BLE read per poll. The Airthings payload contains all
metrics, then the scheduler stores each metric group at its configured cadence:

- CO2: every minute by default
- VOC, temperature, humidity, pressure: every five minutes by default
- Radon short-term and long-term: every sixty minutes by default

Rows use nullable metric columns to make intentional skipped samples explicit.

## Hardware And Software

- Raspberry Pi 3B+
- Raspberry Pi OS Lite
- Airthings Wave Plus
- BlueZ Bluetooth stack
- Go
- Node.js and npm for frontend builds
- SQLite

Verified sensor details:

- BLE address: `D8:71:4D:AA:78:34`
- Measurements characteristic UUID:
  `b42e2a68-ade7-11e4-89d3-123b93f75cba`

## Features

**Dashboard**

- Seven metric cards (CO2, VOC, temperature, humidity, pressure, radon short-term
  and long-term) with live values, trend arrows, and threshold colour coding.
- Air quality badge in the header summarises the worst current threshold breach
  across all metrics as Good / Fair / Poor.
- Interactive line chart for each metric with configurable time range
  (1 h, 24 h, 7 d, 30 d), threshold band overlays, hover details, and
  min / avg / max summary drawn from the database.
- Temperature unit toggle (°C / °F) stored in the browser.
- Dark mode with automatic system-preference detection and a manual override
  toggle stored in the browser.
- Radon cards show WHO and EU reference values inline.

**Notifications**

- Optional push notifications via [ntfy.sh](https://ntfy.sh) or a self-hosted
  ntfy server. Notifications fire when a metric crosses a threshold band boundary
  (good → bad → critical) and on recovery. A configurable cooldown prevents
  repeat notifications while a metric stays in the same band.
- Subscribe to the configured topic in the ntfy mobile app on any device.

## Quick Start

For Debian/Ubuntu development, start with
[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md). For Raspberry Pi deployment, use
[docs/INSTALL.md](docs/INSTALL.md).

```sh
cp config.example.toml config.toml
mkdir -p "$HOME/.local/share/airthings-monitor"
```

For local development without BLE hardware, set:

```toml
sensor_mode = "mock"
database_path = "/home/you/.local/share/airthings-monitor/airthings.dev.db"
listen_address = "127.0.0.1:8080"
```

Replace `/home/you` with your actual home directory path. The config parser does
not expand `~` or environment variables.

Then run:

```sh
make test
make build
./bin/airthings-server -config config.toml
```

Open `http://127.0.0.1:8080`.

On the Pi, keep the production database on the USB drive:

```toml
database_path = "/mnt/pihole-usb/airthings/airthings.db"
```

The service refuses to create a missing database parent directory. This is
intentional so a missing USB mount does not silently redirect writes to the SD
card.

## Configuration

Configuration is TOML. Start from `config.example.toml`.

Important defaults:

```toml
sensor_address = "D8:71:4D:AA:78:34"
database_path = "/mnt/pihole-usb/airthings/airthings.db"
listen_address = "0.0.0.0:8080"
poll_interval = "1m"
min_retry_delay = "45s"
max_retry_delay = "90s"
co2_interval = "1m"
environment_interval = "5m"
radon_interval = "60m"
stale_after = "10m"
ble_discovery_timeout = "20s"
ble_connect_timeout = "20s"
ble_services_timeout = "15s"
ble_read_timeout = "5s"
ble_disconnect_timeout = "5s"
sqlite_journal_mode = "WAL"
sqlite_synchronous = "NORMAL"
```

Graph threshold bands are configured in the `[thresholds]` section. Defaults
are included for CO2, VOC, temperature, humidity, and radon. Pressure is
intentionally omitted because it is useful trend data rather than a direct
good/bad/critical indoor-air quality metric.

Push notifications are configured in the optional `[notifications]` section:

```toml
[notifications]
ntfy_url = "https://ntfy.sh/your-unique-topic"
notify_cooldown = "1h"
```

Set `ntfy_url` to any ntfy-compatible topic URL. Leave the section absent or
`ntfy_url` empty to disable notifications. Subscribe to the topic in the ntfy
mobile app to receive alerts. The `notify_cooldown` (default 1 h) is the
minimum gap between repeated notifications for the same metric when it stays
in the same level; escalations (bad → critical) are always sent immediately.

## API

- `GET /healthz`
- `GET /api/status`
- `GET /api/current`
- `GET /api/readings?metric=co2&range=24h`
- `GET /api/summary?range=7d`
- `GET /api/thresholds`

Supported ranges are `1h`, `24h`, `7d`, and `30d`.

Supported metrics are `co2`, `voc`, `temperature`, `humidity`, `pressure`,
`radon_short`, and `radon_long`.

`/api/status` includes `sensor_stale`, `last_successful_read`, `last_error`,
`last_error_at`, `database_ok`, and `bluetooth_ok` so the frontend can show
degraded sensor state without treating the HTTP service as unhealthy.

The frontend graphs use timestamp-based x-axis spacing, visible axis values,
configurable green/yellow/red threshold bands, and hover details with local
date, time, and measured value.

## Deployment

See [docs/INSTALL.md](docs/INSTALL.md) for Raspberry Pi deployment steps and
[deploy/systemd/airthings.service](deploy/systemd/airthings.service) for the
systemd unit.

The service is intended to run as a non-root `airthings` user. Bluetooth access
usually requires capabilities on the binary or suitable BlueZ policy/group
configuration.

After pulling repository updates on the Pi, rebuild and restart with the
workflow in [docs/OPERATIONS.md](docs/OPERATIONS.md#updating-the-service).
For routine on-Pi deployment after prerequisites are in place, use
`scripts/deploy-local.sh`.

## Operations

- [docs/OPERATIONS.md](docs/OPERATIONS.md)
- [docs/DATABASE.md](docs/DATABASE.md)
- [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)

## Durability Notes

- SQLite WAL mode is enabled.
- SQLite `synchronous=NORMAL` is used by default.
- Only intentional metric samples are inserted.
- Logs stay minimal and go to journald.
- Runtime database files are ignored by git.
- No Docker, InfluxDB, Home Assistant, or Grafana dependency is used in this
  version.

## BLE Implementation

The first version uses `tinygo.org/x/bluetooth` directly from Go. The BLE client
is isolated behind the `airthings.Client` interface, so a small helper process
could be added later if a particular Raspberry Pi OS/BlueZ combination proves
unreliable.

The current implementation is Go-only and no Python helper is needed. On each
sensor read, the service discovers the configured MAC address before connecting
so BlueZ has a device object after reboot. Intermittent BLE failures such as
`le-connection-abort-by-local` are expected on Raspberry Pi/BlueZ setups and are
retried automatically with jitter between `min_retry_delay` and
`max_retry_delay`.

## License

No license has been chosen yet. See [LICENSE](LICENSE).
