# Agent Guide

Read this first when working on atcli.

## Project Purpose

atcli is a small Go CLI for Attio, built incrementally around commands the user needs for their own work.

Current command surface:

- `atcli auth`
- `atcli whoami`
- `atcli objects list`
- `atcli objects attributes`
- `atcli lists list`
- `atcli lists attributes`
- `atcli records create`
- `atcli records upsert`
- `atcli records import`

## Fast Context

- Language: Go.
- CLI framework: Cobra.
- Entry point: [main.go](main.go).
- Commands live in [cmd](cmd).
- Attio REST helpers live in [internal/attio](internal/attio).
- Auth storage and token introspection live in [internal/auth](internal/auth).
- CSV import loading, mapping, conversion, and planning live in [internal/importplan](internal/importplan).
- Local binaries should be built into `bin/`; `bin/` is gitignored.

## Verification Commands

Run these before handing off code changes:

```bash
gofmt -w main.go cmd internal
go test ./...
go build -o bin/atcli .
```

For command wiring:

```bash
./bin/atcli --help
./bin/atcli auth --help
./bin/atcli whoami --help
./bin/atcli objects --help
./bin/atcli objects list --help
./bin/atcli objects attributes --help
./bin/atcli lists --help
./bin/atcli lists list --help
./bin/atcli lists attributes --help
./bin/atcli records --help
./bin/atcli records create --help
./bin/atcli records upsert --help
./bin/atcli records import --help
```

For live Attio testing, prefer:

```bash
ATTIO_ACCESS_TOKEN='token' ./bin/atcli whoami
```

For live schema discovery testing, prefer read-only commands:

```bash
ATTIO_ACCESS_TOKEN='token' ./bin/atcli objects list
ATTIO_ACCESS_TOKEN='token' ./bin/atcli objects attributes people
ATTIO_ACCESS_TOKEN='token' ./bin/atcli lists list
```

For record write testing, prefer dry runs unless intentionally creating workspace data:

```bash
ATTIO_ACCESS_TOKEN='token' ./bin/atcli records create companies --set name='Example Co' --set-json 'domains=["example.com"]' --dry-run
ATTIO_ACCESS_TOKEN='token' ./bin/atcli records upsert companies --match domains --set name='Example Co' --set-json 'domains=["example.com"]' --dry-run
ATTIO_ACCESS_TOKEN='token' ./bin/atcli records import companies ./companies.csv --match domains --output jsonl
```

Do not print real tokens in logs, docs, tests, or final responses.

## Auth Model

Attio API calls use Bearer tokens. atcli supports:

1. `ATTIO_ACCESS_TOKEN` environment variable.
2. Token stored in the OS credential store by `atcli auth`.

The env var wins so automation and machines without a working keyring can still use the CLI.

Linux keyring failures are expected in some terminal, WSL, or headless setups. Keep error messages actionable and do not expose low-level keyring internals unless debugging explicitly requires it.

## Implementation Conventions

- Keep commands thin. Put API-specific code under `internal/attio` and auth-specific code under `internal/auth`.
- Return errors from commands; root Cobra config suppresses duplicate usage/error output.
- Use `context.WithTimeout` for network calls from commands.
- Make new commands degrade gracefully when scopes are missing.
- Add focused tests for formatting/output branches and error classification.

## Adding A Command

1. Add `cmd/<name>.go`.
2. Reuse `auth.LoadToken()` for authenticated commands.
3. Add or extend an `internal/attio` client method for API calls.
4. Add tests where output or error behavior matters.
5. Update [docs/commands.md](docs/commands.md) and [docs/attio-api.md](docs/attio-api.md) if new endpoints are used.
