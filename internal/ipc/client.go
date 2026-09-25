package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Send opens one connection to the daemon at path and exchanges one
// request/response pair (per ADR-0001's one-shot connection lifecycle).
func Send(path, cmd string, args map[string]any) (*Response, error) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", path, err)
	}
	defer conn.Close()

	return sendOnConn(conn, cmd, args)
}

func sendOnConn(conn net.Conn, cmd string, args map[string]any) (*Response, error) {
	// Bounds both the write and the read below, so a daemon that's alive
	// but not draining connections can't hang the CLI either way.
	if err := conn.SetDeadline(time.Now().Add(ioTimeout)); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	reqLine, err := json.Marshal(Request{Cmd: cmd, Args: args})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	reqLine = append(reqLine, '\n')
	if _, err := conn.Write(reqLine); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	respLine, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var resp Response
	if err := json.Unmarshal(respLine, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &resp, nil
}
