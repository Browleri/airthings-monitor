# AI Agent Default Prompt

Paste this context block at the start of any agent session before describing
the task. It gives the agent the minimum stable facts needed to work safely
in this repository without re-reading every file from scratch.

---

## Context Block

```
You are working in the airthings-monitor repository. This is a lightweight,
self-hosted indoor air quality monitoring service for a Raspberry Pi 3B+ that
also runs Pi-hole. Read AGENTS.md and docs/ISSUES.md before making any
decisions. The notes below are a fast summary of the constraints you must
respect.

## Hard Constraints

- Do not add Docker, InfluxDB, Home Assistant, Grafana, or any service that
  would increase Pi-hole's memory or CPU footprint.
- Do not write runtime data to the SD card. The database and any runtime
  artifacts belong at /mnt/pihole-usb/airthings/ on the Pi.
- Keep Go dependencies minimal. Prefer standard library. Justify each addition.
- Keep npm dependencies minimal. Do not add a charting library — the SVG chart
  in web/src/main.tsx is intentional and covers the current requirements.
- Do not add authentication or TLS to the HTTP server; it is local-network only
  and authentication is out of scope for this version.
- SQLite must use WAL mode and synchronous=NORMAL. Do not change these settings.
- Do not introduce fixed sleep loops or retry patterns that ignore context
  cancellation.

## Package Boundaries

Imports must follow this direction (→ means "may import"):
  cmd/airthings-server → internal/* (all packages)
  internal/httpapi     → internal/db, internal/config, internal/scheduler
  internal/scheduler   → internal/airthings, internal/db, internal/config
  internal/db          → internal/config
  internal/airthings   → internal/config
  internal/config      → standard library only

If your task requires a new dependency between packages that violates these
rules, stop and describe the proposed change before implementing it.

## Coding Style

- No comments unless the WHY is non-obvious (hidden constraint, workaround for
  a specific bug, subtle invariant). Never describe what the code does.
- No docstrings or multi-line comment blocks.
- Return errors to callers — do not log-and-continue inside library packages.
  Only cmd/airthings-server and internal/scheduler should log errors.
- Use slog (log/slog) with structured key-value pairs. No fmt.Println.
- All long-running operations must accept a context.Context and respect
  cancellation. Check ctx.Err() before blocking calls.

## Testing

- Tests live next to the package they cover (*_test.go in the same directory).
- Add tests when you change behaviour in: decoder, sampling, retry, HTTP
  handlers.
- Do not mock the SQLite database in tests; use an in-memory database
  (path ":memory:") to exercise real SQL.
- Do not add a test framework dependency; use standard library testing only.

## Frontend

- The frontend is a single React + TypeScript file at web/src/main.tsx.
- Build output goes to web/dist/ and is embedded in the Go binary via embed.FS.
- Do not add a component library or CSS framework.
- Keep responsive breakpoints and accessibility attributes consistent with what
  is already in web/src/styles.css.
- Run `npm run build` in web/ after any frontend change and verify the output
  compiles without TypeScript errors before reporting the task complete.

## Operational Targets

- Memory use: keep the Go binary under ~30 MB RSS on Pi 3B+.
- CPU: the polling goroutine should be idle between reads; no tight loops.
- Startup time: service should be ready to accept HTTP within 5 seconds on Pi.
- Log volume: normal operation should produce at most one log line per poll
  cycle (about one line per minute at default intervals).

## Known Issues (docs/ISSUES.md)

These are deferred — do not fix them unless the task explicitly asks for it,
but do not make them worse:
1. internal/db imports internal/scheduler (SampledReading coupling).
2. Migration SQL exists in both migrations/001_initial.sql and db.go.
3. Frontend chart logic has no automated test coverage.
4. Clean builds on the Pi require npm registry access.

## What to Do Before Starting

1. Read the relevant source file(s) for the task area.
2. Run `make test` mentally — ensure your change does not break existing tests.
3. If the task touches the database schema, read docs/DATABASE.md.
4. If the task touches BLE, read AGENTS.md §Operational Notes.
5. If the task touches deployment, read docs/INSTALL.md and
   deploy/systemd/airthings.service.

State your plan in one short paragraph before writing any code. If the plan
requires violating a constraint above, stop and ask.
```

---

## Task Suffix Templates

Append one of these after the context block depending on the kind of work.

### Bug Fix

```
## Task: Fix — [one-line description]

Symptom: [what the user sees or what the logs show]
Reproduction: [steps or log excerpt]
Suspected area: [package or file]

Constraints: fix only what is broken. Do not refactor surrounding code, do not
change interfaces, do not add logging beyond what is needed to confirm the fix.
```

### New Feature

```
## Task: Feature — [one-line description]

Goal: [what the feature does and why it is needed]
Scope: [what packages or files are expected to change]
Out of scope: [what should not change]

Before coding: confirm the feature does not require a new Go dependency or a
new npm package. If it does, name the dependency and justify it.
```

### Refactor

```
## Task: Refactor — [one-line description]

Motivation: [why this refactor is needed — reference docs/ISSUES.md if applicable]
Expected outcome: [what improves — API, maintainability, test surface]
Behaviour change: none expected

Constraints: all existing tests must pass after the refactor. If a test must
change, explain why the old test was testing implementation rather than
behaviour.
```

### Maintenance / Dependency Update

```
## Task: Maintenance — [dependency or area]

What changed externally: [new version, CVE, deprecation notice]
Expected impact: [which files change, what behaviour might differ]

Before updating: confirm the new version's changelog for breaking changes.
After updating: run `make test`. If tests fail, diagnose before proceeding.
```

### Investigation / Report

```
## Task: Investigate — [question or concern]

Context: [what prompted the investigation]
Output format: written report only — do not modify any files unless explicitly
asked. End with a recommendation and the trade-offs for each option.
```
