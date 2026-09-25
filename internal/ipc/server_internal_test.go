package ipc

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// fakeConn is a hand-rolled net.Conn double (no real socket), so this test
// can verify handleConn's deadline handling deterministically and fast
// instead of depending on real, OS-dependent socket buffer exhaustion.
type fakeConn struct {
	readBuf bytes.Reader

	readDeadlineSet  bool
	writeDeadlineSet bool
}

func newFakeConn(request string) *fakeConn {
	f := &fakeConn{}
	f.readBuf = *bytes.NewReader([]byte(request))
	return f
}

func (f *fakeConn) Read(b []byte) (int, error)  { return f.readBuf.Read(b) }
func (f *fakeConn) Write(b []byte) (int, error) { return len(b), nil }
func (f *fakeConn) Close() error                { return nil }
func (f *fakeConn) LocalAddr() net.Addr         { return fakeAddr{} }
func (f *fakeConn) RemoteAddr() net.Addr        { return fakeAddr{} }

func (f *fakeConn) SetReadDeadline(time.Time) error {
	f.readDeadlineSet = true
	return nil
}

func (f *fakeConn) SetWriteDeadline(time.Time) error {
	f.writeDeadlineSet = true
	return nil
}

func (f *fakeConn) SetDeadline(t time.Time) error {
	_ = f.SetReadDeadline(t)
	return f.SetWriteDeadline(t)
}

type fakeAddr struct{}

func (fakeAddr) Network() string { return "fake" }
func (fakeAddr) String() string  { return "fake" }

func TestHandleConnSetsWriteDeadline(t *testing.T) {
	srv := NewServer(map[string]Handler{
		"ping": func(args map[string]any) (any, error) {
			return map[string]string{"message": "pong"}, nil
		},
	})

	conn := newFakeConn(`{"cmd":"ping"}` + "\n")
	srv.handleConn(conn)

	if !conn.writeDeadlineSet {
		t.Fatalf("expected handleConn to arm a write deadline before writing the response, so a stalled peer can't block the goroutine forever")
	}
}
