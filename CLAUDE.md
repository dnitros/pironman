# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go CLI + daemon that controls a Pironman 5 case (base edition) on a Raspberry Pi 5: WS2812 RGB strip (SPI), SSD1306 OLED (I2C), case-fan relay (GPIO character device), and the power button (evdev). Requires Go 1.27.1+.

## Commands

```sh
go build -o pironman ./cmd/pironman   # build
go test ./...                          # run all tests
go test ./internal/rgb/...             # run one package's tests
go test ./internal/rgb/ -run TestName  # run a single test
go vet ./...                           # static checks
```

There is no Makefile or linter config beyond `go vet`.

The daemon (`daemon run`) opens real hardware at startup, so it can't run standalone on a dev machine — `internal/hardware`, `internal/rgb`, `internal/oled`, `internal/fan`, and `internal/powerbutton` are unit-tested against hand-rolled fakes instead. Tests named `*_internal_test.go` are white-box (same package, exercise unexported helpers); plain `*_test.go` files are black-box.

For local CLI/daemon testing without root:

```sh
PIRONMAN_SOCKET_PATH=/tmp/pironman.sock ./pironman doctor
```

See `README.md` for the full deployment flow (`daemon install`/`enable`/`start`, the `pironman` group, SPI/I2C enablement).

## Architecture

**Two processes, one socket.** The CLI and a root-owned daemon (`pironman daemon run`) talk over a Unix domain socket (`internal/ipc`, default `/run/pironman/pironman.sock`, override via `PIRONMAN_SOCKET_PATH`), exchanging newline-delimited JSON `Request`/`Response` frames — see [ADR-0001](docs/adr/0001-newline-delimited-json-ipc.md). The socket is `root:pironman`-owned at `0660`; `daemon install` creates that group and adds the invoking user so the CLI doesn't need sudo for normal commands — see [ADR-0002](docs/adr/0002-socket-group-permissions.md). The `daemon` subcommands themselves (`install`/`start`/`stop`/`enable`/`disable`/`uninstall`) always require sudo, enforced directly in `internal/cli/daemon.go`, independent of group membership.

**Layering**, outside-in:
1. `internal/cli` — cobra commands. Each CLI command (`rgb`, `oled`, `fan`, `status`, `doctor`, `daemon`) builds an `ipc.Request` and sends it over the socket via `internal/ipc` client code.
2. `internal/cli/daemon.go` (`runDaemon`) — the daemon's composition root. Loads config, constructs each subsystem's store/machine, builds the IPC handler map, starts tick loops and the power-button watcher, then serves until `SIGINT`/`SIGTERM`.
3. `internal/handlers` — one handler-building function per subsystem (`RGBHandlers`, `OLEDHandlers`, `FanHandlers`, `StatusHandler`). Each handler validates IPC args, persists the change to `internal/config` under a shared `*sync.Mutex`, then calls into the subsystem's store/machine. Persist-then-apply ordering matters: config is the source of truth restored on the next daemon start.
4. Subsystem packages (`internal/rgb.Store`, `internal/oled.Machine`, `internal/fan.Machine`, `internal/powerbutton.Classifier`) — hold in-memory state behind their own mutex, apply it to a hardware interface, and expose it read-only. `oled.Machine` and `fan.Machine` also expose a `Tick()` driven by the daemon's tick loops (`startTickLoop` in `daemon.go`); `powerbutton.Classifier` turns raw press/release events into the four classified `Event`s (click, double-click, long-press, long-press-released) that `dispatchPowerButtonEvent` routes to OLED page changes or shutdown.
5. `internal/hardware` — thin interfaces (`Relay`, `WS2812Strip`, `PowerButtonWatcher`, ...) with real implementations behind `//go:build linux` and stub implementations behind `//go:build !linux` that return "unsupported on this platform" errors. This is what lets the package build (though not run its hardware path) on a non-Linux dev machine. GPIO lines are resolved by kernel-assigned name (e.g. `GPIO6`), not a fixed `/dev/gpiochipN` number, since the chip number varies across kernels/OS images.

**Config** (`internal/config`): a single YAML file (default `/etc/pironman/config.yaml`, override via `PIRONMAN_CONFIG_PATH`) holding RGB/OLED/fan settings, loaded with built-in defaults (`config.Default()`) if the file doesn't exist yet. Handlers mutate a copy, save it, then swap it in — never save partial state.

**Daemon shutdown**: `serveDaemon` runs a list of `shutdownHooks` (closures) after `ipc.Server.Serve` returns, turning off RGB/fan so they don't stay energized when the daemon isn't managing them. `Server.Serve` drains in-flight connections via `sync.WaitGroup` first, so a hook never races a handler's hardware write — see [ADR-0003](docs/adr/0003-daemon-shutdown-hooks-and-connection-draining.md).

**Domain vocabulary** (OLED page vs. page advance vs. content scroll, press event, PWM fan vs. case fan, RGB strip) is defined precisely in `CONTEXT.md` — read it before touching OLED or power-button code, since the terms are easy to conflate and the codebase uses them exactly as defined there.

## Agent skills

### Issue tracker

Issues live in Linear, team **Personal** (`PER`), project **Pironman 5**. See `docs/agents/issue-tracker.md`.

### Triage labels

Default canonical labels, used as-is. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
