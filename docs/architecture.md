# Architecture

atcli is intentionally small. Keep the structure boring until the command surface forces more abstraction.

## Package Layout

```text
.
├── main.go
├── cmd/
├── internal/auth/
├── internal/attio/
└── internal/importplan/
```

## Entry Point

[../main.go](../main.go) calls `cmd.Execute()` and handles the final process exit.

## Commands

[../cmd](../cmd) owns Cobra command definitions and user-facing output.

Current files:

- [../cmd/root.go](../cmd/root.go): root command and Cobra error/usage behavior.
- [../cmd/attio_client.go](../cmd/attio_client.go): command-side authenticated Attio client loading.
- [../cmd/auth.go](../cmd/auth.go): interactive or stdin token authentication.
- [../cmd/objects.go](../cmd/objects.go): object schema discovery commands.
- [../cmd/lists.go](../cmd/lists.go): list schema discovery commands.
- [../cmd/match_policy.go](../cmd/match_policy.go): safe default and explicit match policy for record upserts.
- [../cmd/record_metadata.go](../cmd/record_metadata.go): metadata lookup and local validation for record creates and upserts.
- [../cmd/records.go](../cmd/records.go): one-off record create and upsert commands.
- [../cmd/records_import.go](../cmd/records_import.go): CSV import command wiring and plan/apply mode selection.
- [../cmd/records_import_apply.go](../cmd/records_import_apply.go): CSV import apply execution, row retry behavior, summaries, and table/JSONL output.
- [../cmd/records_import_errors.go](../cmd/records_import_errors.go): failed-row CSV export.
- [../cmd/schema_output.go](../cmd/schema_output.go): table output helpers for schema discovery.
- [../cmd/value_flags.go](../cmd/value_flags.go): `--set` and `--set-json` parsing for record writes.
- [../cmd/whoami.go](../cmd/whoami.go): token introspection and optional member display.
- [../cmd/write_errors.go](../cmd/write_errors.go): command-facing write error classification.
- [../cmd/write_output.go](../cmd/write_output.go): table and JSON output helpers for one-off writes.

Commands should stay thin. If a command needs Attio API behavior, add it under `internal/attio`.

## CSV Import Planning and Apply

[../internal/importplan](../internal/importplan) owns CSV loading, mapping, conversion, and row planning behavior that is not Attio-specific API plumbing.

Responsibilities:

- Load CSV files, validate headers, preserve row numbers, and reject malformed input.
- Build CSV-column to Attio-attribute mapping plans from headers, `--map`, `--ignore`, and static values.
- Prepare first-pass Attio values from CSV cells using optional object attribute metadata.
- Build row-by-row plans with validation status, skipped empty cells, warnings, and values.

The command layer owns Cobra flags, token/client loading, metadata fallback policy, apply execution, row-scoped rate-limit retries, failed-row CSV export, and table/JSONL output.

## Auth

[../internal/auth](../internal/auth) owns token storage and OAuth token introspection.

Responsibilities:

- Load `ATTIO_ACCESS_TOKEN`.
- Store/load token via OS credential store.
- Classify missing auth and credential-store failures.
- Call Attio token introspection.

## Attio API Client

[../internal/attio](../internal/attio) owns calls to `https://api.attio.com/v2`.

Current behavior:

- Shared client with Bearer auth header.
- API error type with HTTP status.
- Retry metadata from `Retry-After` when Attio returns rate limits.
- Workspace member lookup used by `whoami`.
- Object, list, and attribute discovery used by schema commands.
- Record create support used by `records create`.
- Record assert support used by `records upsert`.
- Object and attribute discovery used by `records import` validation.

Keep endpoint-specific response structs close to the method that uses them until reuse justifies splitting them out.
