package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"
)

// Server accepts one connection per request (per ADR-0001) and dispatches
// each decoded command to the matching registered Handler.
type Server struct {
	ln       net.Listener
	handlers map[string]Handler
}

// NewServer registers the given command handlers.
func NewServer(handlers map[string]Handler) *Server {
	return &Server{handlers: handlers}
}

// Listen opens the Unix socket at path, creating its parent directory and
// clearing a stale socket left by a previous run.
func (s *Server) Listen(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create socket dir: %w", err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", path, err)
	}
	s.ln = ln

	if err := restrictSocketAccess(path); err != nil {
		return fmt.Errorf("restrict socket access: %w", err)
	}
	return nil
}

// restrictSocketAccess applies ADR-0002's root:pironman 0660 socket
// permissions. Chown is best-effort: the pironman group is created by
// `daemon install`, so it won't exist yet on a dev machine or before that
// step has run, and that's not fatal here.
func restrictSocketAccess(path string) error {
	if err := os.Chmod(path, 0o660); err != nil {
		return err
	}

	grp, err := user.LookupGroup("pironman")
	if err != nil {
		return nil
	}
	gid, err := strconv.Atoi(grp.Gid)
	if err != nil {
		return nil
	}
	if err := os.Chown(path, -1, gid); err != nil {
		log.Printf("ipc: chown socket to pironman group: %v", err)
	}
	return nil
}

// Serve accepts connections until ctx is canceled or the listener is closed.
func (s *Server) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.ln.Close()
	}()

	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}
		go s.handleConn(conn)
	}
}

// Close stops accepting new connections.
func (s *Server) Close() error {
	return s.ln.Close()
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	// Bounds how long an accepted-but-silent connection can hold a
	// goroutine open, mirroring the client's own deadline.
	if err := conn.SetReadDeadline(time.Now().Add(ioTimeout)); err != nil {
		return
	}

	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return
	}

	resp := s.dispatch(line)

	encoded, err := json.Marshal(resp)
	if err != nil {
		return
	}
	encoded = append(encoded, '\n')
	if _, err := conn.Write(encoded); err != nil {
		log.Printf("ipc: write response: %v", err)
	}
}

func (s *Server) dispatch(line []byte) Response {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return Response{OK: false, Error: fmt.Sprintf("malformed request: %v", err)}
	}

	handler, ok := s.handlers[req.Cmd]
	if !ok {
		return Response{OK: false, Error: fmt.Sprintf("unknown command: %q", req.Cmd)}
	}

	data, err := handler(req.Args)
	if err != nil {
		return Response{OK: false, Error: err.Error()}
	}
	return Response{OK: true, Data: data}
}
