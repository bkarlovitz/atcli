# atcli

An Attio CLI built for humans and agents.

## Quick Start

```bash
go test ./...
go build -o bin/atcli .
```

Authenticate with an Attio API key or access token:

```bash
./bin/atcli auth
```

If the OS credential store is unavailable, use an environment variable:

```bash
export ATTIO_ACCESS_TOKEN='your-token-here'
./bin/atcli whoami
```

Use dry runs for one-off record writes until the payload looks right:

```bash
./bin/atcli records upsert companies \
  --match domains \
  --set name='Example Co' \
  --set-json 'domains=["example.com"]' \
  --dry-run
```

Plan CSV imports without writing records:

```bash
./bin/atcli records import companies ./companies.csv \
  --match domains \
  --map 'Company Name=name' \
  --map 'Domain=domains' \
  --output jsonl
```

Apply CSV imports only after reviewing the plan. Upsert/assert mode is the default and is preferred for rerunnable imports:

```bash
./bin/atcli records import companies ./companies.csv \
  --match domains \
  --map 'Company Name=name' \
  --map 'Domain=domains' \
  --errors ./companies-errors.csv \
  --apply
```

## Documentation

Start with [docs/README.md](docs/README.md).

For coding agents, read [AGENTS.md](AGENTS.md) first. It is the compact operating guide for the codebase.
