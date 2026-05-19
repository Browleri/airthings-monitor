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

## Retention

`retention_days` controls old-row cleanup. Cleanup runs on
`retention_cleanup_interval`. Set `retention_days = 0` to disable cleanup.

## Shutdown

The service handles SIGTERM from systemd and shuts down the HTTP server
gracefully. An in-flight BLE read may finish after cancellation depending on the
BlueZ call state, but no extra process is spawned.
