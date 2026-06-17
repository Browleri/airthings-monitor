# Operations

## Normal Checks

```sh
systemctl status airthings.service
journalctl -u airthings.service -n 100 --no-pager
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/api/status
```

`/healthz` reports that the process is running. `/api/status` reports sensor
freshness, the last successful read time, and the last error.

For BLE debugging after reboot, recent logs should include the read flow:
adapter enabled, discovery start, target discovered, connection attempt,
services resolved, characteristic read, disconnect, or a clear retry reason.

Follow live logs with:

```sh
journalctl -u airthings.service -f
```

Check Bluetooth state with:

```sh
rfkill list
bluetoothctl info D8:71:4D:AA:78:34
```

## Updating The Service

After downloading repository updates on the Pi, rebuild and redeploy the binary
and frontend. Keep the Go build cache on the USB drive, not `/tmp`, to avoid
RAM pressure during builds.

The normal update path is:

```sh
cd /path/to/airthings-monitor
git pull --ff-only
scripts/deploy-local.sh
```

The script does not overwrite `/etc/airthings-monitor/config.toml`.

The manual workflow is:

```sh
cd /path/to/airthings-monitor
git pull --ff-only
make CACHE_ROOT=/mnt/pihole-usb/airthings/cache build
sudo systemctl stop airthings.service
sudo install -m 755 bin/airthings-server /opt/airthings-monitor/bin/airthings-server
sudo rm -rf /opt/airthings-monitor/web/dist
sudo cp -r web/dist /opt/airthings-monitor/web/dist
sudo cp -r README.md docs /opt/airthings-monitor/
sudo chown -R root:root /opt/airthings-monitor
sudo cp deploy/systemd/airthings.service /etc/systemd/system/airthings.service
sudo setcap 'cap_net_raw,cap_net_admin+eip' /opt/airthings-monitor/bin/airthings-server
sudo systemctl daemon-reload
sudo systemd-analyze verify /etc/systemd/system/airthings.service
sudo systemctl start airthings.service
sudo systemctl status airthings.service
```

If `config.example.toml` changed, compare it with the live config before
restarting:

```sh
diff -u config.example.toml /etc/airthings-monitor/config.toml
sudoedit /etc/airthings-monitor/config.toml
```

Do not blindly overwrite `/etc/airthings-monitor/config.toml`; it contains the
local database path, listen address, and tuning values for this Pi.

## Logging

The application writes compact structured logs to stdout/stderr. Do not add
application file logging by default; journald and log2ram should handle local
log durability and SD-card wear concerns.

## Polling

The service performs one BLE read per poll interval. By default:

- CO2 is stored every minute.
- VOC, temperature, humidity, and pressure are stored every five minutes.
- Radon short-term and long-term are stored every sixty minutes.

These intervals are configured in `/etc/airthings-monitor/config.toml`.

## Retries And Staleness

BLE read failures are non-fatal. The service retries with jitter rather than a
fixed loop so reconnect attempts do not repeatedly land on the same bad timing
after Bluetooth or radio hiccups. Defaults:

- `min_retry_delay = "45s"`
- `max_retry_delay = "90s"`
- `ble_discovery_timeout = "20s"`
- `ble_connect_timeout = "20s"`
- `ble_services_timeout = "15s"`
- `ble_read_timeout = "5s"`
- `ble_disconnect_timeout = "5s"`

The frontend shows a stale data warning when the last successful sensor read is
older than `stale_after`, which defaults to `10m`. Stale readings do not make
`/healthz` fail; use `/api/status` to distinguish live, stale, Bluetooth retry,
and database states.

The BLE timeout settings are used for discovery bounds, connection parameters,
overall read bounding, and slow-phase warnings. Some TinyGo/BlueZ D-Bus calls
are synchronous, so the client serializes BLE operations to avoid overlapping
retries if BlueZ returns later than the configured timeout.

## Retention

`retention_days` controls old-row cleanup. Cleanup runs on
`retention_cleanup_interval`. Set `retention_days = 0` to disable cleanup.

## Shutdown

The service handles SIGTERM from systemd and shuts down the HTTP server
gracefully. An in-flight BLE read may finish after cancellation depending on the
BlueZ call state, but no extra process is spawned.
