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
