# Task

Produce a phased **implementation plan** for a CLI + daemon tool (working name:
`pironman`) that controls a Pironman 5 case (base edition — not mini/pro/max)
on a Raspberry Pi 5 (8GB). This is a planning pass only — do not write Go
code in this session. The output should be a markdown design document I will
review before any implementation starts.

# Hardware / environment context

- Case: Pironman 5, base edition.
- Host: Raspberry Pi 5, 8GB.
- OS-level prerequisites already done: i2c-tools installed, I2C and SPI
  enabled in boot config, OLED confirmed detected on the I2C bus.
- Devices involved: `/dev/i2c-1` (OLED, and likely fan-state read),
  `/dev/spidev0.0` (likely WS2812 RGB), a GPIO line for the fan on/off relay,
  and the power button, wired as a standard Linux input device (evdev, under
  `/dev/input`) rather than a raw GPIO line.
- **Important**: Raspberry Pi 5 exposes GPIO through the RP1 southbridge
  chip (PCIe-attached), not the SoC's legacy memory-mapped GPIO used on
  earlier Pi models. Libraries built around the old `/dev/gpiomem`-style
  direct register access may not work correctly, or at all, on Pi 5. Before
  recommending any GPIO/I2C/SPI library, explicitly verify and document its
  RP1/Pi 5 compatibility status — don't just check "is it maintained."

# Reference material (for research, not for copying)

- Original Python implementation: `~/dev/dnitros/original-pironman5/`. Use
  this to understand actual hardware protocol details: pin mappings, I2C
  register usage, OLED page definitions, fan temperature thresholds, and
  power-button press semantics (what short/double/long press each do by
  default). Do not port its code or structure directly.
- Existing Go port: https://github.com/l-you/pironman5-go. Use this only to
  learn from its architectural choices — what to avoid or improve on. This
  project should not follow its design; note explicitly where and why this
  plan diverges from it.

# Fixed project identity

- Go module path: `github.com/dnitros/pironman`
- Repository: `github.com/dnitros/pironman`

# Fixed architecture constraints (non-negotiable — do not redesign these)

- Split into a daemon and a thin CLI, communicating over a Unix domain
  socket. Default path `/run/pironman/pironman.sock`, overridable via
  env var `PIRONMAN_SOCKET_PATH`.
- Daemon persists configuration at `/etc/pironman/config.yaml` by default,
  overridable via env var `PIRONMAN_CONFIG_PATH`.
- Daemon runs as root, installed as a systemd unit `pironman.service`
  (root is required for `/dev/i2c-1`, `/dev/spidev0.0`, GPIO, and exclusive
  access to the power-button input device).
- The CLI must never require sudo for normal operation — only the daemon
  touches hardware; the CLI only talks to the socket. The exception is
  `pironman daemon install`/`uninstall`, which do need elevated privileges
  since they write the unit file and manage the systemd service.
- `pironman daemon start`/`stop` only control an **already-installed**
  service (`systemctl start|stop pironman.service`). They must not implicitly
  install the unit. If the unit isn't installed, they should fail with a
  clear error telling the user to run `pironman daemon install` first —
  installing and starting are deliberately separate steps.
- The daemon owns all hardware state in a single process: the RGB effect
  loop, the OLED page-cycle loop, the GPIO-fan temperature loop, and the
  power-button event watcher, each running as a goroutine.
- The PWM fan is **read-only** from this tool's side — no control logic,
  since the Pi's own governor handles it. The Python original reads its
  state via the `pm_auto` library; investigate what that actually reads
  (I2C register vs. tach GPIO) and replicate only the read, not any control.

# Target CLI surface (a guide, not a contract — refine names/flags if it
improves the design, but keep the same functional coverage)

pironman daemon install / uninstall / start / stop
pironman doctor / status

pironman rgb on / off
pironman rgb color <#hex>
pironman rgb style <name> [--speed N]   # v2, see phases below
pironman rgb brightness <0-100>

pironman oled on / off
pironman oled page <mix|performance|ips|disk|next|prev>
pironman oled text "<message>"          # v2
pironman oled resume                    # v2
pironman oled rotation <0|180>          # v2
pironman oled sleep-timeout <seconds>   # v2

pironman fan on / off / auto
pironman fan mode <mode>                # v2

# Scope: v1 must cover every feature's base functionality

v1 is not "just RGB" or "just one feature end to end" — it must ship the
**base** functionality of every feature area (RGB, OLED, GPIO fan, power
button) together. Only the *advanced* extensions per feature (effects,
modes, custom content) are deferred to v2+. Build incrementally within v1,
but all of v1's phases together are the first release; nothing gets held
back to v2 that's listed as "base" below.

# Delivery phases (build the plan's structure around these — propose
adjustments if you have a good reason, but keep them explicit and ordered
with acceptance criteria per phase, not an open-ended "iterative" build)

**Phase 0 — Scaffolding**: repo/module layout under
`github.com/dnitros/pironman`, socket IPC skeleton (a no-op ping/pong
command), daemon skeleton, `pironman daemon install`/`uninstall` (systemd
unit file management), `pironman daemon start`/`stop` wired to the
installed service with the not-installed error case handled, `doctor` and
`status` commands, config load/save skeleton respecting
`PIRONMAN_CONFIG_PATH`.

**v1 (all base features — required for first release):**
- Phase 1 — RGB base: on/off, color, brightness (solid only).
- Phase 2 — OLED base: live auto-cycling system stats pages
  (mix/performance/ips/disk) on a timer.
- Phase 3 — GPIO fan base: on/off only.
- Phase 4 — Power button base: short/double/long press handling. Confirm
  the original's default action mapping for each and state it as an
  assumption to be confirmed, rather than guessing.

**v2+ (later, after v1 ships):**
- Phase 5 — RGB effects: breathing/flow/flow_reverse/rainbow/
  rainbow_reverse/hue_cycle, with `--speed`.
- Phase 6 — GPIO fan modes: always_on/performance/cool/balanced/quiet,
  matching the original's ~50°C/60°C/67.5°C/70°C thresholds.
- Phase 7 — OLED custom text/image, rotation, sleep-timeout.

# What the plan document should contain

1. High-level architecture: component responsibilities, goroutines, data
   flow between CLI, socket, daemon, and hardware.
2. Proposed package/directory layout.
3. Socket IPC protocol design (message framing/format) — propose and justify
   a choice; this is not dictated above.
4. Draft config schema (YAML).
5. Library selection for GPIO/I2C/SPI/WS2812/SSD1306, each with version,
   maintenance status, and explicit RP1/Pi 5 compatibility findings. If
   nothing production-ready exists for a given need, document the fallback
   (e.g. shelling out to or cgo-wrapping `libgpiod`) and its trade-offs.
6. The phase breakdown above, refined if needed, each with concrete
   "done" acceptance criteria, and the v1/v2+ boundary preserved.
7. Testing/validation strategy given no CI hardware access — e.g. hardware
   access behind interfaces so most logic is unit-testable without real
   devices, plus a manual on-device verification checklist per phase.
8. Open questions, assumptions, and risks that need my confirmation before
   implementation starts.

# Instructions

- Inspect the reference repos as needed for hardware behavior and pin/protocol
  details, but do not copy their code or directory structure.
- If something is genuinely blocking, ask me directly rather than guessing.
  Otherwise state the assumption explicitly in the "open questions" section
  and proceed.
- Do not write Go code in this pass — output only the plan document.
