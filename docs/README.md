# atcli Docs

This directory documents how to work on atcli.

The docs are intentionally short and link-heavy. They should help a human developer navigate the project and help an agent recover the current design quickly without rereading every source file.

## Reading Order

For humans:

1. [../README.md](../README.md) for quick start commands.
2. [architecture.md](architecture.md) for package layout.
3. [commands.md](commands.md) for current CLI behavior.
4. [attio-api.md](attio-api.md) for API assumptions and source links.

For agents:

1. [../AGENTS.md](../AGENTS.md) first.
2. Open only the focused doc for the area being changed.
3. Read the relevant source files listed in that doc.

## Documentation Rules

- Keep docs factual and current with the code.
- Prefer small files with stable headings over long narrative docs.
- Link to source files and external API docs instead of copying large references.
- When adding a command, update [commands.md](commands.md).
- When adding or changing an Attio endpoint, update [attio-api.md](attio-api.md).
- When changing package boundaries or auth flow, update [architecture.md](architecture.md) and [../AGENTS.md](../AGENTS.md).
