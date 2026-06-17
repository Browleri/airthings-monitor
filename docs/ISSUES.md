# Design Issues

This file records design issues found during repo inspection. It is not a list
of urgent defects; it is a backlog for maintainability and operational risk.

## Database Types Depend On Scheduler Types

The README describes narrow package boundaries: `internal/db` should own
migrations and storage code, while `internal/scheduler` should own polling
cadence and sampling decisions. Today `internal/db` accepts
`scheduler.SampledReading`, so the storage layer depends on scheduler types and
transitively on the Airthings client package. That makes package boundaries less
clean and expands the compile/test surface for database-only changes.

Consider moving the storage write DTO into `internal/db` or a smaller shared
type so `internal/db` does not need to import `internal/scheduler`.

## Migration SQL Exists In Two Places

`docs/DATABASE.md` explains the schema, but it does not explain why migration
SQL is duplicated. The initial schema exists in both `migrations/001_initial.sql`
and the `initialMigration` string in `internal/db/db.go`. That creates drift
risk: future schema edits could update one copy and leave the other stale.

Consider making the runtime migration read from embedded migration files using
Go `embed`, or remove the unused copy if runtime-embedded SQL remains the
chosen approach.

## Frontend Chart Behavior Has No Automated Coverage

The AGENTS instructions call out tests for HTTP handlers, scheduler decisions,
and decoders when behavior changes, but there is no equivalent frontend test
path. The chart now contains timestamp spacing, threshold rendering, and hover
selection logic; regressions there would currently be caught only by manual
inspection or TypeScript compilation.

Consider adding a lightweight component test or extracting the chart scale and
nearest-point calculations into pure functions with tests.

## Frontend Build Requires Network For Each Clean Build

`docs/DEVELOPMENT.md` and `docs/INSTALL.md` document that Node.js/npm are build
dependencies, and the Makefile now uses `npm ci` for reproducible installs.
That is correct, but it means a clean build on the Pi still needs registry
access unless `web/node_modules` or an npm cache is already populated.

For a Pi that is kept intentionally quiet and conservative, consider documenting
an offline artifact workflow or adding a build mode that deploys a previously
built `web/dist` from a workstation.
