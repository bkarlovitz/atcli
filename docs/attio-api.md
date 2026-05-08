# Attio API Notes

This file records the Attio API assumptions the code currently depends on.

Primary docs:

- REST API overview: https://docs.attio.com/rest-api/overview
- Authentication guide: https://docs.attio.com/rest-api/guides/authentication
- List objects: https://docs.attio.com/rest-api/endpoint-reference/objects/list-objects
- List all lists: https://docs.attio.com/rest-api/endpoint-reference/lists/list-all-lists
- List attributes: https://docs.attio.com/rest-api/endpoint-reference/attributes/list-attributes
- Create a record: https://docs.attio.com/rest-api/endpoint-reference/records/create-a-record
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

- API error status and response bodies are preserved for validation, permission, and rate-limit responses.
- The active bearer token is redacted from preserved API error bodies if an upstream response echoes it.
