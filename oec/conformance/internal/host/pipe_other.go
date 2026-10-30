//go:build !windows

package host

import (
	"fmt"
	"os"
)

func openPipe(name string) (*os.File, error) {
	return nil, fmt.Errorf("%w: named pipes exist only on Windows (%s)", errUnsupported, name)
}
