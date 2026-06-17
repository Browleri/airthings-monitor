# Development Ideas

Ideas for features, optimizations, and other improvements. Each entry
includes an effort estimate and notes on constraints relevant to this project.

Effort scale:
- **XS** — an hour or two, single file change
- **S** — half a day, frontend-only or one small backend change
- **M** — 1–3 days, backend + frontend together
- **L** — 1 week+, architectural change or new subsystem
- **XL** — multi-week, touches many packages and deployment

---

## Usability — making the app more engaging for everyday use

### ~~Air quality plain-language summary~~ [done]

Replace the raw metric display at the top with a single sentence like
"Air quality is **Good**" or "CO2 is elevated — consider ventilating."
The threshold bands are already configured in `config.toml` and returned by
`/api/thresholds`; this is a frontend computation only.

Add a colour-coded badge (green / amber / red) that reflects the worst
current threshold breach across all metrics. This is the highest-value
single change for a non-technical user checking in from outside the home.

*Frontend-only. No new dependencies.*

---

### ~~Trend arrows on metric cards~~ [done]

Each metric card currently shows the latest value and nothing else. Compare
the latest reading to the reading from 30 minutes ago (available in
`/api/readings?metric=X&range=1h`) and show a ↑ ↓ → indicator. A rising
CO2 trend is more actionable than the absolute number alone.

*Frontend-only. Requires one extra fetch per page load.*

---

### ~~Surface the summary panel~~ [done]

The `/api/summary` endpoint returns min, max, and average for all metrics
over any time range. The frontend never calls it. Add a small summary row
below the chart (or below the metric grid) showing "Today: min X · avg Y ·
max Z" for the selected metric. This gives context that the current reading
alone does not.

*Frontend-only. No new API work needed.*

---

### ~~Temperature unit toggle (°C / °F)~~ [done]

The sensor reports Celsius. An F toggle stored in localStorage would make the
app immediately more comfortable for users in households that think in
Fahrenheit. Conversion is `(C * 9/5) + 32`.

*Frontend-only. One state variable and a conversion function.*

---

### ~~Radon plain-language context~~ [done]

Radon readings (Bq/m³) are unfamiliar to most people. Add a static tooltip
or footnote on the radon cards explaining:
- WHO guideline: 100 Bq/m³
- EU action level: 300 Bq/m³
- Short-term (30-day) average is the meaningful number

This turns a confusing number into something a user can act on.

*Frontend-only. Static text.*

---

### ~~Dark mode~~ [done]

The CSS already uses a consistent colour palette. Wrap the colour values in
CSS custom properties and add a `@media (prefers-color-scheme: dark)` block
with a dark palette. Optionally add a manual toggle stored in localStorage.

*Frontend-only (CSS + one toggle component). No new dependencies.*

---

### Named sensor / room labels [XS → S]

The header currently shows the raw MAC address
(`Sensor D8:71:4D:AA:78:34`). Add an optional `sensor_name` field to
`config.toml` (e.g., `sensor_name = "Living Room"`). The API already returns
`sensor_address`; add `sensor_name` alongside it and display it in the header.

*XS if config + display only. S if it also becomes the label in a
multi-sensor view (see below).*

---

### Multi-chart overview (sparklines) [M]

The current layout shows one metric chart at a time. A secondary "overview"
tab or view could show all seven metrics as small sparkline charts
simultaneously — useful for spotting correlation (CO2 rising at the same time
as VOC is a different problem than CO2 alone).

This requires fetching all seven metrics in parallel on page load. The
24h range is reasonable for sparklines; add `Promise.all` over seven
`/api/readings` calls.

*Frontend-only. No new API work. Moderate layout work.*

---

### Configurable longer time ranges [S]

The time range selector is hardcoded to `1h / 24h / 7d / 30d`. With 400 days
of retention configured, a 90d or 1y view would be useful for seeing seasonal
patterns (radon and pressure vary with season). The backend `rangeStart`
function in `internal/httpapi/server.go` would need two new cases; the
frontend selector gets two new buttons.

*Small backend change (one switch case) + small frontend change.*

---

## Notifications — proactive alerts

### Browser Web Push notifications [M]

The browser Push API allows sending a notification to a subscribed device
even when the tab is closed — no external service needed for the
subscription itself. The service worker handles delivery.

Implementation:
1. Add a service worker (`web/sw.js`) that handles `push` events.
2. Add a subscribe flow in the frontend (ask for notification permission,
   POST the push subscription to a new `/api/push/subscribe` endpoint).
3. Store subscriptions in SQLite (a new `push_subscriptions` table).
4. In the poller, after each successful read, check whether any metric
   crossed a threshold boundary and send a Web Push notification to all
   subscriptions using the VAPID protocol.

The VAPID signing and push delivery would require one new Go dependency
(e.g., `github.com/SherClockHolmes/webpush-go`). Justify carefully —
this is the single largest dependency addition the project has considered.

*Medium backend + frontend. Requires a new Go dependency and a new DB table.
High value for a spouse who wants to be alerted without opening the app.*

---

### ~~ntfy.sh push notifications (server-side only)~~ [done]

A lighter alternative to Web Push: POST a message to a self-hosted or public
`ntfy.sh` topic whenever a threshold is breached. The user subscribes to that
topic in the ntfy mobile app. No service worker, no VAPID keys, no new DB
table. Add a `[notifications]` section to `config.toml` with the topic URL
and the metrics/thresholds that should trigger a notification.

The trade-off: requires the ntfy app installed on each device, and either
trusting ntfy.sh (public, free) or self-hosting ntfy (another service on the
Pi, which competes with the constraint of not destabilising Pi-hole).

*S effort, but evaluate the operational cost of a second service on the Pi.*

---

## Multi-sensor support

### Multiple sensors / rooms [L]

Currently the architecture assumes exactly one BLE sensor: one poller, one
`sensor_address`, one set of current readings. Supporting multiple sensors
(e.g., bedroom + living room) would require:

1. `config.toml`: replace single `[sensor]` block with `[[sensors]]` array,
   each with an address, name, and optional per-sensor intervals.
2. `internal/db`: add a `sensor_id` column to the `readings` table (new
   migration), update all queries to filter by sensor.
3. `internal/scheduler`: run one poller goroutine per sensor; share the DB
   but not the BLE adapter (BlueZ handles multiple connections, but test this
   on Pi 3B+ — BLE throughput is limited).
4. `internal/httpapi`: all endpoints need a `?sensor=X` parameter (or
   return all sensors in a single response).
5. Frontend: add a room/sensor selector; the sparkline overview view
   becomes the natural landing page.

This is the highest-value long-term feature given the plan to add sensors,
but it is also the most invasive change. Do the schema migration work first,
since adding `sensor_id` retroactively is harder than doing it while the
table is small.

*L effort. Touches every package. Plan carefully before starting —
see `docs/AI_DEVELOPMENT_CHECKS.md` Package Boundary Check prompt before
beginning this work.*

---

## Data access

### CSV / JSON data export [S]

Add a `/api/export?metric=X&range=Y&format=csv` endpoint that streams the
raw readings as a CSV file. This lets a user download their data for use in
a spreadsheet — useful for sharing radon averages with a doctor or landlord,
or for seasonal analysis outside the app.

Implementation: one new HTTP handler, no new dependencies (Go's
`encoding/csv` is in the standard library). Return
`Content-Disposition: attachment; filename="co2_24h.csv"` so the browser
downloads rather than displays it.

*Backend-only. No new dependencies.*

---

## Performance and reliability

### Reduced frontend poll interval with staleness indication [XS]

The frontend polls `/api/current` and `/api/status` every 30 seconds. The
sensor itself only reads every 60 seconds by default. The poll could be
reduced to 70 seconds (slightly over one read interval) to halve the
HTTP traffic over a Cloudflare tunnel. Add a visual "updating…" flash when a
fresh reading arrives to keep the "live" feel.

*Frontend-only. One constant change.*

---

### Database VACUUM schedule [XS]

SQLite accumulates free pages over time as old readings are deleted by the
retention cleanup. An incremental `PRAGMA incremental_vacuum(100)` run once
per day in the scheduler would keep the database file from growing unboundedly
on the USB drive without the write amplification of a full VACUUM.

*One new scheduled call in `internal/scheduler/poller.go`. No new dependencies.*

---

### Pre-computed hourly averages for long ranges [M]

Loading 30 days of CO2 readings (one sample per minute = ~43,000 rows)
into the frontend is unnecessary for a 30d chart. Add a
`/api/readings?metric=X&range=30d&resolution=1h` query mode that returns
one row per hour (average of that hour's samples) rather than every
individual sample. This reduces the JSON payload by ~60× for the 30d view
and makes the chart render faster on a mobile connection.

Implementation: a new SQL query using `strftime` to group by hour, or a
new `readings_hourly` materialized view updated during retention cleanup.

*M effort. New SQL, new API parameter, no frontend dependency changes.*

---

## Developer experience

### Fix the known package boundary issue (db → scheduler) [S]

`internal/db` currently imports `internal/scheduler` to use
`scheduler.SampledReading` as its write DTO. This violates the package
boundary rule (db should not import scheduler). Extract a
`db.Reading` struct that contains the same nullable fields, and have the
scheduler construct it before calling db. This is the minimal change needed
to untangle the dependency.

See `docs/ISSUES.md` — Database Types Depend On Scheduler Types.

*S effort. Purely internal refactor, no API change, no behaviour change.*

---

### Embed migrations from files rather than string literals [XS]

The SQL schema exists both in `migrations/001_initial.sql` and as a string
literal in `internal/db/db.go`. Use `//go:embed migrations/*.sql` to read
migration files at build time and eliminate the duplicate. Future schema
changes then have one canonical location.

See `docs/ISSUES.md` — Migration SQL Exists In Two Places.

*XS effort. Go `embed` is already in the standard library.*

---

### Chart unit and scale extraction for testing [S]

The chart's scale functions (`xForTime`, `yForValue`, nearest-point
hit-testing) are embedded inside the `LineChart` component function. Extract
them to standalone pure functions in a separate file
(`web/src/chart-math.ts`). This makes them testable with Vitest without
rendering the full SVG, and is a prerequisite for catching chart regressions
automatically.

See `docs/ISSUES.md` — Frontend Chart Behaviour Has No Automated Coverage.

*S effort. No behaviour change.*
