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
