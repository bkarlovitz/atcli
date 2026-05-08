# Attio API Notes

This file records the Attio API assumptions the code currently depends on.

Primary docs:

- REST API overview: https://docs.attio.com/rest-api/overview
- Authentication guide: https://docs.attio.com/rest-api/guides/authentication
- List objects: https://docs.attio.com/rest-api/endpoint-reference/objects/list-objects
- List all lists: https://docs.attio.com/rest-api/endpoint-reference/lists/list-all-lists
- List attributes: https://docs.attio.com/rest-api/endpoint-reference/attributes/list-attributes
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
