# Pironman

A CLI + daemon for controlling a Pironman 5 case (base edition) on a Raspberry Pi 5.

## Development

Requires Go 1.27.1+.

```sh
go build -o pironman ./cmd/pironman
```

The CLI and daemon talk over a Unix domain socket (default `/run/pironman/pironman.sock`, override with `PIRONMAN_SOCKET_PATH`). `/run` needs root, so for local testing:

```sh
PIRONMAN_SOCKET_PATH=/tmp/pironman.sock ./pironman daemon run &
PIRONMAN_SOCKET_PATH=/tmp/pironman.sock ./pironman doctor
```

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

`daemon install` creates the `pironman` group and adds the invoking user (`$SUDO_USER`) to it automatically (see [ADR-0002](docs/adr/0002-socket-group-permissions.md)) — start a new login session before using the CLI without `sudo`. If `$SUDO_USER` isn't set, or the automatic add fails, it prints the `usermod` command to run manually instead:

```sh
sudo usermod -aG pironman <your-username>
```

Manage the service afterward with `daemon stop`, `daemon start`, `daemon enable`, `daemon disable`, or `daemon uninstall`. `install`/`uninstall` always require `sudo`. `start`/`stop`/`enable`/`disable` don't enforce a root check in code, but may still require `sudo` in practice depending on your system's polkit policy for `systemctl`. `daemon uninstall` also disables the service as part of cleanup.

All `daemon` subcommands (and `doctor`) detect whether `systemctl` is on `$PATH` first. On a non-systemd machine (e.g. macOS, or a systemd-less Linux distro), `daemon install`/`uninstall`/`start`/`stop`/`enable`/`disable` fail immediately with a clear "unsupported platform" message instead of a raw exec error, and `doctor` reports `platform: unsupported` instead of hard-erroring.

Config lives at `/etc/pironman/config.yaml` by default (override with `PIRONMAN_CONFIG_PATH`).
