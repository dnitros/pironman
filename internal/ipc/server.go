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
	"syscall"
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
	if err := removeStaleSocket(path); err != nil {
		return err
	}

	ln, err := listenRestricted(path)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", path, err)
	}
	s.ln = ln

	if err := restrictSocketAccess(path); err != nil {
		return fmt.Errorf("restrict socket access: %w", err)
	}
	return nil
}

// removeStaleSocket clears a socket file left by a previous run, but
// refuses to touch one a live daemon is still listening on — otherwise a
// second `daemon run` would silently steal the first instance's socket.
func removeStaleSocket(path string) error {
	conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return fmt.Errorf("a daemon is already listening on %s", path)
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	return nil
}

// listenRestricted binds the socket under a restrictive umask, so ADR-0002's
// 0660 permissions apply from the socket's first instant. Safe because
// Listen runs once at daemon startup, before any other goroutine creates
// files; umask is process-wide.
func listenRestricted(path string) (net.Listener, error) {
	old := syscall.Umask(0o117) // 0777 (bind's default) &^ 0117 = 0660
	defer syscall.Umask(old)
	return net.Listen("unix", path)
}

// restrictSocketAccess applies ADR-0002's root:pironman group ownership; the
// 0660 mode is set by listenRestricted's umask. Chown is best-effort: the
// pironman group is created by `daemon install`, so it won't exist yet on a
// dev machine or before that step has run, and that's not fatal here.
func restrictSocketAccess(path string) error {
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

	// Bounds the whole request/response exchange, mirroring the client's
	// own deadline: an accepted-but-silent connection, or one that stops
	// draining its response, can't hold this goroutine open forever.
	if err := conn.SetDeadline(time.Now().Add(ioTimeout)); err != nil {
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
