# Pironman

A CLI + daemon for controlling a Pironman 5 case (base edition) on a Raspberry Pi 5.

## Prerequisites

- Go (version pinned in `go.mod`)
- A Raspberry Pi 5 to run the daemon against real hardware — the CLI, daemon skeleton, and test suite all build and run on macOS/Linux without it

## Build

```
go build ./...
```

## Test

```
go test ./...
```

## Run locally

The CLI and daemon talk over a Unix domain socket (default `/run/pironman/pironman.sock`, override with `PIRONMAN_SOCKET_PATH`):

```
go run ./cmd/pironman daemon run &
go run ./cmd/pironman ping
```

`daemon run` is normally started by the systemd unit, not invoked directly by a user — see `PLAN.md` for the full command surface as it lands.

## Documentation

- [`PROMPT.md`](PROMPT.md) — the original task brief
- [`PLAN.md`](PLAN.md) — the phased implementation plan
- [`CONTEXT.md`](CONTEXT.md) — domain glossary
- [`docs/adr/`](docs/adr/) — architectural decisions

Work is tracked in Linear (team Personal, project Pironman 5) — see [`docs/agents/issue-tracker.md`](docs/agents/issue-tracker.md) for the conventions.
