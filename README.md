# Pironman

A CLI + daemon for controlling a Pironman 5 case (base edition) on a Raspberry Pi 5.

## Development

Requires Go 1.27.1+.

```sh
go build -o pironman ./cmd/pironman
```

The CLI and daemon talk over a Unix domain socket (default `/run/pironman/pironman.sock`, override with `PIRONMAN_SOCKET_PATH`). `/run` needs root, so for local testing:

```sh
PIRONMAN_SOCKET_PATH=/tmp/pironman.sock ./pironman doctor
```

`daemon run` opens the Pironman 5's WS2812 RGB strip over SPI, SSD1306 OLED over I2C, and case-fan relay over a GPIO character device at startup, so it only runs on the actual case hardware with those interfaces available — it can't be smoke-tested standalone on a dev machine anymore. `internal/hardware`, `internal/rgb`, `internal/oled`, and `internal/fan` are unit-tested against hand-rolled fakes instead (see `go test` below).

Run tests with:

```sh
go test ./...
```

`./pironman version` prints the build's git short SHA (with a `+dirty` suffix if the tree had uncommitted changes at build time), embedded automatically by `go build` via `runtime/debug.ReadBuildInfo()`. Prints `unknown` if built without VCS info (e.g. outside a git repo, or with `-buildvcs=false`).

## Deployment (on a Raspberry Pi)

`daemon install` embeds the currently running binary's own path into the systemd unit's `ExecStart`, so put the binary at its final location before installing:

```sh
go build -o pironman ./cmd/pironman
sudo cp pironman /usr/local/bin/pironman
```

Then install, enable, and start the service:

```sh
sudo pironman daemon install
sudo pironman daemon enable  # survive a reboot
sudo pironman daemon start   # run it now
```

`daemon install` never implicitly enables or starts the service — `enable` (boot-time autostart) and `start` (run now) are separate, explicit steps.

`daemon install` creates the `pironman` group and adds the invoking user (`$SUDO_USER`) to it automatically (see [ADR-0002](docs/adr/0002-socket-group-permissions.md)) — start a new login session before using socket-based commands like `doctor` without `sudo` (this doesn't apply to the `daemon` subcommands below, which always need `sudo` regardless of group membership). If `$SUDO_USER` isn't set, or the automatic add fails, it prints the `usermod` command to run manually instead:

```sh
sudo usermod -aG pironman <your-username>
```

Manage the service afterward with `daemon stop`, `daemon start`, `daemon enable`, `daemon disable`, or `daemon uninstall` — all six `daemon` subcommands enforce a `sudo` requirement directly in the CLI. This matches `systemctl`'s own default polkit policy, which requires admin authentication to manage a unit regardless of group membership — so the `pironman` group (which only governs the control-socket permissions, see [ADR-0002](docs/adr/0002-socket-group-permissions.md)) was never going to grant passwordless access to `start`/`stop`/`enable`/`disable`, and pironman now requires `sudo` outright rather than relying on that external policy. `daemon uninstall` also disables the service as part of cleanup.

Stopping the daemon — via `daemon stop`, a system shutdown/reboot, or `systemctl restart pironman` — turns off the RGB strip and de-energizes the case-fan relay, since their power stays live independently of the Pi's own running state. A restart therefore flashes the strip off then on (and drops the fan relay briefly) rather than leaving them running unmanaged, since the daemon can't distinguish a restart from a shutdown from a bare `SIGTERM`. The persisted RGB/fan config is untouched, so both return to their last configured state the next time the daemon starts (see [ADR-0003](docs/adr/0003-daemon-shutdown-hooks-and-connection-draining.md)).

All `daemon` subcommands (and `doctor`) detect whether `systemctl` is on `$PATH` first. On a non-systemd machine (e.g. macOS, or a systemd-less Linux distro), `daemon install`/`uninstall`/`start`/`stop`/`enable`/`disable` fail immediately with a clear "unsupported platform" message instead of a raw exec error, and `doctor` reports `platform: unsupported` instead of hard-erroring.

Config lives at `/etc/pironman/config.yaml` by default (override with `PIRONMAN_CONFIG_PATH`).

The daemon drives the onboard WS2812 RGB strip over SPI and the SSD1306 OLED over I2C, so both must be enabled first (`sudo raspi-config` → Interface Options → SPI and I2C, or `dtparam=spi=on`/`dtparam=i2c_arm=on` in `/boot/firmware/config.txt`) — otherwise `daemon start` fails to open `/dev/spidev0.0` or `/dev/i2c-1`. The case-fan relay uses the Linux GPIO character-device ABI (`/dev/gpiochip*`), which needs no equivalent enable step; it resolves the relay's line by its kernel-assigned name (`GPIO6`) rather than a fixed chip number, since which `/dev/gpiochipN` it lands on varies across kernels/OS images.
