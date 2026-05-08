# Attio API Notes

This file records the Attio API assumptions the code currently depends on.

Primary docs:

- REST API overview: https://docs.attio.com/rest-api/overview
- Authentication guide: https://docs.attio.com/rest-api/guides/authentication
- List objects: https://docs.attio.com/rest-api/endpoint-reference/objects/list-objects
- List all lists: https://docs.attio.com/rest-api/endpoint-reference/lists/list-all-lists
- List attributes: https://docs.attio.com/rest-api/endpoint-reference/attributes/list-attributes
- Create a record: https://docs.attio.com/rest-api/endpoint-reference/records/create-a-record
- Assert a record: https://docs.attio.com/rest-api/endpoint-reference/records/assert-a-record
- Create a list entry: https://docs.attio.com/rest-api/endpoint-reference/entries/create-an-entry-add-record-to-list
- Assert a list entry by parent: https://docs.attio.com/rest-api/endpoint-reference/entries/assert-a-list-entry-by-parent
- Rate limiting: https://docs.attio.com/rest-api/guides/rate-limiting
- OAuth introspection: https://docs.attio.com/docs/oauth/introspect
- Get workspace member: https://docs.attio.com/rest-api/endpoint-reference/workspace-members/get-a-workspace-member

## Authentication

Attio REST API calls use Bearer auth:

```http
Authorization: Bearer <access_token>
```

For this project, the first auth path is a manually provided API key or access token. This fits personal CLI usage and avoids full OAuth app setup until the CLI needs multi-workspace installation.

## Token Introspection

Endpoint:

```http
POST https://app.attio.com/oauth/introspect
```

Used by:

- `atcli auth` to validate a token before storing it.
- `atcli whoami` to identify the authenticated workspace.

Fields currently modeled:

- `active`
- `scope`
- `token_type`
- `workspace_id`
- `workspace_name`
- `workspace_slug`
- `authorized_by_workspace_member_id`

## Workspace Member Lookup

Endpoint:

```http
GET https://api.attio.com/v2/workspace_members/{workspace_member_id}
```

Used by:

- `atcli whoami` when introspection provides `authorized_by_workspace_member_id`.

Fields currently modeled:

- `id.workspace_member_id`
- `first_name`
- `last_name`
- `email_address`
- `access_level`

This lookup may require `user_management:read`. If the token lacks permission, `whoami` should still print workspace/token details and report that member details are unavailable.

## Objects

Endpoint:

```http
GET https://api.attio.com/v2/objects
```

Used by:

- `atcli objects list` to discover object slugs and display nouns before record writes or CSV import planning.

Required scope:

- `object_configuration:read`

Fields currently modeled:

- `id.workspace_id`
- `id.object_id`
- `api_slug`
- `singular_noun`
- `plural_noun`
- `created_at`

Important naming rule:

- Command arguments use Attio object slugs or IDs. Standard object slugs are usually plural, such as `people` and `companies`.
- Display should use Attio's returned `singular_noun` and `plural_noun` fields when available.
- The CLI should not derive singular or plural names from object slugs.

## Lists

Endpoint:

```http
GET https://api.attio.com/v2/lists
```

Used by:

- `atcli lists list` to discover list slugs, IDs, names, and parent object compatibility.

Required scope:

- `list_configuration:read`

Fields currently modeled:

- `id.workspace_id`
- `id.list_id`
- `api_slug`
- `name`
- `parent_object`
- `workspace_access`
- `created_at`

## Attributes

Endpoint:

```http
GET https://api.attio.com/v2/{target}/{identifier}/attributes
```

Path parameters:

- `target`: `objects` or `lists`
- `identifier`: object or list UUID/API slug

Query parameters used:

- `show_archived=true` when a command receives `--all`

Used by:

- `atcli objects attributes <object>` with target `objects`
- `atcli lists attributes <list>` with target `lists`

Required scopes:

- `object_configuration:read` when target is `objects`
- `list_configuration:read` when target is `lists`

Fields currently modeled:

- `id.workspace_id`
- `id.object_id`
- `id.list_id`
- `id.attribute_id`
- `title`
- `description`
- `api_slug`
- `type`
- `is_system_attribute`
- `is_writable`
- `is_editable` when returned
- `is_required`
- `is_unique`
- `is_multiselect`
- `is_archived`

Archived behavior:

- Attribute commands hide archived attributes by default.
- Attribute commands request and display archived attributes with `--all`.

## Record Create

Endpoint:

```http
POST https://api.attio.com/v2/objects/{object}/records
```

Used by:

- `atcli records create <object>` to create one record from shell flags.

Path parameters:

- `object`: Attio object UUID or API slug. atcli passes the user's `<object>` argument through unchanged. Standard slugs are usually plural, such as `people` and `companies`.

Required scopes in Attio's docs:

- `record_permission:read-write`
- `object_configuration:read`

Payload shape:

```json
{
  "data": {
    "values": {
      "attribute_api_slug_or_id": "value"
    }
  }
}
```

Command flag mapping:

- `--set attr=value` adds a string value under `data.values[attr]`.
- `--set-json attr=json` adds a decoded JSON value under `data.values[attr]`.
- Duplicate attributes across both flag types are rejected locally.

Fields currently modeled from successful responses:

- `data.id.workspace_id`
- `data.id.object_id`
- `data.id.record_id`
- `data.created_at`
- `data.web_url`
- `data.values`

Metadata behavior before writes:

- atcli tries to call `GET /objects` and `GET /objects/{object}/attributes` before non-dry-run creates.
- When metadata is available, output uses Attio's returned `singular_noun` and `plural_noun`, and create input is checked against returned attribute `api_slug`, `is_writable`, `is_editable`, and `is_required`.
- If metadata calls fail with a permission error, atcli warns that local validation and noun display were skipped, then still attempts the create request with the explicit user-provided values.
- Non-permission metadata errors stop the command before the write.

Dry-run behavior:

- `--dry-run` prints the exact JSON payload shape atcli would send to the create endpoint.
- It marks `write_endpoint_called` as false in JSON output or prints `DRY RUN: no write endpoint called` in table output.
- It does not load credentials, fetch metadata, or call Attio endpoints.

Error behavior:

- Command-facing errors normalize common write failures into actionable messages for missing auth, missing `record_permission:read-write`, validation failures, rate limits, and network timeouts.
- API error status and response bodies are preserved for validation, permission, and rate-limit responses.
- The active bearer token is redacted from preserved API error bodies if an upstream response echoes it.

## Record Assert

Endpoint:

```http
PUT https://api.attio.com/v2/objects/{object}/records?matching_attribute={attribute}
```

Used by:

- `atcli records upsert <object>` to create or update one record using a unique matching attribute.

Path parameters:

- `object`: Attio object UUID or API slug. atcli passes the user's `<object>` argument through unchanged.

Query parameters:

- `matching_attribute`: Attio attribute UUID or API slug. It must identify a unique attribute on the object.

Required scopes in Attio's docs:

- `record_permission:read-write`
- `object_configuration:read`

Payload shape:

```json
{
  "data": {
    "values": {
      "attribute_api_slug_or_id": "value"
    }
  }
}
```

Command flag mapping:

- `--match attr` sets the `matching_attribute` query parameter.
- `--set attr=value` adds a string value under `data.values[attr]`.
- `--set-json attr=json` adds a decoded JSON value under `data.values[attr]`.
- Duplicate attributes across both value flag types are rejected locally.

Fields currently modeled from successful responses:

- `data.id.workspace_id`
- `data.id.object_id`
- `data.id.record_id`
- `data.created_at`
- `data.web_url`
- `data.values`
- `data.status`, `data.outcome`, or `data.operation` when Attio reports create/update outcome
- `data.created` when Attio reports a boolean create/update marker

Matching attribute policy:

- `--match` is required unless the object is one of the exact standard Attio slugs with a safe default.
- Safe defaults are `companies` -> `domains`, `people` -> `email_addresses`, `users` -> `primary_email_address`, and `workspaces` -> `workspace_id`.
- `deals`, custom objects, object IDs, unknown slugs, and singular/plural variants require explicit `--match`.
- atcli does not infer defaults from singularized or pluralized object names.

Metadata behavior before upserts:

- atcli tries to call `GET /objects` and `GET /objects/{object}/attributes` before upserts, including dry runs when credentials are available.
- When metadata is available, output uses Attio's returned nouns, value attributes are checked against returned `api_slug` or `id.attribute_id`, and the match attribute must exist, be unique, and have a non-null payload value.
- If metadata calls fail with a permission error, atcli continues only when `--match` was explicit, warns that local validation and match uniqueness validation were skipped, then attempts the assert request.
- If metadata calls fail with a permission error while the match attribute came from a safe default, atcli stops and asks for explicit `--match`.
- Non-permission metadata errors stop the command before the write.

Dry-run behavior:

- `--dry-run` prints the exact JSON payload body atcli would send to the assert endpoint and reports the match attribute separately.
- It marks `write_endpoint_called` as false in JSON output or prints `DRY RUN: no write endpoint called` in table output.
- It fetches metadata when credentials/scopes allow it and never calls the assert write endpoint.

Error behavior:

- Command-facing errors normalize missing auth, missing `record_permission:read-write`, validation failures, non-unique matching attributes, rate limits, and network timeouts.
- API error status and response bodies remain wrapped in the returned error for debugging.
- The active bearer token is redacted from preserved API error bodies if an upstream response echoes it.

## List Entry Create

Endpoint:

```http
POST https://api.attio.com/v2/lists/{list}/entries
```

Used by:

- `atcli entries add <list>` to add one existing record to a list.
- `atcli records import --list <list> --list-mode create` after each successful record write.

Path parameters:

- `list`: Attio list UUID or API slug. atcli passes the user's list argument through unchanged.

Required scopes in Attio's docs:

- `list_entry:read-write`
- `list_configuration:read`

Payload shape:

```json
{
  "data": {
    "parent_record_id": "record-id",
    "parent_object": "people",
    "entry_values": {
      "list_attribute_api_slug_or_id": "value"
    }
  }
}
```

Semantics:

- List entries point at parent records; they are not records themselves.
- `parent_object` is the object slug or ID for the parent record.
- `parent_record_id` must be the returned record ID for the parent record.
- `entry_values` are list-entry attributes, not object attributes. A list entry can have values such as pipeline stage or list-specific source while the parent record keeps object values such as name and email.
- Attio allows multiple list entries for the same parent record when using create. Unique list-entry attributes can still produce duplicate/conflict errors.

Fields currently modeled from successful responses:

- `data.id.workspace_id`
- `data.id.list_id`
- `data.id.entry_id`
- `data.parent_record_id`
- `data.parent_object`
- `data.created_at`
- `data.entry_values`
- `data.status`, `data.outcome`, or `data.operation` when Attio reports create/update outcome
- `data.created` when Attio reports a boolean create/update marker

Metadata behavior:

- atcli tries to call `GET /lists`, `GET /objects`, and `GET /lists/{list}/attributes` before non-dry-run one-off entry writes and import planning with `--list`.
- When metadata is available, atcli validates writable list-entry attributes and checks that the list's `parent_object` accepts the requested/imported parent object.
- If metadata calls fail with a permission error, atcli warns and still attempts explicit writes without local list-entry validation, parent compatibility validation, or noun display.

## List Entry Assert By Parent

Endpoint:

```http
PUT https://api.attio.com/v2/lists/{list}/entries
```

Used by:

- `atcli entries upsert <list>` to create or update the list entry for a parent record.
- `atcli records import --list <list>` by default, because `--list-mode upsert` is the default.

Payload shape:

```json
{
  "data": {
    "parent_record_id": "record-id",
    "parent_object": "people",
    "entry_values": {
      "list_attribute_api_slug_or_id": "value"
    }
  }
}
```

Semantics:

- If exactly one entry exists for the parent record, Attio updates it.
- If no entry exists for the parent record, Attio creates it.
- If multiple entries exist for the same parent record, Attio reports `MULTIPLE_MATCH_RESULTS`; atcli surfaces this as a duplicate-resolution failure.
- This assert behavior is preferred for rerunnable imports that should not create duplicate list entries for corrected rows.

Error behavior:

- Command-facing errors normalize missing auth, missing `list_entry:read-write`, validation failures, duplicate entry/unique conflicts, multiple-match failures, rate limits, and network timeouts.
- API error status and response bodies remain wrapped in the returned error for debugging.
- The active bearer token is redacted from preserved API error bodies if an upstream response echoes it.

## CSV Import Planning and Apply

`atcli records import <object> <csv>` is a local dry-run planner unless `--apply` is present. Apply mode reuses the planned row payloads and then calls Attio record write endpoints. When `--list` is present, apply mode writes a list entry after each successful record write.

Endpoints used when credentials/scopes allow metadata validation:

- `GET /objects`
- `GET /objects/{object}/attributes`
- `GET /lists` when `--list` is present
- `GET /lists/{list}/attributes` when `--list` is present

Write endpoints used only when `--apply` is present:

- Create mode: `POST /objects/{object}/records`
- Upsert/assert mode: `PUT /objects/{object}/records?matching_attribute={attribute}`
- List create mode: `POST /lists/{list}/entries`
- List upsert/assert mode: `PUT /lists/{list}/entries`

CSV planning assumptions:

- CSV headers map to Attio attribute API slugs by default.
- `--map csv_column=attio_attribute` changes the target attribute for a CSV column.
- `--ignore csv_column` removes a CSV column from the planned payload.
- `--set attr=value` and `--set-json attr=json` add static values to every planned row.
- `--list list` enables list-entry planning and apply after record writes.
- `--list-mode create|upsert` controls list-entry writes. The default is `upsert`.
- `--entry-map csv_column=list_attribute` maps a CSV column to a list-entry attribute and removes that CSV column from the record payload.
- `--entry-set attr=value` adds a static string list-entry value to every planned entry.
- Duplicate target attributes and conflicts between mapped and static attributes are rejected locally.
- Empty CSV cells are omitted from the planned values. They do not clear existing Attio values.
- Explicit clearing is intentionally deferred to a future command or flag.
- Complex attributes can be supplied through static `--set-json` values when the first-pass CSV conversion is not rich enough.

Supported first-pass conversions when attribute metadata is available:

| Attio attribute kind | CSV behavior |
| --- | --- |
| text-like, personal/name-like, email address, domain, phone number | trimmed string passthrough |
| select/status | trimmed option title passthrough |
| relationship/reference-like or unknown complex types | trimmed string passthrough until richer conversion exists |
| number | parsed as a JSON number-compatible float |
| checkbox/boolean | parses `true/false`, `yes/no`, `y/n`, and `1/0` |
| date | validates and emits ISO `YYYY-MM-DD` |
| timestamp/date-time | validates RFC3339 and emits UTC RFC3339 |
| metadata-marked multivalue attributes with `--multi-sep` | splits the cell, trims parts, and omits empty parts |

Matching behavior:

- Import planning and apply default to upsert/assert mode.
- Upsert/assert mode is preferred for CSV imports because corrected imports should be rerunnable without creating duplicate records.
- Upsert/assert mode uses the same safe match defaults as `records upsert`.
- Metadata validation checks that the matching attribute exists, is unique, and has a row value when metadata is available.
- If metadata is blocked by missing `object_configuration:read`, planning can continue only when the match attribute was explicit; otherwise the user must pass `--match`.

Apply behavior:

- The command never calls record write endpoints unless `--apply` is present.
- Apply mode writes rows sequentially and reports a result for each planned row.
- By default, the command stops after the first failed row and marks remaining planned rows as skipped.
- `--continue-on-error` keeps processing rows after validation or write failures.
- If a record write fails, the corresponding list-entry write is skipped and the row is reported as a record failure.
- If a list-entry write fails after a record write succeeds, the record ID/status are retained, the row is reported as failed, and the entry status/error describe the list-entry failure.
- `--errors <csv>` writes only failed input rows. It preserves original CSV columns and appends `atcli_row_number`, `atcli_mode`, `atcli_object`, `atcli_matching_attribute`, `atcli_status`, `atcli_errors`, `atcli_list`, `atcli_list_mode`, `atcli_record_status`, `atcli_record_id`, `atcli_entry_status`, and `atcli_entry_id`.
- Failed-row CSV files are not created when no rows fail.
- Row errors are sanitized before table output, JSONL output, and failed-row CSV export.

Apply output assumptions:

- Table output includes planned, succeeded, failed, skipped, created, updated, elapsed milliseconds, and row-level statuses.
- JSONL apply output emits one `row` event per planned row and one final `summary` event.
- Row events include `record_id` when Attio returns `data.id.record_id`.
- When `--list` is present, row events include `list`, `list_mode`, `record_status`, `entry_status`, `entry_id` when available, and separate record/entry write endpoint markers.
- Summary events include machine-readable row-number to record-ID mappings for successful rows.
- Created/updated counts are best-effort. They are incremented only when Attio returns `data.created` or an outcome/status/operation value that clearly says created or updated.

Rate-limit and retry assumptions:

- Attio documents HTTP `429` responses for rate limits and a `Retry-After` header that identifies when the limit resets.
- atcli parses `Retry-After` as either delta seconds or an HTTP date.
- When `Retry-After` is present, import apply sleeps for that row before retrying.
- When `Retry-After` is absent or unparsable, import apply uses bounded exponential backoff starting at 100ms and capped at 2s.
- Retries are row-scoped. A rate-limited row does not discard previously printed or accumulated successful row results.
- If row retries are exhausted, that row is reported as failed and normal stop or continue-on-error behavior applies.

Import order guidance:

- Plan and apply linked parent objects first, such as `companies`.
- Then plan and apply child objects, such as `people`, using stable unique values from the parent import.
- Review dry-run JSONL row validation before using `--apply`.
