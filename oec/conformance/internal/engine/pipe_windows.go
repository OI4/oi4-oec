package engine

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"time"
)

// errPipeBusy is ERROR_PIPE_BUSY: every instance of the pipe is in use.
const errPipeBusy syscall.Errno = 231

// openPipe opens a named pipe for overlapped I/O; os.NewFile associates
// such a handle with the I/O completion port of the runtime (Go 1.25), so
// reads and writes support deadlines and do not block a thread.
func openPipe(ctx context.Context, name string) (net.Conn, error) {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	for {
		h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OVERLAPPED, 0)
		if err == nil {
			return pipeConn{os.NewFile(uintptr(h), name)}, nil
		}
		if !errors.Is(err, errPipeBusy) {
			return nil, &os.PathError{Op: "open", Path: name, Err: err}
		}
		select {
		case <-ctx.Done():
			return nil, &os.PathError{Op: "open", Path: name, Err: ctx.Err()}
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// pipeConn is a named pipe as net.Conn.
type pipeConn struct{ *os.File }

type pipeAddr string

func (a pipeAddr) Network() string { return "npipe" }
func (a pipeAddr) String() string  { return string(a) }

func (c pipeConn) LocalAddr() net.Addr  { return pipeAddr(c.Name()) }
func (c pipeConn) RemoteAddr() net.Addr { return pipeAddr(c.Name()) }
