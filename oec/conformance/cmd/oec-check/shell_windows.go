package main

import (
	"context"
	osexec "os/exec"
	"syscall"
)

// shell runs the command c with cmd /S /C "c". The command line is passed
// as it is: the quoting of Go (\") is unknown to cmd, so a quoted path in
// c would break.
func shell(ctx context.Context, c string) *osexec.Cmd {
	cmd := osexec.CommandContext(ctx, "cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd /S /C "` + c + `"`}
	return cmd
}
