# Sprint 3: One-Off Record Upsert

## Objective
After this sprint, a user can safely create or update a single Attio record using a unique matching attribute.

## Tasks
- [x] **Task 3.1**: Add record assert API support
  - Add an `internal/attio` method for Attio's assert record endpoint.
  - Accept an object slug or ID, a matching attribute slug or ID, and record values.
  - Return enough identity data for downstream workflows, including object identifier, record ID, and whether Attio reports create/update status when available.
  - Validation: `go test ./internal/attio` includes `httptest` coverage for request method/path/body, matching attribute placement, success response decode, validation errors, permission errors, and rate-limit status.

- [x] **Task 3.2**: Implement matching attribute policy
  - Require `--match <attribute>` unless a safe default is defined for the object.
  - Use safe defaults only for standard Attio object slugs: `companies` maps to `domains`, `people` maps to `email_addresses`, `users` maps to `primary_email_address`, and `workspaces` maps to `workspace_id`.
  - Require explicit `--match` for `deals` and custom objects.
  - Do not infer defaults from singularized or pluralized object names.
  - Validation: unit tests cover every default, explicit overrides, `deals` requiring `--match`, custom objects requiring `--match`, and unknown object slugs requiring `--match`.

- [x] **Task 3.3**: Add metadata-backed match validation
  - When attribute metadata is available, verify that the match attribute exists and is unique.
  - Verify that the match attribute has a value in the provided record payload.
  - If metadata cannot be fetched because of missing scope, continue with explicit `--match` and explain that uniqueness validation was skipped.
  - Validation: command tests cover unique match success, non-unique match failure, missing match value failure, metadata permission fallback, and missing metadata fields.

- [x] **Task 3.4**: Implement `records upsert <object>`
  - Add `records upsert <object>` using the assert record endpoint.
  - Support the same `--set`, `--set-json`, `--dry-run`, and `--output table|json` flags as `records create`.
  - In dry-run mode, fetch metadata when possible, validate the payload, print the exact assert payload, and avoid all write endpoints.
  - Validation: command tests cover successful upsert, default match use, explicit match use, dry-run behavior, JSON output, table output, and command help.

- [x] **Task 3.5**: Improve write error classification
  - Normalize common write failures into actionable messages: missing auth, missing record write scope, validation failure, non-unique matching attribute, rate limit, and network timeout.
  - Keep lower-level Attio response details available enough for debugging without printing secrets.
  - Validation: unit tests cover representative `APIError` statuses and response bodies for the command-facing error messages.

- [x] **Task 3.6**: Document one-off upsert
  - Update `docs/commands.md` with upsert examples for `companies` and `people`, including explicit `--match`.
  - Update `docs/attio-api.md` with the assert record endpoint, matching attribute requirements, and default-match policy.
  - Document that upsert is the preferred mental model for rerunnable imports.
  - Validation: `gofmt -w main.go cmd internal`, `go test ./...`, `go build -o bin/atcli .`, `./bin/atcli records --help`, and `./bin/atcli records upsert --help`.
