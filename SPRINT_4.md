# Sprint 4: CSV Import Dry-Run Planner

## Objective
After this sprint, a user can point the CLI at an agent-produced CSV and see a validated, row-by-row import plan without writing records.

## Tasks
- [x] **Task 4.1**: Add CSV loading and header validation
  - Read CSV files with headers.
  - Reject empty headers, duplicate headers, unreadable files, inconsistent row lengths, and empty files.
  - Preserve row numbers for error reporting.
  - Validation: unit tests use fixture CSVs for valid input, duplicate headers, empty headers, uneven rows, empty files, and unreadable paths.

- [x] **Task 4.2**: Add CSV-to-Attio mapping rules
  - Map CSV headers to Attio attribute slugs by default.
  - Support repeated `--map csv_column=attio_attribute`.
  - Support repeated `--ignore csv_column`.
  - Support static values with the existing `--set` and `--set-json` helpers.
  - Reject mappings for missing CSV columns, duplicate target attributes, ignored columns that are also mapped, and static values that conflict with mapped attributes.
  - Validation: unit tests cover default mapping, explicit mapping, ignored columns, static values, and every conflict case.

- [x] **Task 4.3**: Add first-pass attribute-aware CSV value preparation
  - Empty cells omit values by default and do not clear existing Attio values.
  - Support string passthrough for text-like attributes, names, email addresses, domains, phone numbers, select/status option titles, and relationship-like values when no richer conversion is available.
  - Support number parsing, checkbox parsing, ISO date parsing, and RFC3339 timestamp parsing.
  - Support multi-value attributes with `--multi-sep`, preserving empty-cell omit semantics.
  - Provide a raw JSON escape hatch for complex attributes through static `--set-json`; defer explicit clearing to a future command or flag.
  - Validation: unit tests cover each supported type, invalid values, multi-value splitting, empty cells, and JSON static values.

- [x] **Task 4.4**: Implement `records import <object> <csv>` dry-run mode
  - Add `records import <object> <csv>` with dry-run as the default behavior.
  - Treat `<object>` as an Attio object slug or ID, usually plural for standard objects.
  - Use the Sprint 3 matching attribute policy, defaulting to upsert planning unless `--mode create` is passed.
  - Fetch object attributes when possible, validate unknown columns, required values, writable/editable status, match attribute presence, and match uniqueness when metadata is available.
  - Avoid all record write endpoints in dry-run mode.
  - Validation: command tests cover planned upsert, planned create, unknown columns, missing required values, non-writable attributes, metadata permission fallback, and no write endpoint calls.

- [x] **Task 4.5**: Add dry-run output for humans and agents
  - Support `--output table|jsonl`.
  - Table output should summarize row count, mode, object, matching attribute, skipped empty cells, warnings, and sample planned rows.
  - JSONL output should emit one machine-readable event per row with row number, mode, object, matching attribute, values, warnings, and validation status.
  - Include record identifiers only when known from input or metadata; do not invent IDs.
  - Validation: tests cover table summaries, JSONL shape, warnings, and stable output with missing optional metadata.

- [x] **Task 4.6**: Document CSV dry-run planning
  - Update `docs/commands.md` with CSV dry-run examples, mapping examples, ignore examples, and `--output jsonl`.
  - Update `docs/attio-api.md` with CSV planning assumptions and supported first-pass attribute conversions.
  - Document empty-cell semantics, match defaults, and recommended import order for linked objects, such as companies before people.
  - Validation: `gofmt -w main.go cmd internal`, `go test ./...`, `go build -o bin/atcli .`, `./bin/atcli records import --help`, and a local dry-run against fixture CSVs.
