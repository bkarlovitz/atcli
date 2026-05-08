# Sprint 5: CSV Import Apply

## Objective
After this sprint, a user can apply a validated CSV import to Attio records with resumable, machine-readable results.

## Tasks
- [x] **Task 5.1**: Add import execution engine
  - Reuse the Sprint 4 dry-run planner to build row payloads before executing writes.
  - Require `--apply` before any record write endpoint is called.
  - Default to upsert/assert mode and support `--mode create`.
  - Return row-level results with row number, mode, object, matching attribute, Attio record ID when returned, status, and error details.
  - Validation: unit tests cover planner reuse, `--apply` gating, create mode, upsert mode, and result identity propagation.

- [x] **Task 5.2**: Implement rate-limit handling for imports
  - Detect Attio rate-limit responses from record write endpoints.
  - Respect retry metadata when available and otherwise use bounded backoff.
  - Keep retries row-scoped so one rate-limited row does not lose previous successful row results.
  - Validation: `httptest` import tests simulate 429 responses, retry success, retry exhaustion, and summary reporting.

- [x] **Task 5.3**: Implement row failure behavior
  - Stop on the first row failure by default after reporting prior successful rows.
  - Support `--continue-on-error` to process remaining rows after validation or write failures.
  - Never print tokens or secret environment values in row errors.
  - Validation: command tests cover default stop behavior, `--continue-on-error`, validation failures before writes, Attio write failures, and sanitized errors.

- [x] **Task 5.4**: Add import apply output modes
  - Support `--output table|jsonl` for apply mode.
  - Table output should include totals for planned, succeeded, failed, skipped, created/updated when known, and elapsed time.
  - JSONL output should emit row result events and a final summary event with machine-readable record IDs.
  - Validation: output tests cover table summaries, JSONL row events, final summary events, and missing optional create/update status.

- [x] **Task 5.5**: Add failed-row export
  - Support `--errors <csv>` to write failed input rows plus error columns.
  - Preserve the original input columns and row numbers.
  - Include enough error context for a later corrected rerun without writing secrets.
  - Validation: tests cover no-error file behavior, one failed row, multiple failed rows, validation failures, write failures, and preserving original CSV data.

- [x] **Task 5.6**: Document CSV apply workflows
  - Update `docs/commands.md` with safe apply examples, `--continue-on-error`, `--errors`, and JSONL examples for agents.
  - Update `docs/attio-api.md` with rate-limit and retry assumptions for imports.
  - Document that upsert/assert is the preferred CSV mode because imports should be rerunnable.
  - Validation: `gofmt -w main.go cmd internal`, `go test ./...`, `go build -o bin/atcli .`, `./bin/atcli records import --help`, and an apply-mode test using mocked Attio endpoints.
