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

type Server struct {
	ln       net.Listener
	handlers map[string]Handler
}

func NewServer(handlers map[string]Handler) *Server {
	return &Server{handlers: handlers}
}

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

func listenRestricted(path string) (net.Listener, error) {
	old := syscall.Umask(0o117)
	defer syscall.Umask(old)
	return net.Listen("unix", path)
}

func restrictSocketAccess(path string) error {
	grp, err := user.LookupGroup(GroupName)
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

func (s *Server) Close() error {
	return s.ln.Close()
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

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
