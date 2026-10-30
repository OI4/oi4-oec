//go:build !linux

package host

// mountTable: only Linux has a mount table D8 reads; elsewhere the mounts
// of the storages are reported as unknown.
func mountTable() ([]Mount, error) { return nil, ErrNoMountTable }
