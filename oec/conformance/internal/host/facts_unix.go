//go:build !windows

package host

import (
	"bufio"
	"os"
	"strings"
)

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// osRelease is PRETTY_NAME (else NAME VERSION_ID) of an os-release file.
func osRelease(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	kv := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if ok {
			kv[k] = strings.Trim(v, `"'`)
		}
	}
	if kv["PRETTY_NAME"] != "" {
		return kv["PRETTY_NAME"]
	}
	return strings.TrimSpace(kv["NAME"] + " " + kv["VERSION_ID"])
}
