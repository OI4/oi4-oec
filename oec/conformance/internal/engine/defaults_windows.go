package engine

// Defaults returns the default endpoints of Docker and Podman on Windows,
// in order (A-266).
func Defaults() []string {
	return []string{"npipe:////./pipe/docker_engine", "npipe:////./pipe/podman-machine-default"}
}
