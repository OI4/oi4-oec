package feedback

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// diffFlags pin every option that changes the output of git diff, so that
// the package is the same with every git version and configuration.
var diffFlags = []string{
	"--no-color", "--no-ext-diff", "--no-textconv", "--no-relative",
	"--diff-algorithm=myers", "--indent-heuristic", "--inter-hunk-context=0",
	"-M50%", "--src-prefix=a/", "--dst-prefix=b/", "-O" + os.DevNull,
}

// Git runs git in the guideline repository without the user and system
// configuration.
type Git struct{ Dir string }

func (g Git) run(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", g.Dir, "-c", "core.quotepath=off"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "LC_ALL=C")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

var adrFileRe = regexp.MustCompile(`^rfcs_adrs/ADR-(0\d{2})_[^/]+\.md$`)

// Load reads the history of base..target for the documents.
func (g Git) Load(base, target string) (*History, error) {
	h := &History{}
	var err error
	for _, x := range []struct {
		rev        string
		hash, date *string
	}{{base, &h.Base, &h.BaseDate}, {target, &h.Target, &h.TargetDate}} {
		out, err := g.run("log", "-1", "--format=%H %ad", "--date=short", x.rev+"^{commit}", "--")
		if err != nil {
			return nil, err
		}
		f := strings.Fields(out)
		if len(f) != 2 {
			return nil, fmt.Errorf("cannot resolve %s", x.rev)
		}
		*x.hash, *x.date = f[0], f[1]
	}
	if _, err := g.run("merge-base", "--is-ancestor", h.Base, h.Target); err != nil {
		return nil, fmt.Errorf("base %.7s is not an ancestor of the target %.7s", h.Base, h.Target)
	}
	rng := h.Base + ".." + h.Target
	out, err := g.run(append([]string{"log", "--no-merges", "--reverse", "--format=%H %ad", "--date=short", rng, "--"}, Paths...)...)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		c := Commit{Hash: f[0], Date: f[1]}
		if c.Files, err = g.files(c.Hash); err != nil {
			return nil, err
		}
		h.Commits = append(h.Commits, c)
	}
	out, err = g.run("log", "--merges", "--format=%H", rng)
	if err != nil {
		return nil, err
	}
	for _, m := range strings.Fields(out) {
		cc, err := g.run(append([]string{"diff-tree", "--cc", "--no-color", "--format=", m, "--"}, Paths...)...)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(cc) != "" {
			h.MergesWithChanges = append(h.MergesWithChanges, m)
		}
	}
	out, err = g.run("ls-tree", "--name-only", h.Target, "rfcs_adrs/")
	if err != nil {
		return nil, err
	}
	for _, p := range strings.Split(strings.TrimSpace(out), "\n") {
		m := adrFileRe.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		text, err := g.run("show", h.Target+":"+p)
		if err != nil {
			return nil, err
		}
		h.ADRs = append(h.ADRs, ADR{ID: "GL-ADR-" + m[1], Path: p, Title: title(text)})
	}
	sort.Slice(h.ADRs, func(i, j int) bool { return h.ADRs[i].ID < h.ADRs[j].ID })
	out, err = g.run(append(append(append([]string{"diff", "--shortstat"}, diffFlags...), h.Base, h.Target, "--"), Paths...)...)
	if err != nil {
		return nil, err
	}
	h.ShortStat = strings.TrimSpace(out)
	return h, nil
}

func title(md string) string {
	for _, l := range strings.Split(md, "\n") {
		if strings.HasPrefix(l, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(l, "# "))
		}
	}
	return ""
}

// files reads the changed files of a commit and the word diffs of its
// Markdown files.
func (g Git) files(hash string) ([]File, error) {
	args := append([]string{"show", "--format=", "--numstat", "-z"}, diffFlags...)
	out, err := g.run(append(append(args, hash, "--"), Paths...)...)
	if err != nil {
		return nil, err
	}
	files, err := parseNumstat(out)
	if err != nil {
		return nil, fmt.Errorf("%.7s: %w", hash, err)
	}
	args = append([]string{"show", "--format=", "-U1", "--word-diff=plain", "--word-diff-regex=[^[:space:]]+"}, diffFlags...)
	out, err = g.run(append(append(args, hash, "--"), Paths...)...)
	if err != nil {
		return nil, err
	}
	diffs := splitDiff(out)
	if len(diffs) != len(files) {
		return nil, fmt.Errorf("%.7s: %d files in --numstat, %d in the diff", hash, len(files), len(diffs))
	}
	for i := range files {
		if strings.HasSuffix(files[i].Path, ".md") || strings.HasSuffix(files[i].Path, ".mdx") {
			files[i].Diff = diffs[i]
		}
	}
	return files, nil
}

// parseNumstat reads git --numstat -z: "added\tdeleted\tpath\0" or, for a
// rename, "added\tdeleted\t\0old\0new\0".
func parseNumstat(out string) ([]File, error) {
	var files []File
	parts := strings.Split(out, "\x00")
	for i := 0; i < len(parts); i++ {
		p := strings.TrimLeft(parts[i], "\n")
		if p == "" {
			continue
		}
		f := strings.SplitN(p, "\t", 3)
		if len(f) != 3 {
			return nil, fmt.Errorf("bad numstat record %q", p)
		}
		file := File{Added: f[0], Deleted: f[1], Path: f[2]}
		if f[2] == "" {
			if i+2 >= len(parts) {
				return nil, fmt.Errorf("truncated rename record")
			}
			file.OldPath, file.Path = parts[i+1], parts[i+2]
			i += 2
		}
		files = append(files, file)
	}
	return files, nil
}

// splitDiff splits a diff into its files and keeps the hunks of each (from
// the first @@ line); a file without hunks (binary, pure rename) is empty.
func splitDiff(out string) []string {
	var diffs []string
	var cur *strings.Builder
	inHunks := false
	for _, l := range strings.SplitAfter(out, "\n") {
		if strings.HasPrefix(l, "diff --git ") {
			if cur != nil {
				diffs = append(diffs, cur.String())
			}
			cur, inHunks = &strings.Builder{}, false
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(l, "@@") {
			inHunks = true
		}
		if inHunks {
			cur.WriteString(l)
		}
	}
	if cur != nil {
		diffs = append(diffs, cur.String())
	}
	for i, d := range diffs {
		if d != "" && !strings.HasSuffix(d, "\n") {
			diffs[i] = d + "\n"
		}
	}
	return diffs
}
