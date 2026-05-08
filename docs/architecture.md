# Architecture

atcli is intentionally small. Keep the structure boring until the command surface forces more abstraction.

## Package Layout

```text
.
├── main.go
├── cmd/
├── internal/auth/
└── internal/attio/
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
- [../cmd/schema_output.go](../cmd/schema_output.go): table output helpers for schema discovery.
- [../cmd/whoami.go](../cmd/whoami.go): token introspection and optional member display.

Commands should stay thin. If a command needs Attio API behavior, add it under `internal/attio`.

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
- Workspace member lookup used by `whoami`.
- Object, list, and attribute discovery used by schema commands.

Keep endpoint-specific response structs close to the method that uses them until reuse justifies splitting them out.
