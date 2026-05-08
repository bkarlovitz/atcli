# Sprint 6: List Entry Workflows

## Objective
After this sprint, a user can add records to Attio lists one-off or as part of a CSV import while keeping record writes and list-entry writes clearly reported.

## Tasks
- [x] **Task 6.1**: Add list entry API support
  - Add `internal/attio` methods for creating a list entry and asserting a list entry by parent record.
  - Accept list slug/ID, parent object slug/ID, parent record ID, and list-entry values.
  - Return entry identity, list identity, parent record identity, and create/update status when available.
  - Validation: `go test ./internal/attio` includes `httptest` coverage for request method/path/body, parent record references, success decode, permission errors, validation errors, duplicate/multiple-match errors, and rate-limit status.

- [x] **Task 6.2**: Implement `entries add <list>`
  - Add an `entries` command group and `entries add <list>`.
  - Require `--parent-object <object>` and `--parent-record-id <id>`.
  - Support `--set`, `--set-json`, `--dry-run`, and `--output table|json`.
  - Treat `<list>` and `<object>` as Attio slugs or IDs; use Attio-provided nouns for display when metadata is available.
  - Validation: command tests cover successful add, dry-run payload preview, missing parent flags, metadata permission fallback, JSON output, table output, and command help.

- [x] **Task 6.3**: Implement `entries upsert <list>`
  - Add `entries upsert <list>` using Attio's list-entry assert behavior by parent record.
  - Require `--parent-object <object>` and `--parent-record-id <id>`.
  - Support the same value and output flags as `entries add`.
  - Validate list-entry attributes and parent object compatibility when metadata is available.
  - Validation: command tests cover successful upsert, parent object mismatch, missing metadata fallback, dry-run behavior, JSON output, table output, and command help.

- [x] **Task 6.4**: Add list integration to CSV imports
  - Extend `records import <object> <csv>` with `--list <list>`, `--list-mode create|upsert`, repeated `--entry-map csv_column=list_attribute`, and repeated `--entry-set attr=value`.
  - Use record IDs returned from record create/upsert results as parent record IDs for list-entry writes.
  - Validate that the list parent object matches the imported object when metadata is available.
  - Validation: import tests cover record-plus-entry success, parent object mismatch, missing record ID response, list metadata fallback, and entry payload construction.

- [x] **Task 6.5**: Define combined import failure behavior
  - If a record write fails, skip the corresponding list-entry write and report the row as a record failure.
  - If a list-entry write fails after a record write succeeds, keep the record result and report the row as an entry failure.
  - Include both record and entry status in JSONL row events and failed-row CSV exports.
  - Surface Attio multiple-match, duplicate entry, validation, permission, and rate-limit failures clearly.
  - Validation: tests cover record failure skip, entry failure after record success, `--continue-on-error`, JSONL status fields, and errors CSV fields.

- [x] **Task 6.6**: Document list entry workflows
  - Update `docs/commands.md` with one-off entry examples and CSV import examples that add imported records to a list.
  - Update `docs/attio-api.md` with list entry create/assert endpoints, parent record semantics, and failure behavior.
  - Document the distinction between records and list entries: list entries point at records and may have their own values.
  - Validation: `gofmt -w main.go cmd internal`, `go test ./...`, `go build -o bin/atcli .`, `./bin/atcli entries --help`, `./bin/atcli entries add --help`, `./bin/atcli entries upsert --help`, and `./bin/atcli records import --help`.
