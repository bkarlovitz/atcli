# Sprint 1: Schema Discovery Commands

## Objective
After this sprint, a user can inspect the Attio objects, lists, and attributes needed to plan one-off record creation and CSV imports without writing data.

## Tasks
- [x] **Task 1.1**: Add injectable Attio client transport for tests
  - Allow `internal/attio.Client` to use a custom base URL and HTTP client in tests while preserving `attio.NewClient(token)` production behavior.
  - Keep Bearer auth, JSON accept headers, API error handling, and response decoding behavior centralized.
  - Validation: `go test ./internal/attio` includes tests for auth header, custom base URL, successful JSON decode, and non-2xx `APIError` behavior.

- [x] **Task 1.2**: Add Attio schema API methods
  - Add `internal/attio` methods for listing objects, listing lists, listing object attributes, and listing list attributes.
  - Model the fields needed by commands: API slug or ID, Attio-provided `singular_noun`, Attio-provided `plural_noun`, title/name, parent object, type, required, unique, multiselect, writable/editable flags, and archived status when returned.
  - Handle missing, nullable, or nested response fields without panics; command output should remain stable when optional metadata is absent.
  - Validation: `go test ./internal/attio` includes `httptest` coverage for each endpoint and representative missing optional fields.

- [x] **Task 1.3**: Implement `objects list`
  - Add an `objects` command group and an `objects list` command.
  - Display Attio object identifiers plus Attio-provided singular and plural nouns verbatim.
  - Do not derive singular or plural names from object slugs; standard object arguments remain Attio slugs/IDs, usually plural, such as `people` and `companies`.
  - Validation: command output tests cover table formatting, empty optional nouns, and use of returned nouns without string derivation.

- [x] **Task 1.4**: Implement `objects attributes <object>`
  - Treat `<object>` as an Attio object slug or ID, not a singularized noun.
  - Show attribute slug/ID, title, type, writable/editable status, required status, unique status, and multiselect status.
  - Hide archived attributes by default and include them with `--all`.
  - Validation: command output tests cover default archived filtering, `--all`, missing optional metadata, and the help text for `<object>` semantics.

- [x] **Task 1.5**: Implement `lists list`
  - Add a `lists` command group and a `lists list` command.
  - Display list identifiers, names, and parent object information when available.
  - Keep list arguments as Attio list slugs or IDs; do not infer list names.
  - Validation: command output tests cover parent object display and missing parent metadata.

- [x] **Task 1.6**: Implement `lists attributes <list>`
  - Treat `<list>` as an Attio list slug or ID.
  - Show list-entry attribute slug/ID, title, type, writable/editable status, required status, unique status, and multiselect status.
  - Hide archived attributes by default and include them with `--all`.
  - Validation: command output tests cover list-entry attribute display, archived filtering, and help output.

- [x] **Task 1.7**: Document schema discovery workflows
  - Update `docs/commands.md` with examples for discovering object slugs, object attributes, lists, and list-entry attributes.
  - Update `docs/attio-api.md` with the new endpoint links, required scopes, and response fields the CLI depends on.
  - Include a short planning workflow: use `objects list`, `objects attributes people`, `objects attributes companies`, and `lists attributes <list>` to identify CSV headers and write targets.
  - Validation: `go test ./...`, `go build -o bin/atcli .`, `./bin/atcli --help`, `./bin/atcli objects --help`, `./bin/atcli objects list --help`, `./bin/atcli objects attributes --help`, `./bin/atcli lists --help`, `./bin/atcli lists list --help`, and `./bin/atcli lists attributes --help`.
