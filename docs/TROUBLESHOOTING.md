# Troubleshooting

## Bluetooth Device Not Found

- Confirm the sensor address in config: `D8:71:4D:AA:78:34`.
- Confirm Bluetooth is up: `bluetoothctl show`.
- Scan nearby devices: `bluetoothctl scan on`.
- Move the Pi closer to the sensor and retry.

## BlueZ Device Missing After Reboot

If logs contain an error like:

```text
Method "Get" with signature "ss" on interface "org.freedesktop.DBus.Properties" doesn't exist
```

and `bluetoothctl info D8:71:4D:AA:78:34` says `Device not available`, BlueZ has
not created a device object for the sensor yet. This can happen after reboot.

The service discovery flow is designed to handle this by powering the adapter,
starting BLE discovery, waiting for the configured MAC address to appear,
connecting to the discovered device, waiting for services to resolve, reading
the measurement characteristic, and disconnecting.

Useful checks:

```sh
bluetoothctl show
bluetoothctl scan on
bluetoothctl info D8:71:4D:AA:78:34
busctl tree org.bluez
journalctl -u airthings.service -n 100 --no-pager
```

During a healthy read, logs should show discovery start, target discovered,
connection attempt, services resolved, characteristic read, and disconnect.

## Permission Denied On BLE

Check the service user and capabilities:

```sh
getcap /opt/airthings-monitor/bin/airthings-server
id airthings
journalctl -u airthings.service -n 100 --no-pager
```

Depending on Raspberry Pi OS and BlueZ policy, the binary may need:

```sh
sudo setcap 'cap_net_raw,cap_net_admin+eip' /opt/airthings-monitor/bin/airthings-server
```

If `id airthings` fails, create the service user from the install guide before
starting the unit.

## SQLite Database Path Missing

The service deliberately refuses to create the database parent directory. This
helps avoid silently writing to the SD card if the USB mount is missing.

```sh
mount | grep /mnt/pihole-usb
ls -ld /mnt/pihole-usb/airthings
```

Create and chown the directory only after verifying the USB drive is mounted.
If `chown airthings:airthings` reports `invalid user`, create the service user
first:

```sh
sudo getent group airthings >/dev/null || sudo groupadd --system airthings
id -u airthings >/dev/null 2>&1 || sudo useradd --system --gid airthings --home /nonexistent --shell /usr/sbin/nologin airthings
```

## Sensor Stale

`/api/status` reports `stale = true` when the last successful read is older than
`stale_after`. Check:

- sensor battery
- distance from Pi
- Bluetooth service health
- recent logs in `journalctl -u airthings.service`

## Service Starts Before Bluetooth Is Ready

The systemd unit has `After=bluetooth.service` and `Wants=bluetooth.service`.
If the adapter is still slow to appear, add a conservative delay:

```ini
ExecStartPre=/bin/sleep 10
```

Prefer fixing Bluetooth startup first; use a delay only if the Pi consistently
needs it.

## USB Drive Missing

If `/mnt/pihole-usb/airthings` is absent or owned by the wrong user, the service exits
and systemd retries. This is intentional. Restore the USB mount and then run:

```sh
sudo systemctl restart airthings.service
```

The default systemd service includes `RequiresMountsFor=/mnt/pihole-usb/airthings`
so systemd waits for the USB-backed data path before starting the monitor.
