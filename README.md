# Pironman

A CLI + daemon for controlling a Pironman 5 case (base edition) on a Raspberry Pi 5.

## Status

Phase 0 (scaffolding) is landing incrementally, one Linear ticket per PR — see `PLAN.md` §6 for the full phase breakdown. So far:

- [x] Module scaffold, cobra CLI, and the `daemon run` / `ping` IPC round-trip (PER-2)
- [ ] `daemon install`/`uninstall`/`start`/`stop` (systemd lifecycle)
- [ ] `status`/`doctor` commands
- [ ] Config load/save

See also:

- [`PROMPT.md`](PROMPT.md) — the original task brief
- [`PLAN.md`](PLAN.md) — the phased implementation plan
- [`CONTEXT.md`](CONTEXT.md) — domain glossary
- [`docs/adr/`](docs/adr/) — architectural decisions

Work is tracked in Linear (see [`docs/agents/issue-tracker.md`](docs/agents/issue-tracker.md)), linked to this repo via Linear's GitHub integration.

## Development

Requires Go 1.27.1+.

```sh
go build -o pironman ./cmd/pironman
```

The CLI and daemon talk over a Unix domain socket (default `/run/pironman/pironman.sock`, override with `PIRONMAN_SOCKET_PATH`). `/run` needs root, so for local testing:

```sh
PIRONMAN_SOCKET_PATH=/tmp/pironman.sock ./pironman daemon run &
PIRONMAN_SOCKET_PATH=/tmp/pironman.sock ./pironman ping
```

Run tests with `go test ./...` — no Pi hardware required for anything built so far.
