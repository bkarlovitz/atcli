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
