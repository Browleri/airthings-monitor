# Development

These notes describe a Debian/Ubuntu Linux development setup. They are separate
from the Raspberry Pi deployment steps in [INSTALL.md](INSTALL.md).

Local development should use the mock sensor by default and should keep runtime
data outside the repository.

## Packages

Install the packages needed for Go builds, SQLite CGO builds, frontend builds,
and optional Bluetooth inspection tools:

```sh
sudo apt update
sudo apt install -y git build-essential pkg-config sqlite3 libsqlite3-dev bluetooth bluez ca-certificates curl gnupg
```

Install Go 1.23 or newer:

```sh
go version
```

If the distro package is too old, install Go from the official Linux tarball.

Install Node.js and npm for the React frontend:

```sh
node --version
npm --version
```

Vite 7 expects Node.js 20.19 or newer, or Node.js 22.12 or newer. If your distro
package is older, install a current Node.js release before running frontend
commands.

## Local Configuration

Create a writable data directory outside the repository:

```sh
mkdir -p "$HOME/.local/share/airthings-monitor"
```

Copy the example config and edit it for local development:

```sh
cp config.example.toml config.toml
```

Set these values in `config.toml`:

```toml
sensor_mode = "mock"
database_path = "/home/you/.local/share/airthings-monitor/airthings.dev.db"
listen_address = "127.0.0.1:8080"
```

Replace `/home/you` with your actual home directory path. The config parser does
not expand `~` or environment variables.

## Build And Run

Run the backend tests and build the full app:

```sh
make test
make build
./bin/airthings-server -config config.toml
```

Open `http://127.0.0.1:8080`.

The Makefile stores Go build and module caches under `./cache` by default for
development. That directory is ignored by git.

## Frontend Development

For frontend-only work, run Vite directly:

```sh
cd web
npm ci
npm run dev
```

The Vite development server serves frontend assets only. For a full app smoke
test with API responses, run `make build` and serve `web/dist` through the Go
service.

## Raspberry Pi Differences

On the Pi, use [INSTALL.md](INSTALL.md) instead of this development guide. In
particular:

- use `sensor_mode = "ble"`
- keep `database_path` on `/mnt/pihole-usb/airthings`
- set `CACHE_ROOT=/mnt/pihole-usb/airthings/cache` when building on the Pi
- run the service through systemd as the `airthings` user
