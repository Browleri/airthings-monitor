# Troubleshooting

## Bluetooth Device Not Found

- Confirm the sensor address in config: `D8:71:4D:AA:78:34`.
- Confirm Bluetooth is up: `bluetoothctl show`.
- Scan nearby devices: `bluetoothctl scan on`.
- Move the Pi closer to the sensor and retry.

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

## SQLite Database Path Missing

The service deliberately refuses to create the database parent directory. This
helps avoid silently writing to the SD card if the USB mount is missing.

```sh
mount | grep /mnt/pihole-usb
ls -ld /mnt/pihole-usb/airthings
```

Create and chown the directory only after verifying the USB drive is mounted.

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
