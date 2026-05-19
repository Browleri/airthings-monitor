# Database

## Location

The default database path is:

```text
/mnt/pihole-usb/airthings/airthings.db
```

Keep it on the USB drive rather than the microSD card.

## SQLite Settings

The service configures SQLite with:

- `PRAGMA journal_mode=WAL`
- `PRAGMA synchronous=NORMAL`
- a configurable busy timeout
- one open connection from the Go process

WAL mode creates sidecar files next to the database:

- `airthings.db-wal`
- `airthings.db-shm`

These files are normal and should not be deleted while the service is running.

## Schema

The `readings` table has one row per successful sensor read. Columns are nullable
because not every metric is intentionally sampled at every timestamp.

```text
recorded_at
co2_ppm
voc_ppb
temperature_c
humidity_percent
pressure_hpa
radon_short_bqm3
radon_long_bqm3
raw_payload
```

Indexes support time-range queries and metric-specific chart queries.

## Backup

For a consistent online backup, use SQLite's backup command:

```sh
sqlite3 /mnt/pihole-usb/airthings/airthings.db ".backup '/mnt/pihole-usb/airthings/backup.db'"
```

To copy files manually, stop the service first so the database, WAL, and shared
memory files are in a simple state:

```sh
sudo systemctl stop airthings.service
cp /mnt/pihole-usb/airthings/airthings.db /mnt/pihole-usb/airthings/airthings.db.backup
sudo systemctl start airthings.service
```

If copying while the service is running, copy the database with SQLite tooling
rather than copying only the main `.db` file.

## Restore

Stop the service, move the old database aside, copy the backup into place, fix
ownership, and start the service.

```sh
sudo systemctl stop airthings.service
sudo mv /mnt/pihole-usb/airthings/airthings.db /mnt/pihole-usb/airthings/airthings.db.old
sudo cp backup.db /mnt/pihole-usb/airthings/airthings.db
sudo chown airthings:airthings /mnt/pihole-usb/airthings/airthings.db
sudo systemctl start airthings.service
```
