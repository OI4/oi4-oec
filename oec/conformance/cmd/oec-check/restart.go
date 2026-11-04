package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// restartHook builds the restart hook of T7-LIFE-04 and T7-SET-10
// (DESIGN.md §3.4): --restart-cmd, or --stop-cmd followed by --start-cmd,
// each run by the shell; nil without these flags. The output of the
// commands goes to stderr. A command that starts the component under test
// in the background redirects its output, else D7 waits up to 10 s for
// the output to close.
func (f *monitorFlags) restartHook(stderr io.Writer) (func(context.Context) error, error) {
	var cmds []string
	switch {
	case f.restartCmd != "" && (f.stopCmd != "" || f.startCmd != ""):
		return nil, errors.New("--restart-cmd excludes --stop-cmd and --start-cmd")
	case f.restartCmd != "":
		cmds = []string{f.restartCmd}
	case f.stopCmd != "" && f.startCmd != "":
		cmds = []string{f.stopCmd, f.startCmd}
	case f.stopCmd != "" || f.startCmd != "":
		return nil, errors.New("--stop-cmd and --start-cmd come together")
	}
	if f.restartWait <= 0 {
		return nil, errors.New("--restart-wait must be positive")
	}
	if cmds == nil {
		return nil, nil
	}
	return func(ctx context.Context) error {
		for _, c := range cmds {
			cmd := shell(ctx, c)
			cmd.Stdout, cmd.Stderr = stderr, stderr
			cmd.WaitDelay = 10 * time.Second
			fmt.Fprintf(stderr, "%s: restart hook: %s\n", tool, c)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("%q: %w", c, err)
			}
		}
		return nil
	}, nil
}
