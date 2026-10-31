//go:build !windows

package engine

import (
	"os"
	"path/filepath"
	"runtime"
)

// Defaults returns the default endpoints of Docker and Podman on this
// system, in order (A-266): Linux /var/run/docker.sock, /run/docker.sock,
// /run/podman/podman.sock and the rootless sockets below XDG_RUNTIME_DIR;
// macOS /var/run/docker.sock and ~/.docker/run/docker.sock.
func Defaults() []string {
	out := []string{"unix:///var/run/docker.sock"}
	switch runtime.GOOS {
	case "darwin":
		if h, err := os.UserHomeDir(); err == nil {
			out = append(out, "unix://"+filepath.Join(h, ".docker", "run", "docker.sock"))
		}
	default:
		out = append(out, "unix:///run/docker.sock", "unix:///run/podman/podman.sock")
		if x := os.Getenv("XDG_RUNTIME_DIR"); x != "" {
			out = append(out, "unix://"+filepath.Join(x, "podman", "podman.sock"), "unix://"+filepath.Join(x, "docker.sock"))
		}
	}
	return out
}
