package host

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/OI4/oec/conformance/internal/report"
)

// maxFile limits what D8 reads of a storage file.
const maxFile = 1 << 20

// store is one container storage as D8 found it.
type store struct {
	dir     string
	exists  bool
	err     error // the directory could not be read (e.g. permission)
	entries map[string]fs.FileInfo
	rep     report.Storage
}

// scan lists a storage (one level, names, modes and sizes).
func (r *runner) scan(name, dir string) *store {
	s := &store{dir: dir, entries: map[string]fs.FileInfo{}, rep: report.Storage{Name: name, Path: dir, Mount: "unknown"}}
	st, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return s
	case err != nil:
		s.err = err
		return s
	case !st.IsDir():
		s.err = errors.New("not a directory")
		return s
	}
	s.exists, s.rep.Exists = true, true
	des, err := os.ReadDir(dir)
	if err != nil {
		s.err = err
		return s
	}
	for _, de := range des {
		fi, err := os.Stat(filepath.Join(dir, de.Name()))
		if err != nil {
			fi, err = de.Info()
			if err != nil {
				continue
			}
		}
		s.entries[de.Name()] = fi
		f := report.File{Name: de.Name(), Size: fi.Size()}
		if runtime.GOOS != "windows" {
			f.Mode = fi.Mode().String()
		}
		s.rep.Files = append(s.rep.Files, f)
	}
	sort.Slice(s.rep.Files, func(i, j int) bool { return s.rep.Files[i].Name < s.rep.Files[j].Name })
	s.rep.Mount = r.mountOf(s)
	return s
}

func (s *store) path(name string) string { return filepath.Join(s.dir, name) }

// has reports whether the storage has a regular file of the name.
func (s *store) has(name string) bool {
	fi, ok := s.entries[name]
	return ok && fi.Mode().IsRegular()
}

// read returns a file of the storage; ok is false if it does not exist.
func (s *store) read(name string) (data []byte, ok bool, err error) {
	if !s.has(name) {
		return nil, false, nil
	}
	f, err := os.Open(s.path(name))
	if err != nil {
		return nil, true, err
	}
	defer f.Close()
	data, err = io.ReadAll(io.LimitReader(f, maxFile+1))
	if err != nil {
		return nil, true, err
	}
	if len(data) > maxFile {
		return nil, true, errors.New("larger than 1 MiB")
	}
	return data, true, nil
}

// Mount is one entry of the mount table.
type Mount struct {
	Point    string
	ReadOnly bool
}

// ErrNoMountTable: the system has no mount table D8 reads (not Linux).
var ErrNoMountTable = errors.New("no mount table on this system")

// within reports whether p is dir or below it.
func within(p, dir string) bool {
	if p == dir {
		return true
	}
	if !strings.HasSuffix(dir, string(filepath.Separator)) {
		dir += string(filepath.Separator)
	}
	return strings.HasPrefix(p, dir)
}

// mountOf classifies the mount of a storage: ro or rw if the storage or a
// file in it is a mount point (Docker mounts secrets per file) or a
// read-only mount covers it; none if it has no mount of its own; unknown
// without a mount table.
func (r *runner) mountOf(s *store) string {
	if !r.mountsKnown {
		return "unknown"
	}
	dir, err := filepath.Abs(s.dir)
	if err != nil {
		return "unknown"
	}
	own, rw := false, false
	var cover *Mount
	for i, m := range r.mounts {
		switch {
		case m.Point == dir || filepath.Dir(m.Point) == dir && s.entries[filepath.Base(m.Point)] != nil:
			own = true
			rw = rw || !m.ReadOnly
		case within(dir, m.Point):
			if cover == nil || len(m.Point) >= len(cover.Point) {
				cover = &r.mounts[i]
			}
		}
	}
	switch {
	case own && rw:
		return "rw"
	case own:
		return "ro"
	case cover != nil && cover.ReadOnly:
		return "ro"
	}
	return "none"
}

// judgeMount judges the read-only mount of a storage (RIG 4.1). Without a
// mount of its own the storage is judged only if D8 runs in a container:
// then the platform did not map a host path to it.
func (r *runner) judgeMount(test, req string, s *store) {
	switch s.rep.Mount {
	case "ro":
		r.checked(test, req, s.dir, "%s is mounted read-only", s.dir)
	case "rw":
		r.violation(test, req, s.dir, "%s (or a file in it) is mounted read-write; the platform mounts the storage read-only (RIG 4.1)", s.dir)
	case "none":
		if r.facts.InContainer {
			r.violation(test, req, s.dir, "%s is no mount point although D8 runs in a container: the platform maps a host-defined path to it, mounted read-only (RIG 4.1)", s.dir)
		}
	}
}

// worldWritable notes entries of a storage that every user may write: a
// read-only mount protects them, a plain directory does not.
func (r *runner) worldWritable(test, req string, s *store) {
	if runtime.GOOS == "windows" {
		return
	}
	if st, err := os.Stat(s.dir); err == nil && st.Mode().Perm()&0o002 != 0 {
		r.note(test, req, s.dir, "%s is writable for every user (mode %s)", s.dir, st.Mode())
	}
	names := make([]string, 0, len(s.entries))
	for n := range s.entries {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if fi := s.entries[n]; fi.Mode().Perm()&0o002 != 0 {
			r.note(test, req, s.path(n), "%s is writable for every user (mode %s)", s.path(n), fi.Mode())
		}
	}
}

// unreadable is the detail of a storage or file D8 cannot read.
func unreadable(path string, err error) string {
	if errors.Is(err, fs.ErrPermission) {
		return path + ": permission denied; run D8 as a user that may read the storages (e.g. as the user of the application or as root)"
	}
	return path + ": " + err.Error()
}
