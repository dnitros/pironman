package ipc

import (
	"os"
	"time"
)

const DefaultSocketPath = "/run/pironman/pironman.sock"

const SocketPathEnvVar = "PIRONMAN_SOCKET_PATH"

const ioTimeout = 5 * time.Second

func SocketPath() string {
	if p := os.Getenv(SocketPathEnvVar); p != "" {
		return p
	}
	return DefaultSocketPath
}

type Request struct {
	Cmd  string         `json:"cmd"`
	Args map[string]any `json:"args,omitempty"`
}

type Response struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

type Handler func(args map[string]any) (any, error)
