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
PIRONMAN_SOCKET_PATH=/tmp/pironman.sock ./pironman ping
```

Run tests with:

```sh
go test ./...
```

## Deployment (on a Raspberry Pi)

`daemon install` embeds the currently running binary's own path into the systemd unit's `ExecStart`, so put the binary at its final location before installing:

```sh
go build -o pironman ./cmd/pironman
sudo cp pironman /usr/local/bin/pironman
```

Then install and start the service:

```sh
sudo pironman daemon install
sudo pironman daemon start
```

`daemon install` creates the `pironman` group (see [ADR-0002](docs/adr/0002-socket-group-permissions.md)) and prints the command to add your user to it — run that, then start a new login session before using the CLI without `sudo`:

```sh
sudo usermod -aG pironman <your-username>
```

Manage the service afterward with `daemon stop`, `daemon start`, or `daemon uninstall` — `install`/`uninstall` require `sudo`, `start`/`stop` only need `pironman` group membership.

Config lives at `/etc/pironman/config.yaml` by default (override with `PIRONMAN_CONFIG_PATH`).
