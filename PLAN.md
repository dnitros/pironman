# Pironman Implementation Plan

Phased implementation plan for `pironman`, a CLI + daemon controlling a Pironman 5 case (base edition) on a Raspberry Pi 5. Written after a grilling session with the maintainer; see `CONTEXT.md` for the domain glossary and `docs/adr/` for the architectural decisions this plan depends on.

## 1. High-level architecture

One binary, `pironman`, built from `github.com/dnitros/pironman`, running in two modes:

- **CLI mode** (default): parses a subcommand, opens the control socket, sends one request, prints the response, exits. Never touches hardware directly and never needs root, except `daemon install`/`uninstall`.
- **Daemon mode** (`pironman daemon run`, invoked by the systemd unit's `ExecStart`, never run directly by a user): runs as root, owns every hardware resource, and serves the control socket.

The daemon is a single process with five long-lived goroutines:

1. **RGB loop** — applies the current RGB state (color/brightness in v1; effects in v2) to the WS2812 strip over SPI.
2. **OLED loop** — drives the current page's rendering and the sleep-timeout/content-scroll timers (see `CONTEXT.md`: page advance vs. content scroll).
3. **Case-fan loop** — evaluates the case fan's on/off/auto state against the PWM fan's read-only temperature reading (v1: fixed threshold; v2: selectable mode) and drives the GPIO relay.
4. **PWM-fan reader** — polls `/sys/class/thermal/cooling_device0/cur_state` and the `fan1_input` hwmon path on a short interval; feeds the case-fan loop and the `status` command. Read-only — see ADR context: this fan is the Pi 5's own official active cooler, governed by the kernel, not by this tool (`CONTEXT.md`: PWM fan vs. case fan).
5. **Power-button watcher** — blocks on evdev reads, classifies press events (click / double-click / long-press / long-press-released — see `CONTEXT.md`), and dispatches the corresponding action (OLED advance/previous, shutdown-confirm, shutdown).

A shared in-process **state store** (a mutex-guarded struct, not a database) holds current RGB/OLED/fan state; the IPC server reads and mutates it, and the goroutines above react to it. Config load happens once at daemon startup; config save happens on any state-mutating command so state survives a restart.

## 2. Package / directory layout

Standard Go layout — one entrypoint, hardware isolated from domain logic:

```
cmd/pironman/
    main.go              — cobra root command, wires every subcommand

internal/
    cli/                 — cobra command tree: daemon, doctor, status, rgb, oled, fan
    daemon/              — orchestrates the five goroutines above; owns the state store
    ipc/                 — socket framing (client + server), see section 3
    config/              — YAML schema, load/save, PIRONMAN_CONFIG_PATH resolution
    hardware/
        gpio.go          — go-gpiocdev wrapper (case-fan relay line)
        i2cbus.go        — periph.io I2C bus open/wrap
        spibus.go        — periph.io SPI bus open/wrap
        ws2812.go        — hand-rolled WS2812 SPI bit-encoder
        ssd1306.go       — hand-rolled SSD1306 driver
        pwmfan.go        — sysfs read-only telemetry
        powerbutton_linux.go — hand-rolled evdev reader (stdlib syscall only)
        shutdowner.go    — shells out to `shutdown -h now`
    rgb/                 — RGB domain logic against a hardware.WS2812 interface
    oled/                — page state machine: advance/previous/sleep/content-scroll
    fan/                 — case-fan on/off/auto/mode logic + PWM-fan reading
    powerbutton/         — press-event classification → action dispatch
    systemdunit/         — unit file template, install/uninstall
    buildinfo/           — version/build metadata for `status`/`doctor`
```

Every `hardware/*` file exposes a small interface (e.g. `type Relay interface { Set(bool) error }`) that `rgb`/`oled`/`fan`/`powerbutton` depend on — see section 7 for why.

## 3. Socket IPC protocol

See [ADR-0001](docs/adr/0001-newline-delimited-json-ipc.md) for why newline-delimited JSON over a binary/RPC protocol.

- **Transport**: Unix domain socket, default `/run/pironman/pironman.sock`, override `PIRONMAN_SOCKET_PATH`. Permissions `root:pironman`, `0660` — see [ADR-0002](docs/adr/0002-socket-group-permissions.md).
- **Framing**: one JSON object per line (`\n`-terminated). `encoding/json.Marshal` never emits raw newlines inside a value, so this is safe framing without a length prefix.
- **Request**: `{"cmd": "rgb.color", "args": {"hex": "#ff0000"}}`
- **Response**: `{"ok": true, "data": {...}}` or `{"ok": false, "error": "human-readable message"}`
- **Lifecycle**: one connection per CLI invocation — connect, write one request line, read one response line, close. The client sets a read deadline (5s) so a stuck daemon fails fast instead of hanging the CLI.
- **Command namespace**: dotted strings mirroring the CLI tree (`rgb.on`, `rgb.color`, `oled.page`, `fan.state`, `status`, `doctor`) — one switch in `internal/ipc` dispatches to the matching domain package.

## 4. Draft config schema (YAML)

Persisted at `/etc/pironman/config.yaml` (override `PIRONMAN_CONFIG_PATH`), written by the daemon on every state-mutating command:

```yaml
rgb:
  enabled: true
  color: "#00ff00"
  brightness: 80
  style: solid    # solid | breathing | flow | flow_reverse (Phase 5, shipped) |
                  # rainbow | rainbow_reverse | hue_cycle (Phase 5, remaining)
  speed: 50       # 0-100; meaningless for the non-animated solid style

oled:
  enabled: true
  sleep_timeout_seconds: 10
  scroll_interval_seconds: 3
  page_order: [mix, performance, ips, disk]

fan:
  case_fan_state: auto     # on | off | auto  (v1)
                           # v2 adds: case_fan_mode, case_fan_custom_threshold_c
  # the PWM fan (Pi 5's own cooler) is never configured here — read-only, see CONTEXT.md
```

The v1 `auto` threshold (67.5°C, matching the original's "Balanced" curve) is a hardcoded constant in `internal/fan`, not a config field — it isn't configurable in v1, so a schema entry would be premature. `case_fan_mode` and its threshold fields get added in v2 when `fan mode <name>` ships.

## 5. Library selection

| Component | Approach | Library / path | Why |
|---|---|---|---|
| GPIO (case-fan relay) | Library | `github.com/warthog618/go-gpiocdev v0.9.1` | Uses the Linux `gpiochip` character-device ABI, which RP1 exposes. Which `/dev/gpiochipN` number the RP1 header-GPIO chip lands on varies by kernel/OS image (confirmed on real Pi 5 hardware: it landed on `gpiochip15`, not the originally guessed `gpiochip4`/`0`/`1`), so the relay's line is resolved by its kernel-assigned name (`gpiocdev.FindLine("GPIO6")`) instead of a chip number. Legacy `/dev/gpiomem`-based libraries (`periph.io`'s GPIO layer, `stianeikeland/go-rpio`) are not RP1-safe and are excluded. |
| I2C bus (OLED) | Library | `periph.io/x/conn/v3` + `periph.io/x/host/v3` (`i2creg`) | I2C character-device access (`/dev/i2c-1`) is unaffected by the RP1 GPIO-register change. |
| SPI bus (WS2812) | Library | `periph.io/x/conn/v3` (`spireg`) | Same rationale and package family as the I2C pick — one fewer dependency to justify. |
| SSD1306 driver | Hand-rolled | raw `periph` `i2c.Dev` + init/command sequence | Both the original Python implementation and the existing Go port independently hand-wrote this against the raw bus rather than using a third-party driver — confirms it's simple and stable enough not to need one. Pi 5/RP1 compatibility isn't explicitly documented for this approach; verify on-device in Phase 2 (see section 8). |
| WS2812 driver | Hand-rolled | SPI bit-encoder over `periph` `spi.Conn`, ~2.4 MHz clock | No maintained Go WS2812 library exists. The existing Go port's `EncodeWS2812GRB` is a working reference for the bit-encoding technique (not to be copied verbatim). |
| PWM-fan telemetry (read-only) | No library | `os.ReadFile` on `/sys/class/thermal/cooling_device0/cur_state` and the matching `hwmon*/fan1_input` | Plain sysfs reads — matches both reference implementations exactly. |
| Power button | Hand-rolled | stdlib `syscall` — `EVIOCGBIT`/`EVIOCGRAB` via the documented Linux ioctl-encoding macro, plus raw `input_event` reads | `github.com/holoplot/go-evdev` was evaluated (pure Go, actively maintained) but rejected: the ~3 primitives actually needed (open, one capability-check ioctl, one grab ioctl) don't justify pulling in ~600 lines of unused ioctl wrappers (uinput, force-feedback, LED/SND/SW) with no tagged release to pin against. |
| CLI framework | Library | `spf13/cobra` (version to be confirmed via context7 at implementation time) | The CLI surface is a genuine nested-subcommand tree (`daemon {install,uninstall,start,stop}`, `rgb {...}`, `oled {...}`, `fan {...}`); cobra removes the boilerplate stdlib `flag` would otherwise require, and is the de facto standard for this shape of tool. |

## 6. Delivery phases

**Phase 0 — Scaffolding.** Unchanged from the brief: repo/module layout, socket IPC skeleton (no-op ping/pong), daemon skeleton, `daemon install`/`uninstall` (systemd unit management), `daemon start`/`stop` wired to the installed service with the not-installed error case, `doctor`, config load/save skeleton.
*Done when*: `pironman daemon install && pironman daemon start` brings up a running daemon; `pironman doctor` round-trips a ping over the socket; `pironman daemon stop` on a never-installed system fails with a clear "run `daemon install` first" error.

*Scope note (decided during PER-5, confirmed during PER-1 close-out): `status` was dropped for this phase — with no hardware yet, it had nothing to report that `doctor` didn't already cover (reachability, installed/active state). `doctor` is Phase 0's single diagnostic command. `status` returns starting Phase 1 (below), once there's real hardware state to show.*

**Phase 1 — RGB base.** On/off, color, brightness (solid only). Unchanged from the brief.
*Done when*: `pironman rgb on|off|color|brightness` visibly changes the WS2812 strip; state survives a daemon restart via config.

*`status`/`doctor` split, starting this phase*: `status` reintroduces as the hardware-state command — reports current RGB state (on/off, color, brightness) first, growing per phase below. `doctor` stays the environment/setup diagnostic command, and gains its first hardware-prerequisite check here: SPI enabled, via a `/dev/spidev0.0` existence check (`os.Stat`, matching the existing `IsInstalled`/`IsSupported` pattern — no library or `/boot/firmware/config.txt` parsing needed).

**Phase 2 — OLED base.** Renders `mix`/`performance`/`ips`/`disk` pages. Implements the page state machine — advance/previous, sleep-timeout blanking, per-page content scroll (see `CONTEXT.md`) — as an API (`oled.Advance()`, `oled.Previous()`) the daemon can call directly. **Not wired to the physical button yet** — that's Phase 4, which depends on this phase's API. Acceptance testing here uses direct API calls / unit tests against a faked SSD1306, not the physical button.
*Done when*: all four pages render correct content on-device; `oled.Advance()`/`Previous()` switch pages correctly; the display blanks after the configured sleep timeout with no input; multi-value pages (`ips`, `disk`) scroll their sub-content on the configured interval. SSD1306 RP1 compatibility confirmed on-device (flagged risk, section 8).

`status` gains OLED page/sleep-state fields. `doctor` gains an I2C-enabled check (`/dev/i2c-1` existence), plus a separate convenience check for whether `i2c-tools` (`i2cdetect`) is installed — not something pironman needs at runtime (it talks to the bus directly via `periph.io`), just useful for a human debugging further.

**Phase 3 — Case-fan base.** On/off, plus `auto` at the fixed 67.5°C threshold (section 4). Surfaces the PWM fan's (Pi 5's own cooler) read-only state/speed via `status`.
*Done when*: `pironman fan on|off` toggles the relay; `pironman fan auto` turns the relay on above 67.5°C and off below 62.5°C (confirmed against the `pm_auto` source and verified on-device); `pironman status` shows both the case fan's state and the PWM fan's read-only speed/state.

**Phase 4 — Power-button base.** Press-event classification via evdev (click / double-click / long-press / long-press-released — see `CONTEXT.md`), wired to Phase 2's OLED API and to shutdown. No reboot action — the original hardware doesn't have one in its default map.
*Done when*: on physical hardware, a short press advances the OLED (waking it if asleep); a double-click goes to the previous page; a long press shows a shutdown-confirmation screen; releasing after a long press shuts the Pi down.

**v2+ (unchanged from the brief, refined where this plan's findings apply):**
- Phase 5 — RGB effects (breathing/flow/flow_reverse/rainbow/rainbow_reverse/hue_cycle, `--speed`).
- Phase 6 — Case-fan modes: `always_on`/`performance`/`cool`/`balanced`/`quiet`, at ≈50/60/67.5/75°C (correcting the brief's guessed 70°C for `quiet` to the original's actual ≈75°C), each with hysteresis matching the original's level logic. Adds `case_fan_mode` and a custom-threshold field to the config schema.
- Phase 7 — OLED custom text/image, rotation, sleep-timeout as a configurable value (v1 ships it as a fixed default; v2 exposes it in the CLI/config).

## 7. Testing / validation strategy

No CI hardware access, so every hardware boundary in `internal/hardware/*` is a small interface (`Relay`, `PWMFanReader`, `WS2812Strip`, `SSD1306Display`, `PowerButtonWatcher`, `Shutdowner`). `rgb`/`oled`/`fan`/`powerbutton`/`cli` are unit-tested entirely against hand-rolled fakes of these interfaces — no mocking library, since each interface is a handful of methods and hand-writing a fake is less code than adopting and learning a generator. The IPC protocol (`internal/ipc`) is tested with a real Unix socket in a temp directory (or `net.Pipe` for the framing logic alone) — no real daemon process needed.

Manual on-device verification checklist, one per phase, run against the actual Pi 5 + case:
- **Phase 0**: install/start/stop/uninstall the systemd unit; confirm socket permissions (`root:pironman`, `0660`) and that a non-root user in the `pironman` group can run `doctor`.
- **Phase 1**: visually confirm RGB color/brightness changes; confirm state survives `systemctl restart pironman`.
- **Phase 2**: visually confirm all four pages, sleep-timeout blanking, and content scroll; explicitly confirm the SSD1306 driver works over I2C on this specific Pi 5 (RP1 compatibility risk, section 8).
- **Phase 3**: confirm relay audibly/visibly switches at the expected temperature in `auto` mode; confirm `status` reports both fans correctly.
- **Phase 4**: physically test all four press events end-to-end, including the shutdown path.

## 8. Open questions, assumptions, and risks

- **Fan hysteresis band is unspecified.** The original's level-based comparison (`level >= mode`) likely has separate on/off boundaries per band to avoid relay chatter, but the exact band wasn't extracted from the source. Assumption: derive a reasonable hysteresis (e.g. ±2–3°C) during Phase 3 implementation and confirm on-device; not blocking the plan, but flagged so it isn't silently guessed at implementation time.
- **SSD1306 Pi 5/RP1 compatibility is unconfirmed by documentation.** Both reference implementations use it successfully via `periph.io`'s I2C layer, which is itself RP1-safe, but no source explicitly confirms the SSD1306 driver code path on a Pi 5. Verify on-device in Phase 2 (section 6/7).
- **Reference material path correction**: `PROMPT.md` names `~/dev/dnitros/original-pironman5/`, which doesn't exist. The actual local repos are `~/dev/dnitros/pironman5` (CLI/daemon wrapper) and `~/dev/dnitros/pm_auto` (hardware library) — worth fixing in the task doc for future reference.
- **PWM-fan control fallback**: `pm_auto` has a fallback path where it writes to the PWM fan on non-stock OSes where the kernel doesn't already govern it. This plan assumes stock Raspberry Pi OS, where the PWM fan is always read-only from this tool's side, per the fixed architecture constraint. If that assumption is wrong for the target install, the PWM-fan telemetry design in section 5/6 needs revisiting.
- **Config schema is a first draft.** Field names/shape will likely shift once Phase 0 scaffolding is underway; not treated as a compatibility-locked contract yet.
