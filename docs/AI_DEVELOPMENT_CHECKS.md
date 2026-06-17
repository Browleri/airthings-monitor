# AI Development Checks

Periodic maintenance prompts for keeping this codebase well-structured and the
service performing well on Raspberry Pi 3B+ hardware.

Run these in Claude Code (or any capable agent) with `AI_DEV_PROMPT.md` loaded
as context so the agent understands project constraints before acting.

---

## Monthly

### Dependency Audit

```
Review go.mod and web/package.json. For each dependency, confirm:
1. It is still actively used (grep for import paths / require calls).
2. No newer patch or minor version is available that fixes a known CVE or bug.
3. No dependency was added that duplicates standard library functionality.
Report findings only — do not update dependencies without explicit approval.
```

### Dead Code Scan

```
Search the Go source tree for unexported functions, types, and variables that
are never referenced outside their own file. Also look for exported symbols in
internal/ packages that are only used within the same package. Report any
candidates for removal. Do not delete anything — report only.
```

### Test Gap Review

```
List every public function and method in internal/ packages. For each one,
state whether a test exists that covers it, and whether the existing test
exercises error paths (not just the happy path). Flag gaps in:
- internal/airthings/decoder.go (edge-case payloads, corrupt bytes)
- internal/scheduler/sampling.go (boundary intervals, zero values)
- internal/httpapi/server.go (missing/invalid query parameters)
Report gaps as a prioritised list. Do not write tests — report only.
```

---

## Quarterly

### Package Boundary Check

```
Inspect import graphs across internal/ packages. The allowed import directions
are:
  cmd/airthings-server → internal/*  (all packages allowed)
  internal/httpapi     → internal/db, internal/config, internal/scheduler
  internal/scheduler   → internal/airthings, internal/db, internal/config
  internal/db          → internal/config  (no scheduler, no airthings)
  internal/airthings   → internal/config  (no db, no scheduler)
  internal/config      → (standard library only)

Report any import that violates these rules. In particular, check whether
internal/db still imports internal/scheduler (the SampledReading coupling
noted in docs/ISSUES.md). Suggest the minimal refactor to fix each violation.
Do not implement changes — report only.
```

### SQLite Health Check

```
Review internal/db/db.go and docs/DATABASE.md together. Verify:
1. WAL mode pragma is set at open time, not assumed.
2. busy_timeout is set before any write statement.
3. All partial indexes still match the query WHERE clauses in Readings() and
   Summary() — if a query changed, the index may no longer be used.
4. The retention DELETE in DeleteOlderThan() uses the recorded_at index.
5. The migrations/001_initial.sql file and the initialMigration string in
   db.go are byte-for-byte identical (drift risk noted in docs/ISSUES.md).
Report any discrepancies. Do not change SQL — report only.
```

### BLE Retry Logic Review

```
Review internal/airthings/ble.go and internal/scheduler/retry.go together.
Check:
1. Every BLE operation (scan, connect, read, disconnect) has a context
   deadline derived from the poll interval — no operation can block forever.
2. The retry jitter range (minRetryDelay / maxRetryDelay) still makes sense
   relative to the poll_interval in config.example.toml.
3. No fixed sleep or time.Sleep call exists that ignores context cancellation.
4. Error messages distinguish transient adapter errors from permanent config
   errors so operators can tell them apart in journalctl logs.
Report concerns. Do not change code — report only.
```

### Frontend Size and Structure Review

```
Read web/src/main.tsx. If the file exceeds 500 lines, evaluate whether any
of the following are good candidates for extraction into separate files:
- Chart rendering logic (SVG scale functions, nearest-point hit testing)
- API fetch layer (typed fetch wrappers for each /api/* endpoint)
- Threshold colour logic (good/bad/critical zone computation)
- Individual metric card component

For each candidate, describe what it would contain and how it would be
imported. The goal is to keep each file focused enough to read in one sitting.
Do not split files — report only.
```

### API Contract Consistency Check

```
Compare the API endpoints documented in README.md with the handlers registered
in internal/httpapi/server.go. For each endpoint:
1. Confirm the path, method, and query parameters match.
2. Confirm the JSON field names in the response structs match what the
   frontend (web/src/main.tsx) actually reads.
3. Confirm /api/thresholds returns all six metrics that have configured
   threshold bands in internal/config/config.go.
Report any mismatch between docs, server code, and frontend consumption.
```

---

## Semi-Annually

### Resource Usage Audit (Pi 3B+ Focus)

```
Review the polling and scheduling code for anything that could accumulate
memory or goroutines over long uptimes (weeks to months on a Pi 3B+).
Check:
1. internal/scheduler/poller.go — no goroutine leak on retry; the ticker
   is stopped on shutdown.
2. internal/db/db.go — no unbounded result set is loaded into memory;
   Readings() applies a LIMIT or a time window.
3. web/src/main.tsx — the 30-second polling setInterval is cleared on
   component unmount; no dangling event listeners.
4. The HTTP server does not hold database connections open beyond a single
   request lifetime.
Report any concern with an explanation of the failure mode over time.
```

### Systemd Unit Hardening Review

```
Read deploy/systemd/airthings.service. Compare its security directives against
current systemd-analyze security recommendations for a service that only needs:
- Bluetooth socket access (CAP_NET_RAW, CAP_NET_ADMIN)
- Read-write access to /mnt/pihole-usb/airthings
- Outbound TCP on the listen_address port
- No network access beyond localhost

Suggest any additional hardening directives that would not break the above
requirements (e.g. RestrictAddressFamilies, IPAddressDeny, MemoryMax).
Do not edit the unit file — report only.
```

### Configuration Validation Completeness

```
Review internal/config/config.go. For every field in the Config struct:
1. Confirm there is a default value or a clear error if none is provided.
2. Confirm the validation message names the field and gives a corrective hint.
3. Check whether any new interval or threshold field added since the last
   review lacks a range check (e.g. a zero poll_interval would loop tightly).

Also confirm that config.example.toml documents every field that exists in
the Config struct, including any recently added fields.
Report gaps. Do not change config code — report only.
```

### Offline Build Workflow Gap

```
Review docs/INSTALL.md and docs/DEVELOPMENT.md. The current workflow requires
npm registry access during `npm ci` on the Pi (noted in docs/ISSUES.md).

Evaluate two options and recommend one:
Option A: Document a workstation-side `make web-build` step that produces a
  tarball of web/dist, which is then copied to the Pi and unpacked before
  `make build` is run with a flag that skips the npm step.
Option B: Add an npm offline-pack script that runs `npm pack` for each
  dependency and commits a vendor tarball, allowing `npm ci --offline`.

Consider the Pi's available disk space and the frequency of frontend changes
when making the recommendation. Do not implement — report and recommend only.
```
