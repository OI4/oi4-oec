//go:build !windows

package main

import (
	"context"
	osexec "os/exec"
)

// shell runs the command c with sh -c.
func shell(ctx context.Context, c string) *osexec.Cmd {
	return osexec.CommandContext(ctx, "sh", "-c", c)
}
