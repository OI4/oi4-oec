package engine

import "testing"

// TestParseUnix: the path of a unix:// endpoint is taken as it is, so a
// Windows path keeps its drive letter (found in the first hosted Windows
// runs, C-104).
func TestParseUnix(t *testing.T) {
	for in, want := range map[string]string{
		"unix:///var/run/docker.sock":    "/var/run/docker.sock",
		"/run/user/1000/docker.sock":     "/run/user/1000/docker.sock",
		`unix://C:\Users\me\docker.sock`: `C:\Users\me\docker.sock`,
		"unix://C:/Users/me/docker.sock": "C:/Users/me/docker.sock",
		`C:\Users\me\docker.sock`:        `C:\Users\me\docker.sock`,
	} {
		got, err := parse(in)
		if err != nil || got.network != "unix" || got.addr != want {
			t.Errorf("%s: %+v %v, want %s", in, got, err, want)
		}
	}
}
