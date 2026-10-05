# CLAUDE.md

Go CLI and root-owned daemon for a Pironman 5 case (base edition) on a Raspberry Pi 5. It controls the WS2812 RGB strip (SPI), the SSD1306 OLED (I2C), the case-fan relay (GPIO character device) and the power button (evdev). `README.md` covers install and usage.

## Commands

```sh
go build -o pironman ./cmd/pironman
go test ./...
go test ./internal/oled/ -run TestName
go vet ./...
```

`go vet` is the only linter. `daemon run` needs the real hardware, so hardware-facing code is tested against hand-rolled fakes. `*_internal_test.go` files are white-box tests in the same package; plain `*_test.go` files are black-box. To run CLI commands without root, set `PIRONMAN_SOCKET_PATH=/tmp/pironman.sock`.

## Architecture

The CLI sends newline-delimited JSON requests to the daemon over a Unix socket (`internal/ipc`, default `/run/pironman/pironman.sock`). The socket is `root:pironman` at `0660`, so members of the `pironman` group need no `sudo`. The `daemon` subcommands always require root, checked in `internal/cli/daemon.go`.

Layers, outside in:

1. `internal/cli`: cobra commands. Most send one IPC request; `daemon` drives systemd and `version` runs locally.
2. `runDaemon` in `internal/cli/daemon.go`: the composition root. It loads config, builds each subsystem, registers the IPC handlers, and starts the tick loops (`startTickLoop`) and the power-button watcher.
3. `internal/handlers`: one function per subsystem (`RGBHandlers`, `OLEDHandlers`, `FanHandlers`, `StatusHandler`). A handler validates its arguments, saves the change to config, then applies it. Save before apply: config is what the daemon restores on its next start.
4. Subsystems: `rgb.Store`, `oled.Machine`, `fan.Machine` and `powerbutton.Classifier`. Each guards its state with its own mutex and drives a `hardware` interface. The OLED and fan machines advance on `Tick()`. `rgb.Store` runs its own animation goroutine, because each style's frame delay depends on its speed setting. `dispatchPowerButtonEvent` routes classified press events to the OLED or to shutdown.
5. `internal/hardware`: small interfaces with real implementations. The GPIO relay and power button have `//go:build !linux` stubs, so the module builds on macOS. GPIO lines are found by name (`GPIO6`), because the `/dev/gpiochipN` number varies between kernels.

**Config** (`internal/config`): one YAML file, `/etc/pironman/config.yaml` or `PIRONMAN_CONFIG_PATH`. `config.Default()` applies when the file is missing. Handlers change a copy, save it, then swap it in.

**Shutdown**: after `ipc.Server.Serve` returns, `serveDaemon` runs `shutdownHooks`, which turn the RGB strip and fan off. `Serve` first waits for in-flight connections, so a hook never races a handler's hardware write. A failing hook is logged and the rest still run.

**OLED pages**: each page has its own file in `internal/oled` (`mix.go`, `performance.go`, `ips.go`, `disk.go`) with its own `*_internal_test.go`. A page file has a values function (stats → strings and percentages) and a render function (values → `*image.Gray`). Shared drawing (`drawText`, `drawBar`, `fillRect`, the fonts) lives in `render.go`. Page files depend only on `render.go`, never on each other. Fonts are embedded TTFs, each in `internal/oled/fonts/<family>/` with its own `LICENSE`.

**Images**: `internal/imageconv` converts images to 128x64 1-bit `.pbm`, and `internal/pbm` reads and writes that format. Both are stateless and hardware-free.

## Vocabulary

Use these terms exactly:

- **OLED page**: a full-screen content: `mix`, `performance`, `ips`, `disk`, `image`.
- **Page advance**: switching pages, only by power-button press (click = next, double-click = previous).
- **Content scroll**: cycling on a timer through values within one page (IPs, disks, images).
- **OLED rotation**: display orientation, `0` or `180`. It flips pixels; it's unrelated to paging.
- **Press event**: `click`, `double-click`, `long-press` (shutdown prompt), `long-press-released` (shutdown).
- **PWM fan**: the Pi 5's own cooler, kernel-controlled. Read-only here, via sysfs.
- **Case fan**: the case's fans on the GPIO relay, run by one of five curves. Their built-in RGB follows the fan's power.
- **RGB strip**: the 4 WS2812 LEDs on the main board, driven by `pironman rgb`.

## Agent skills

### Issue tracker

Issues live in Linear, team **Personal** (`PER`), project **Pironman 5**. See `docs/agents/issue-tracker.md`.

### Triage labels

Default canonical labels, used as-is. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context. The glossary is the Vocabulary section above; there is no `CONTEXT.md` or `docs/adr/`. See `docs/agents/domain.md`.
