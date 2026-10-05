# CLAUDE.md

Go CLI and root-owned daemon for a Pironman 5 case (base edition) on a Raspberry Pi 5. It controls the WS2812 RGB strip (SPI), the SSD1306 OLED (I2C), the case-fan relay (GPIO character device) and the power button (evdev). `README.md` covers install and usage.

## Commands

```sh
make build
make test
make vet
go test ./internal/oled/ -run TestName  # single test
```

`make` lists every target, including `install` and `update`, which run on the Pi. `go vet` is the only linter. `daemon run` needs the real hardware, so hardware-facing code is tested against hand-rolled fakes. `*_internal_test.go` files are white-box tests in the same package; plain `*_test.go` files are black-box. To run CLI commands without root, set `PIRONMAN_SOCKET_PATH=/tmp/pironman.sock`.

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

**Releases**: a `v*` tag runs `.github/workflows/release.yml`, which builds `pironman-<GOOS>-<GOARCH>` with the tag stamped into `cli.version` and the repo into `selfupdate.Repo` via `-ldflags -X`, and publishes it with `checksums.txt`. `pironman update` (`internal/selfupdate`) downloads the latest release, verifies its SHA-256, renames it over the running binary, and restarts the service if it's running. `selfupdate.BinaryAsset` follows the build's `runtime.GOOS/GOARCH`, and an unstamped build falls back to the repo in its module path.

**Images**: `internal/imageconv` converts images to 128x64 1-bit `.pbm`, and `internal/pbm` reads and writes that format. Both are stateless and hardware-free.

## Vocabulary

`CONTEXT.md` defines the domain terms: OLED page, page advance, content scroll, OLED rotation, press event, PWM fan, case fan and RGB strip. Read it before touching OLED, fan or power-button code, and use its terms exactly.

## Agent skills

### Issue tracker

Issues live in Linear, team **Personal** (`PER`), project **Pironman 5**. See `docs/agents/issue-tracker.md`.

### Triage labels

Default canonical labels, used as-is. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` at the repo root; there is no `docs/adr/`. See `docs/agents/domain.md`.
