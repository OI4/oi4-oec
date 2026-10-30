package host

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseMountinfo parses /proc/<pid>/mountinfo (Linux, proc(5)): field 5 is
// the mount point (octal escapes), field 6 the options of the mount; the
// super block options follow the separator "-". A mount is read-only if
// either has "ro".
func ParseMountinfo(rd io.Reader) ([]Mount, error) {
	var out []Mount
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for n := 1; sc.Scan(); n++ {
		f := strings.Fields(sc.Text())
		if len(f) < 10 {
			return nil, fmt.Errorf("mountinfo line %d: %d fields", n, len(f))
		}
		sep := -1
		for i := 6; i < len(f); i++ {
			if f[i] == "-" {
				sep = i
				break
			}
		}
		if sep < 0 {
			return nil, fmt.Errorf("mountinfo line %d: no separator", n)
		}
		ro := hasOpt(f[5], "ro")
		if sep+3 < len(f) {
			ro = ro || hasOpt(f[sep+3], "ro")
		}
		out = append(out, Mount{Point: unescape(f[4]), ReadOnly: ro})
	}
	return out, sc.Err()
}

func hasOpt(opts, o string) bool {
	for _, x := range strings.Split(opts, ",") {
		if x == o {
			return true
		}
	}
	return false
}

// unescape resolves the octal escapes (\040 …) of mountinfo.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
