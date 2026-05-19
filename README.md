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

## Quick Start

```sh
cp config.example.toml config.toml
```

For local development without BLE hardware, set:

```toml
sensor_mode = "mock"
database_path = "./airthings.dev.db"
listen_address = "127.0.0.1:8080"
```

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
co2_interval = "1m"
environment_interval = "5m"
radon_interval = "60m"
sqlite_journal_mode = "WAL"
sqlite_synchronous = "NORMAL"
```

## API

- `GET /healthz`
- `GET /api/status`
- `GET /api/current`
- `GET /api/readings?metric=co2&range=24h`
- `GET /api/summary?range=7d`

Supported ranges are `1h`, `24h`, `7d`, and `30d`.

Supported metrics are `co2`, `voc`, `temperature`, `humidity`, `pressure`,
`radon_short`, and `radon_long`.

## Deployment

See [docs/INSTALL.md](docs/INSTALL.md) for Raspberry Pi installation steps and
[deploy/systemd/airthings.service](deploy/systemd/airthings.service) for the
systemd unit.

The service is intended to run as a non-root `airthings` user. Bluetooth access
usually requires capabilities on the binary or suitable BlueZ policy/group
configuration.

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

## License

No license has been chosen yet. See [LICENSE](LICENSE).
