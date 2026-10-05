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

**Two processes, one socket.** The CLI and a root-owned daemon (`pironman daemon run`) talk over a Unix domain socket (`internal/ipc`, default `/run/pironman/pironman.sock`, override via `PIRONMAN_SOCKET_PATH`), exchanging newline-delimited JSON `Request`/`Response` frames. Traffic is low-frequency control commands, so plain JSON (inspectable with `socat`/`nc`, no codegen) is preferred over binary framing or an RPC framework. The socket is `root:pironman`-owned at `0660`; `daemon install` creates that group and adds the invoking user so the CLI doesn't need sudo for normal commands. A world-writable socket would let any local user drive the hardware, and per-connection `SO_PEERCRED` checks are more than a single-user Pi needs. The `daemon` subcommands themselves (`install`/`start`/`stop`/`enable`/`disable`/`uninstall`) always require sudo, enforced directly in `internal/cli/daemon.go`, independent of group membership.

**Layering**, outside-in:
1. `internal/cli` — cobra commands. Each CLI command (`rgb`, `oled`, `fan`, `status`, `doctor`, `daemon`) builds an `ipc.Request` and sends it over the socket via `internal/ipc` client code.
2. `internal/cli/daemon.go` (`runDaemon`) — the daemon's composition root. Loads config, constructs each subsystem's store/machine, builds the IPC handler map, starts tick loops and the power-button watcher, then serves until `SIGINT`/`SIGTERM`.
3. `internal/handlers` — one handler-building function per subsystem (`RGBHandlers`, `OLEDHandlers`, `FanHandlers`, `StatusHandler`). Each handler validates IPC args, persists the change to `internal/config` under a shared `*sync.Mutex`, then calls into the subsystem's store/machine. Persist-then-apply ordering matters: config is the source of truth restored on the next daemon start.
4. Subsystem packages (`internal/rgb.Store`, `internal/oled.Machine`, `internal/fan.Machine`, `internal/powerbutton.Classifier`) — hold in-memory state behind their own mutex, apply it to a hardware interface, and expose it read-only. `oled.Machine` and `fan.Machine` expose a `Tick()` driven by the daemon's fixed-interval tick loops (`startTickLoop` in `daemon.go`); `powerbutton.Classifier` turns raw press/release events into the four classified `Event`s (click, double-click, long-press, long-press-released) that `dispatchPowerButtonEvent` routes to OLED page changes or shutdown. `rgb.Store` instead manages its own self-paced animation goroutine internally (started/stopped by `Store` methods, not by a daemon tick loop) while an animated style (e.g. `breathing`) is active, since each style's frame delay is derived live from its speed setting rather than a fixed interval.
5. `internal/hardware` — thin interfaces (`Relay`, `WS2812Strip`, `PowerButtonWatcher`, ...) with real implementations behind `//go:build linux` and stub implementations behind `//go:build !linux` that return "unsupported on this platform" errors. This is what lets the package build (though not run its hardware path) on a non-Linux dev machine. GPIO lines are resolved by kernel-assigned name (e.g. `GPIO6`), not a fixed `/dev/gpiochipN` number, since the chip number varies across kernels/OS images.

**Image conversion** (`internal/imageconv`, `internal/pbm`): stateless, hardware-free utility packages the `OLEDHandlers`' `oled.image` handler calls into before persisting a path onto `oled.Machine` — `imageconv.Convert`/`PersistImage` scale, dither, and write a 128x64 1-bit `.pbm` file; `pbm` decodes/encodes that format. Neither holds state or sits behind a hardware interface, unlike the layer-4 subsystem packages above.

**OLED rendering**: each page lives in its own file (`mix.go`, `performance.go`, `ips.go`, `disk.go`) with its own `*_internal_test.go`. A page file holds a values function (stats snapshot → strings/percentages) and a render function (values → `*image.Gray`), so each half is tested on its own. Drawing helpers shared by every page (`drawText`, `drawBar`, `fillRect`, the fonts) live in `render.go`; a page file never depends on another page's file. Text is rendered from embedded TTFs via `golang.org/x/image/font/opentype`. Each font lives in its own `internal/oled/fonts/<family>/` directory alongside its own `LICENSE`, so adding a font never means splitting a shared licence file.

**Config** (`internal/config`): a single YAML file (default `/etc/pironman/config.yaml`, override via `PIRONMAN_CONFIG_PATH`) holding RGB/OLED/fan settings, loaded with built-in defaults (`config.Default()`) if the file doesn't exist yet. Handlers mutate a copy, save it, then swap it in — never save partial state.

**Daemon shutdown**: `serveDaemon` runs a list of `shutdownHooks` (closures) after `ipc.Server.Serve` returns, turning off RGB/fan so they don't stay energized when the daemon isn't managing them. `Server.Serve` drains in-flight connections via `sync.WaitGroup` first, so a hook never races a handler's hardware write. A hook's own failure is logged, not fatal, so it never blocks exit or later hooks.

**Domain vocabulary**: the codebase uses these terms exactly, and they're easy to conflate:

- **OLED page**: one of the full-screen contents: `mix`, `performance`, `ips`, `disk`, `image`. Not "screen" or "view".
- **Page advance**: switching pages, only ever by a power-button press (click = next, double-click = previous), never by a timer.
- **Content scroll**: cycling on a timer through several values *within* one page, such as IPs, disks or images. The page itself doesn't change.
- **OLED rotation**: the display's physical orientation, `0` or `180` (`config.OLED.Rotation`). It flips every page's pixels. Never use "rotation" for page advance or content scroll.
- **Press event**: one of `click` (wake/next page), `double-click` (previous page), `long-press` (shutdown-confirmation screen) and `long-press-released` (powering-off screen, then shutdown).
- **PWM fan**: the Pi 5's own active cooler, governed by the kernel. This tool only reads its state and speed via sysfs.
- **Case fan**: the Pironman case's fans on a GPIO relay, controlled together by one of five curves (`always_on` plus four temperature-gated). Their built-in RGB is powered by the same connector, so it follows the fan on/off.
- **RGB strip**: the 4 WS2812 LEDs on the main board (SPI0/GPIO10), driven by `pironman rgb`.

## Agent skills

### Issue tracker

Issues live in Linear, team **Personal** (`PER`), project **Pironman 5**, via the `mcp__linear-server__*` tools. Each ticket gets one branch, named by its `gitBranchName`, and one PR on GitHub (`dnitros/pironman`, via `gh`). Status, QA and review summaries go on the Linear ticket, not the PR.

### Triage labels

`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`, used as-is. A label that doesn't exist yet in Linear is created on first use.
