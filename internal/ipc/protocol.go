// Package ipc implements the newline-delimited JSON request/response
// protocol the CLI and daemon speak over the control socket (ADR-0001).
package ipc

import (
	"os"
	"time"
)

// DefaultSocketPath is used when PIRONMAN_SOCKET_PATH is unset.
const DefaultSocketPath = "/run/pironman/pironman.sock"

// SocketPathEnvVar overrides the control socket path.
const SocketPathEnvVar = "PIRONMAN_SOCKET_PATH"

// ioTimeout bounds how long either side of one request/response exchange
// waits on the connection, so a stuck peer fails fast instead of hanging.
const ioTimeout = 5 * time.Second

// SocketPath resolves the control socket path the client and daemon both
// use, so the override lives in one place.
func SocketPath() string {
	if p := os.Getenv(SocketPathEnvVar); p != "" {
		return p
	}
	return DefaultSocketPath
}

// Request is one line sent from the CLI to the daemon.
type Request struct {
	Cmd  string         `json:"cmd"`
	Args map[string]any `json:"args,omitempty"`
}

// Response is one line sent back from the daemon to the CLI.
type Response struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// Handler executes one dispatched command and returns its response data.
type Handler func(args map[string]any) (any, error)
