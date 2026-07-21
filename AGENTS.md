# AGENTS.md

## Project Goals

Build a lightweight, maintainable Raspberry Pi indoor air monitoring service for
an Airthings Wave Plus. The Pi also runs Pi-hole, so this project must stay small,
quiet, and operationally conservative.

## Hard Constraints

- Do not destabilize Pi-hole.
- Do not write runtime data into the repository.
- Do not default runtime writes to the microSD card.
- Default persistent data belongs on the USB drive, for example:
  `/mnt/pihole-usb/airthings/airthings.db`.
- Recent measurement loss during sudden power loss is acceptable.
- SD card or system-state corruption is not acceptable.
- Do not add Docker, InfluxDB, Home Assistant, or Grafana to this version.

## Durability Priorities

- SQLite uses WAL mode and `synchronous=NORMAL`.
- Logs go to stdout/stderr for systemd/journald and log2ram.
- Avoid noisy file logging.
- Keep polling and retention cleanup intervals configurable.
- Keep retry jitter configurable; avoid fixed retry loops that can repeatedly
  collide with transient Bluetooth timing failures.
- Keep write frequency intentional: CO2 every minute, environment every five
  minutes, radon every sixty minutes by default.

## Documentation responsibilities

This repository owns the implementation.

The central documentation repository (`Browleri/documentation-repo`) owns:

- infrastructure documentation
- architecture documentation
- host documentation
- service documentation
- operational procedures
- network documentation

Whenever changes in this repository materially affect deployment,
architecture, networking, operations, monitoring or maintenance,
update the documentation repository as part of the same work item.

Do not duplicate large documentation blocks here.
Keep README focused on this repository.

## Coding Conventions

- Prefer standard library code where practical.
- Keep dependencies modest and justified.
- Keep packages narrow:
  - `internal/airthings`: BLE client and payload decoding.
  - `internal/config`: config loading and validation.
  - `internal/db`: migrations and query/storage code.
  - `internal/scheduler`: polling cadence and sampling decisions.
  - `internal/httpapi`: REST handlers.
- Add tests for decoders, scheduler decisions, and HTTP handlers when behavior
  changes.
- Use context cancellation and graceful shutdown for long-running work.

## Operational Notes

- Treat `/mnt/pihole-usb/airthings` as the expected writable data location.
- A missing USB mount should produce clear errors, not silent fallback writes to
  the SD card.
- Keep resource use appropriate for a Raspberry Pi 3B+.
- Bluetooth permissions are deployment-sensitive; document them rather than
  hiding them in code.
- The current BLE implementation is Go-only. Do not add a Python helper unless
  the Go/BlueZ path is shown to be unreliable and the fallback is clearly
  isolated.
