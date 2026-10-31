//go:build !windows

package engine

import (
	"context"
	"fmt"
	"net"
)

func openPipe(_ context.Context, name string) (net.Conn, error) {
	return nil, fmt.Errorf("%w: named pipes exist only on Windows (%s)", ErrUnsupported, name)
}
