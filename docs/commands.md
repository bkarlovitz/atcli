# Commands

## `atcli auth`

Authenticates by accepting an Attio API key or OAuth access token.

Behavior:

- Prompts interactively without echoing the token.
- Supports `--token-stdin` for non-interactive token input.
- Validates the token with Attio introspection unless `--no-validate` is used.
- Stores the token in the OS credential store.

Examples:

```bash
./bin/atcli auth
printf '%s' "$ATTIO_TOKEN" | ./bin/atcli auth --token-stdin
```

If the OS credential store is unavailable, use:

```bash
export ATTIO_ACCESS_TOKEN='token'
```

## `atcli whoami`

Shows the currently authenticated Attio workspace and, when allowed by scopes, the authorizing workspace member.

Behavior:

- Loads `ATTIO_ACCESS_TOKEN` first, then falls back to the OS credential store.
- Calls Attio token introspection.
- Prints workspace name, slug, ID, token type, and scopes.
- If introspection returns `authorized_by_workspace_member_id`, tries to fetch that workspace member.
- If member details require missing scopes, keeps the command successful and prints an explanatory details line.

Examples:

```bash
./bin/atcli whoami
ATTIO_ACCESS_TOKEN='token' ./bin/atcli whoami
```

Expected no-auth output:

```text
not authenticated; run `atcli auth` or set ATTIO_ACCESS_TOKEN
```

Expected credential-store failure output:

```text
could not read the OS credential store; unlock it, run `atcli auth`, or set ATTIO_ACCESS_TOKEN
```

## `atcli objects list`

Lists Attio objects available to the authenticated workspace.

Behavior:

- Loads `ATTIO_ACCESS_TOKEN` first, then falls back to the OS credential store.
- Calls Attio's list objects endpoint.
- Prints the object API slug, object ID, and Attio-provided singular/plural nouns.
- Does not derive singular or plural names from slugs.

Examples:

```bash
./bin/atcli objects list
```

Use object slugs or IDs in later commands. Standard Attio object slugs are usually plural, such as:

```text
people
companies
```

## `atcli objects attributes <object>`

Lists attributes for an Attio object.

Behavior:

- Treats `<object>` as an Attio object slug or ID, not as a noun to singularize or pluralize.
- Calls Attio's list attributes endpoint with target `objects`.
- Prints attribute API slug, attribute ID, title, type, writable, editable when returned, required, unique, multiselect, and archived status.
- Hides archived attributes by default.
- Includes archived attributes when `--all` is passed.

Examples:

```bash
./bin/atcli objects attributes people
./bin/atcli objects attributes companies
./bin/atcli objects attributes companies --all
```

## `atcli lists list`

Lists Attio lists available to the authenticated workspace.

Behavior:

- Loads `ATTIO_ACCESS_TOKEN` first, then falls back to the OS credential store.
- Calls Attio's list lists endpoint.
- Prints the list API slug, list ID, name, and parent object when available.
- Keeps list identifiers as Attio slugs or IDs.

Examples:

```bash
./bin/atcli lists list
```

## `atcli lists attributes <list>`

Lists attributes for entries in an Attio list.

Behavior:

- Treats `<list>` as an Attio list slug or ID.
- Calls Attio's list attributes endpoint with target `lists`.
- Prints list-entry attribute API slug, attribute ID, title, type, writable, editable when returned, required, unique, multiselect, and archived status.
- Hides archived attributes by default.
- Includes archived attributes when `--all` is passed.

Examples:

```bash
./bin/atcli lists attributes hiring-engineering
./bin/atcli lists attributes hiring-engineering --all
```

## Schema Planning Workflow

Before creating records or planning CSV imports:

```bash
./bin/atcli objects list
./bin/atcli objects attributes people
./bin/atcli objects attributes companies
./bin/atcli lists list
./bin/atcli lists attributes '<list-slug-or-id>'
```

Use the printed API slugs as CSV headers or later command arguments. Use list-entry attributes only for values that live on list entries; they are separate from the parent record's object attributes.

## `atcli records create <object>`

Creates one Attio record from shell flags.

Behavior:

- Treats `<object>` as an Attio object slug or ID. Standard object slugs are usually plural, such as `people` and `companies`.
- Sends the object argument exactly as provided. It never singularizes or pluralizes the argument.
- Parses repeated `--set attr=value` flags as string values.
- Parses repeated `--set-json attr=json` flags as JSON values, including arrays, objects, numbers, booleans, and null.
- Rejects malformed value flags, duplicate attribute names, empty attribute names, and malformed JSON before any API call.
- For non-dry-run writes, loads `ATTIO_ACCESS_TOKEN` first, then falls back to the OS credential store.
- Before writes, tries to fetch object and attribute metadata with `object_configuration:read`.
- When metadata is available, validates unknown attributes, required writable attributes, and non-writable or non-editable attributes before calling the write endpoint.
- When metadata is blocked by missing scope, prints a warning and still attempts the write with the explicit user-provided attributes. Local validation and noun display are skipped.
- Uses Attio-provided `singular_noun` and `plural_noun` for output only when metadata is available.
- Supports `--output table` and `--output json`.
- Supports `--dry-run`, which prints the exact JSON payload that would be sent. Current dry runs do not require auth and do not call Attio metadata or write endpoints.

Company examples:

```bash
./bin/atcli records create companies \
  --set name='Example Co' \
  --set-json 'domains=["example.com"]'

./bin/atcli records create companies \
  --set name='Example Co' \
  --set-json 'domains=["example.com"]' \
  --dry-run
```

People examples:

```bash
./bin/atcli records create people \
  --set-json 'email_addresses=["ada@example.com"]' \
  --set-json 'name=[{"first_name":"Ada","last_name":"Lovelace","full_name":"Ada Lovelace"}]'

./bin/atcli records create people \
  --set-json 'email_addresses=["ada@example.com"]' \
  --set-json 'name=[{"first_name":"Ada","last_name":"Lovelace","full_name":"Ada Lovelace"}]' \
  --output json
```

Dry-run output is the write payload, marked as a non-write:

```text
DRY RUN: no write endpoint called
Payload:
{
  "data": {
    "values": {
      "domains": [
        "example.com"
      ],
      "name": "Example Co"
    }
  }
}
```

## `atcli records upsert <object>`

Creates or updates one Attio record from shell flags using a unique matching attribute. This is the preferred mental model for rerunnable one-off imports: use upsert when rerunning the same command should update the same record instead of creating duplicates.

Behavior:

- Treats `<object>` as an Attio object slug or ID. It never singularizes or pluralizes the argument.
- Calls Attio's assert record endpoint with the matching attribute in the `matching_attribute` query parameter.
- Parses the same repeated `--set attr=value` and `--set-json attr=json` flags as `records create`.
- Supports `--output table` and `--output json`.
- Supports `--dry-run`, which fetches metadata when credentials/scopes allow it, validates the payload, prints the exact JSON body that would be sent, and avoids the write endpoint.
- Requires `--match <attribute>` unless the object slug has a safe default.
- Uses safe defaults only for exact standard slugs: `companies` -> `domains`, `people` -> `email_addresses`, `users` -> `primary_email_address`, and `workspaces` -> `workspace_id`.
- Requires explicit `--match` for `deals`, custom objects, object IDs, unknown slugs, and singular/plural variants such as `company` or `person`.
- When metadata is available, verifies the match attribute exists, is unique, and has a non-null value in the payload.
- When metadata is blocked by missing `object_configuration:read`, continues only if `--match` was explicit, warns that local validation and match uniqueness validation were skipped, and still attempts the upsert.
- Normalizes common write failures into actionable errors for missing auth, missing `record_permission:read-write`, validation failures, non-unique matching attributes, rate limits, and network timeouts while preserving Attio status/body details.

Company examples with explicit match:

```bash
./bin/atcli records upsert companies \
  --match domains \
  --set name='Example Co' \
  --set-json 'domains=["example.com"]'

./bin/atcli records upsert companies \
  --match domains \
  --set name='Example Co' \
  --set-json 'domains=["example.com"]' \
  --dry-run
```

People examples with explicit match:

```bash
./bin/atcli records upsert people \
  --match email_addresses \
  --set-json 'email_addresses=["ada@example.com"]' \
  --set-json 'name=[{"first_name":"Ada","last_name":"Lovelace","full_name":"Ada Lovelace"}]'

./bin/atcli records upsert people \
  --match email_addresses \
  --set-json 'email_addresses=["ada@example.com"]' \
  --set-json 'name=[{"first_name":"Ada","last_name":"Lovelace","full_name":"Ada Lovelace"}]' \
  --output json
```

Dry-run output includes the match attribute and the exact write body:

```text
DRY RUN: no write endpoint called
Matching attribute: domains
Payload:
{
  "data": {
    "values": {
      "domains": [
        "example.com"
      ],
      "name": "Example Co"
    }
  }
}
```

## `atcli records import <object> <csv>`

Plans a CSV record import without writing records. This command is dry-run only: it never calls Attio record create or assert endpoints.

Behavior:

- Treats `<object>` as an Attio object slug or ID. Standard object slugs are usually plural, such as `people` and `companies`.
- Reads CSV files with a header row and preserves CSV row numbers for validation output.
- Rejects empty headers, duplicate headers, inconsistent row lengths, empty files, unreadable files, mapping conflicts, and malformed static values before planning rows.
- Maps CSV headers to Attio attribute slugs by default.
- Supports repeated `--map csv_column=attio_attribute` for agent-friendly CSV headers.
- Supports repeated `--ignore csv_column` to leave columns out of the planned payload.
- Supports repeated `--set attr=value` and `--set-json attr=json` for static values added to every planned row.
- Empty CSV cells are omitted and do not clear existing Attio values.
- Supports `--multi-sep <sep>` to split cells for metadata-marked multivalue attributes.
- Defaults to upsert planning. Use `--mode create` for create-only planning.
- Uses the same safe matching defaults as `records upsert`: `companies` -> `domains`, `people` -> `email_addresses`, `users` -> `primary_email_address`, and `workspaces` -> `workspace_id`.
- Requires explicit `--match` for custom objects, object IDs, `deals`, unknown slugs, and singular/plural variants without a safe default.
- Fetches object attributes when credentials and `object_configuration:read` allow it.
- When metadata is available, validates unknown attributes, writable/editable status, required values, matching attribute presence, and matching attribute uniqueness.
- When metadata is unavailable, still plans explicit input when the match policy allows it and emits warnings that local validation was skipped.
- Supports `--output table` and `--output jsonl`.

Company import with header mapping:

```bash
./bin/atcli records import companies ./companies.csv \
  --match domains \
  --map 'Company Name=name' \
  --map 'Primary Domain=domains' \
  --ignore 'Internal Notes' \
  --multi-sep ';'
```

Create-mode planning:

```bash
./bin/atcli records import companies ./companies.csv \
  --mode create \
  --map 'Company Name=name' \
  --map 'Primary Domain=domains'
```

Static values and JSON escape hatch:

```bash
./bin/atcli records import companies ./companies.csv \
  --match domains \
  --set lifecycle_stage='Prospect' \
  --set-json 'tags=["agent-import","reviewed"]'
```

Agent-oriented JSONL output:

```bash
./bin/atcli records import people ./people.csv \
  --match email_addresses \
  --map 'Email=email_addresses' \
  --map 'Full Name=name' \
  --output jsonl
```

Each JSONL line is one planned row with row number, mode, object, matching attribute, values, warnings, skipped empty cells, and validation status. Record IDs are not invented; the planner omits record identifiers unless a future lookup can prove them.

Recommended import order:

1. Import or plan linked parent objects first, usually `companies`.
2. Review JSONL validation output for missing required values and match warnings.
3. Import or plan linked child objects, usually `people`, after parent identifiers or unique match values are stable.
