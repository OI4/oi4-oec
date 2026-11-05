// Command pack writes a directory as a reproducible archive (release.sh,
// ADR-0033): a .tar.gz or a .zip whose bytes depend only on the names and
// contents of the files and on the fixed time, not on the host, the file
// system, the owner, the umask or the time of the run.
//
//	pack -mtime <unix seconds> -prefix <name>/ -o <archive> <dir>
//
// The entries are the directories and regular files below <dir> in byte
// order of their slash paths, each under <prefix>; directories and files
// with an execute bit get 0755, other files 0644; owner 0, no user or group
// names. A symbolic link or another special file is an error. The gzip
// header carries no name and no time. Standard library only.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func main() {
	out := flag.String("o", "", "the archive to write (.tar.gz or .zip)")
	prefix := flag.String("prefix", "", "the top directory of every entry, ending in /")
	mtime := flag.Int64("mtime", 0, "the modification time of every entry (Unix seconds, e.g. SOURCE_DATE_EPOCH)")
	flag.Parse()
	if *out == "" || flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: pack -mtime <seconds> -prefix <name>/ -o <archive.tar.gz|archive.zip> <dir>")
		os.Exit(2)
	}
	if err := pack(*out, flag.Arg(0), *prefix, *mtime); err != nil {
		fmt.Fprintln(os.Stderr, "pack:", err)
		os.Exit(1)
	}
}

type entry struct {
	name string // slash path below the prefix, directories end in /
	file string // path on disk
	mode int64
}

// pack writes dir to out; on error, out does not exist.
func pack(out, dir, prefix string, mtime int64) (err error) {
	if !strings.HasSuffix(prefix, "/") || strings.HasPrefix(prefix, "/") || !fs.ValidPath(strings.TrimSuffix(prefix, "/")) {
		return fmt.Errorf("prefix %q: a relative path ending in /", prefix)
	}
	var write func(io.Writer, []entry, time.Time) error
	switch {
	case strings.HasSuffix(out, ".tar.gz"):
		write = writeTarGz
	case strings.HasSuffix(out, ".zip"):
		write = writeZip
	default:
		return fmt.Errorf("%s: not .tar.gz or .zip", out)
	}
	entries := []entry{{name: prefix, mode: 0o755}}
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil || rel == "." {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		name := prefix + filepath.ToSlash(rel)
		switch {
		case info.IsDir():
			entries = append(entries, entry{name: name + "/", mode: 0o755})
		case info.Mode().IsRegular():
			mode := int64(0o644)
			if info.Mode().Perm()&0o111 != 0 {
				mode = 0o755
			}
			entries = append(entries, entry{name: name, file: p, mode: mode})
		default:
			return fmt.Errorf("%s: not a directory or regular file (%v)", p, info.Mode().Type())
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(out)
		}
	}()
	return write(f, entries, time.Unix(mtime, 0).UTC())
}

func writeTarGz(w io.Writer, entries []entry, mtime time.Time) error {
	zw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(zw)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: e.mode, ModTime: mtime, Format: tar.FormatPAX}
		if e.file == "" {
			h.Typeflag = tar.TypeDir
		} else {
			h.Typeflag = tar.TypeReg
			info, err := os.Stat(e.file)
			if err != nil {
				return err
			}
			h.Size = info.Size()
		}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if e.file != "" {
			if err := copyFile(tw, e.file); err != nil {
				return err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return zw.Close()
}

func writeZip(w io.Writer, entries []entry, mtime time.Time) error {
	zw := zip.NewWriter(w)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate, Modified: mtime}
		if e.file == "" {
			h.Method = zip.Store
			h.SetMode(fs.ModeDir | fs.FileMode(e.mode))
		} else {
			h.SetMode(fs.FileMode(e.mode))
		}
		fw, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		if e.file != "" {
			if err := copyFile(fw, e.file); err != nil {
				return err
			}
		}
	}
	return zw.Close()
}

func copyFile(w io.Writer, name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
