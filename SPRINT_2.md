# Sprint 2: One-Off Record Create

## Objective
After this sprint, a user can create a single Attio record from shell flags and preview the exact write payload before sending it.

## Tasks
- [x] **Task 2.1**: Add record create API support
  - Add an `internal/attio` method for Attio's create record endpoint.
  - Return enough record identity data for command output, including object identifier and record ID when available.
  - Preserve useful API errors for permission, validation, and rate-limit responses without exposing tokens.
  - Validation: `go test ./internal/attio` includes `httptest` coverage for request method/path/body, successful response decode, validation errors, permission errors, and rate-limit status.

- [x] **Task 2.2**: Add value flag parsing helpers
  - Parse repeated `--set attr=value` flags as string values.
  - Parse repeated `--set-json attr=json` flags for arrays, numbers, objects, booleans, and null.
  - Reject malformed flags, empty attribute names, duplicate attribute names across both flag types, and malformed JSON.
  - Validation: focused unit tests cover valid strings, valid JSON values, duplicates, empty names, missing `=`, and malformed JSON.

- [ ] **Task 2.3**: Define one-off write output conventions
  - Add reusable output helpers for table and JSON output used by one-off record commands.
  - For dry runs, output the exact JSON payload that would be sent and clearly mark that no write endpoint was called.
  - For successful writes, include record ID and object metadata when available.
  - Validation: unit tests cover table output, JSON output, dry-run output, and missing optional metadata.

- [ ] **Task 2.4**: Implement `records create <object>`
  - Add a `records` command group and `records create <object>`.
  - Treat `<object>` as an Attio object slug or ID, usually plural for standard objects such as `people` and `companies`.
  - Use Attio-provided `singular_noun` and `plural_noun` for display when metadata is available; never singularize or pluralize the object argument.
  - Support `--set`, `--set-json`, `--dry-run`, and `--output table|json`.
  - Validation: command tests cover successful create, dry-run avoiding write endpoints, JSON output, table output, missing auth classification, and command help.

- [ ] **Task 2.5**: Add metadata-aware validation for create when available
  - Fetch object attributes before writes when possible.
  - Validate unknown attributes, required attributes, and writable/editable flags when metadata is available.
  - If metadata cannot be fetched because of missing scope, continue with explicit user-provided attributes and explain that local validation and noun display were skipped.
  - Validation: command tests cover metadata success, metadata permission failure fallback, required attribute errors, unknown attribute errors, and non-writable attribute errors.

- [ ] **Task 2.6**: Document one-off create
  - Update `docs/commands.md` with examples for creating `companies` and `people` records.
  - Update `docs/attio-api.md` with the create record endpoint, payload shape, scope notes, and dry-run behavior.
  - Document that `<object>` means Attio slug/ID and that display nouns come only from Attio metadata.
  - Validation: `gofmt -w main.go cmd internal`, `go test ./...`, `go build -o bin/atcli .`, `./bin/atcli records --help`, and `./bin/atcli records create --help`.
