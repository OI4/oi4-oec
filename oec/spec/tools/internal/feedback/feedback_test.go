package feedback

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRegister(t *testing.T) {
	src := strings.Join([]string{
		"# Spec findings",
		"| ID | Area | Finding | Location | Proposed interpretation | Status |",
		"|----|------|---------|----------|-------------------------|--------|",
		"| SF-003 | MG 6.2.2 | found | MG 6.2.2 | proposal | resolved (`abc1234`) |",
		"| ID | Area | Outcome | Status |",
		"| SF-001 | Topics | form `…/<Call|Reply>/…` and a\\|b | resolved (`5f070bb`) |",
		"| SF-002 | Example | `MSG` removed | obsolete (`5f070bb`) |",
		"",
	}, "\n")
	fs, err := ParseRegister(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 3 || fs[0].ID != "SF-001" || fs[2].ID != "SF-003" {
		t.Fatalf("got %+v", fs)
	}
	if fs[0].Outcome != "form `…/<Call|Reply>/…` and a\\|b" || fs[0].Status != "resolved (`5f070bb`)" {
		t.Errorf("SF-001: %+v", fs[0])
	}
	if fs[2].Finding != "found" || fs[2].Location != "MG 6.2.2" || fs[2].Proposal != "proposal" || fs[2].Outcome != "" {
		t.Errorf("SF-003: %+v", fs[2])
	}
	for name, bad := range map[string]string{
		"cells":     "| SF-001 | a | b | c | d |",
		"id":        "| SF-01 | a | b | c |",
		"duplicate": "| SF-001 | a | b | c |\n| SF-001 | a | b | c |",
		"empty":     "# nothing",
	} {
		if _, err := ParseRegister(bad); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestParseAttribution(t *testing.T) {
	src := "# comment\n" +
		"aaaaaaa\tSF-001,GL-ADR-012\t\n" +
		"bbbbbbb\teditorial\tLinks added.\n"
	es, err := ParseAttribution(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 2 || es[0].Line != 2 || strings.Join(es[0].Refs, " ") != "SF-001 GL-ADR-012" || es[1].Class != "editorial" || es[1].Note != "Links added." {
		t.Fatalf("got %+v", es)
	}
	for name, bad := range map[string]string{
		"columns":   "aaaaaaa\tSF-001\n",
		"hash":      "aaaaaa\tSF-001\t\n",
		"long hash": "aaaaaaaa\tSF-001\t\n",
		"duplicate": "aaaaaaa\tSF-001\t\naaaaaaa\tSF-002\t\n",
		"note":      "aaaaaaa\twording\t\n",
		"class mix": "aaaaaaa\tSF-001,wording\tx\n",
		"bad id":    "aaaaaaa\tADR-012\t\n",
		"twice":     "aaaaaaa\tSF-001,SF-001\t\n",
	} {
		if _, err := ParseAttribution(bad); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func hash(c byte) string { return strings.Repeat(string(c), 40) }

func testPackage() *Package {
	h := &History{
		Base: "5f070bb" + strings.Repeat("0", 33), Target: hash('f'), BaseDate: "2026-10-14", TargetDate: "2026-10-31",
		ShortStat: "3 files changed",
		ADRs:      []ADR{{ID: "GL-ADR-012", Path: "rfcs_adrs/ADR-012_x.md", Title: "ADR 012 – Remove Actions"}},
		Commits: []Commit{
			{Hash: hash('a'), Date: "2026-10-15", Files: []File{
				{Path: "docs/guideline/a.md", Added: "1", Deleted: "1", Diff: "@@ -1 +1 @@\n[-old-]{+new `code` ```x```+}\n"},
				{Path: "static/img/guideline/f.png", Added: "-", Deleted: "-"},
			}},
			{Hash: hash('b'), Date: "2026-10-16", Files: []File{
				{Path: "docs/guideline/b.md", OldPath: "docs/guideline/old|b.md", Added: "500", Deleted: "0", Diff: strings.Repeat("{+x+}\n", MaxInline+1)},
			}},
			{Hash: hash('c'), Date: "2026-10-17", Files: []File{{Path: "docs/guideline/a.md", Added: "1", Deleted: "0", Diff: "@@ -2 +2 @@\n{+y+}\n"}}},
		},
	}
	return &Package{
		History: h,
		Findings: []Finding{
			{ID: "SF-001", Area: "Topics", Outcome: "done", Status: "resolved (`ccccccc`)"},
			{ID: "SF-002", Area: "A | B", Finding: "f", Location: "l", Proposal: "p", Status: "resolved (`aaaaaaa`)"},
			{ID: "SF-003", Area: "Old", Outcome: "o", Status: "resolved (`5f070bb`)"},
			{ID: "SF-004", Area: "Gone", Outcome: "o", Status: "obsolete (`5f070bb`)"},
		},
		Attribution: []Entry{
			{Line: 3, Commit: "ccccccc", Class: "editorial", Note: "Links added."},
			{Line: 1, Commit: "aaaaaaa", Refs: []string{"SF-002", "GL-ADR-012"}},
			{Line: 2, Commit: "bbbbbbb", Refs: []string{"GL-ADR-012", "SF-001"}, Note: "Renumbering."},
		},
	}
}

func TestProblems(t *testing.T) {
	p := testPackage()
	if got := p.Problems(); len(got) != 0 {
		t.Fatalf("problems: %v", got)
	}
	p.Attribution = append(p.Attribution[1:], Entry{Line: 9, Commit: "ddddddd", Refs: []string{"SF-009", "GL-ADR-099"}})
	p.History.MergesWithChanges = []string{hash('e')}
	p.Findings = append(p.Findings, Finding{ID: "SF-005", Status: "resolved (`1234567`)"})
	got := strings.Join(p.Problems(), "\n")
	for _, want := range []string{
		"commit ddddddd is not a commit of 5f070bb..fffffff",
		"SF-009 is not in the register",
		"GL-ADR-099 is not in rfcs_adrs",
		"commit ccccccc (2026-10-17) changes the documents but is not attributed",
		"SF-005 (resolved (`1234567`)) has no commit",
		"merge eeeeeee changes the documents itself",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "SF-003") || strings.Contains(got, "SF-004") {
		t.Errorf("findings resolved in the base or obsolete reported:\n%s", got)
	}
}

func TestBuild(t *testing.T) {
	files := testPackage().Build()
	want := []string{"README.md", "files.md", "normative.md", "wording.md", "editorial.md", "SF-001.md", "SF-002.md", "SF-003.md", "SF-004.md", "GL-ADR-012.md"}
	if len(files) != len(want) {
		t.Errorf("%d files, want %d", len(files), len(want))
	}
	for _, n := range want {
		if files[n] == "" {
			t.Errorf("missing %s", n)
		}
	}
	sf2 := files["SF-002.md"]
	for _, s := range []string{
		"# SF-002 – A | B\n", "## Finding\n\nf\n", "## Location\n\nl\n", "## Decision\n\np\n",
		"Related by shared commits: [GL-ADR-012](GL-ADR-012.md).",
		"### Commit aaaaaaa\n", "; also for [GL-ADR-012](GL-ADR-012.md).",
		"| `static/img/guideline/f.png` | - | - |",
		"````\n@@ -1 +1 @@\n[-old-]{+new `code` ```x```+}\n````\n",
	} {
		if !strings.Contains(sf2, s) {
			t.Errorf("SF-002.md lacks %q:\n%s", s, sf2)
		}
	}
	adr := files["GL-ADR-012.md"]
	for _, s := range []string{
		"The changes of this commit are shown on the page of [SF-002](SF-002.md): [`aaaaaaa`](SF-002.md#commit-aaaaaaa).",
		"### Commit bbbbbbb\n", "Renumbering.",
		"| `docs/guideline/old\\|b.md` → `docs/guideline/b.md` | 500 | 0 |",
		"The word diff of this commit has 401 lines and is not shown here.",
	} {
		if !strings.Contains(adr, s) {
			t.Errorf("GL-ADR-012.md lacks %q:\n%s", s, adr)
		}
	}
	if strings.Contains(adr, "{+x+}") {
		t.Error("a diff over MaxInline is shown inline")
	}
	if !strings.Contains(files["SF-001.md"], "The changes of this commit are shown on the page of [GL-ADR-012](GL-ADR-012.md)") {
		t.Errorf("SF-001.md:\n%s", files["SF-001.md"])
	}
	if !strings.Contains(files["SF-003.md"], "None in the range: the finding was resolved in the base `5f070bb`.") ||
		!strings.Contains(files["SF-004.md"], "None: the finding became obsolete.") {
		t.Error("finding without commits")
	}
	if ed := files["editorial.md"]; !strings.Contains(ed, "(1 commits)") || !strings.Contains(ed, "### Commit ccccccc\n") || !strings.Contains(ed, "{+y+}") {
		t.Errorf("editorial.md:\n%s", ed)
	}
	readme := files["README.md"]
	for _, s := range []string{
		"| [SF-002](SF-002.md) | A \\| B | resolved | 1 |",
		"| [SF-003](SF-003.md) | Old | resolved | 0 |",
		"| [GL-ADR-012](GL-ADR-012.md) | ADR 012 – Remove Actions | 2 |",
		"- Findings: 4 (1 obsolete, 3 resolved).",
		"- Commits that change the documents: 3 – 1 shown on a finding page, 1 on a GL-ADR page, 0 normative, 0 wording, 1 editorial.",
		"**Complete diff:** 3 files changed",
	} {
		if !strings.Contains(readme, s) {
			t.Errorf("README.md lacks %q:\n%s", s, readme)
		}
	}
	if fl := files["files.md"]; !strings.Contains(fl, "| `docs/guideline/a.md` | [`aaaaaaa`](SF-002.md#commit-aaaaaaa) ([SF-002](SF-002.md)), [`ccccccc`](editorial.md#commit-ccccccc) ([editorial](editorial.md)) |") ||
		!strings.Contains(fl, "(from `docs/guideline/old\\|b.md`)") {
		t.Errorf("files.md:\n%s", fl)
	}
}

func TestFenceFor(t *testing.T) {
	for in, want := range map[string]string{"x": "```", "``": "```", "a ``` b": "````", "`````": "``````"} {
		if got := fenceFor(in); got != want {
			t.Errorf("fenceFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseNumstat(t *testing.T) {
	got, err := parseNumstat("\n1\t2\tdocs/a b.md\x00-\t-\tstatic/img/guideline/f.png\x003\t0\t\x00docs/old.md\x00docs/new.md\x00")
	if err != nil {
		t.Fatal(err)
	}
	want := []File{
		{Path: "docs/a b.md", Added: "1", Deleted: "2"},
		{Path: "static/img/guideline/f.png", Added: "-", Deleted: "-"},
		{Path: "docs/new.md", OldPath: "docs/old.md", Added: "3", Deleted: "0"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestGitLoad builds a small guideline repository: a base, Markdown changes,
// a rename, a figure, a file outside the documents, a clean merge and a merge
// with changes of its own.
func TestGitLoad(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
			"GIT_AUTHOR_DATE=2026-10-14T12:00:00Z", "GIT_COMMITTER_DATE=2026-10-14T12:00:00Z")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(msg string) string {
		git("add", "-A")
		git("commit", "-q", "-m", msg)
		return git("rev-parse", "HEAD")
	}
	git("init", "-q", "-b", "main")
	long := "line one of a long paragraph with many words so that the rename is detected\n"
	write("docs/guideline/a.md", "# A\n\nThe client must connect.\n\nSecond paragraph.\n")
	write("docs/guideline/old name.md", strings.Repeat(long, 5))
	write("rfcs_adrs/ADR-012_Remove_x.md", "# ADR 012 – Remove x\n")
	write("src/site.js", "x\n")
	base := commit("base")
	write("docs/guideline/a.md", "# A\n\nThe client shall connect.\n\nSecond paragraph.\n")
	write("src/site.js", "y\n")
	c1 := commit("wording")
	git("mv", "docs/guideline/old name.md", "docs/guideline/new name.md")
	write("static/img/guideline/f.png", "\x89PNG\x00\x01\x02")
	c2 := commit("rename and figure")
	write("src/site.js", "z\n")
	commit("site only")
	git("checkout", "-q", "-b", "side", base)
	write("docs/process-description/p.md", "process\n")
	commit("side")
	git("checkout", "-q", "main")
	git("merge", "-q", "--no-ff", "-m", "clean merge", "side")
	git("checkout", "-q", "-b", "evil", "HEAD~1")
	write("docs/intro.md", "intro\n")
	commit("evil side")
	git("checkout", "-q", "main")
	git("merge", "-q", "--no-ff", "--no-commit", "evil")
	write("docs/guideline/a.md", "# A\n\nThe client shall connect.\n\nChanged in the merge.\n")
	git("add", "-A")
	git("commit", "-q", "-m", "evil merge")
	evil := git("rev-parse", "HEAD")

	h, err := Git{Dir: dir}.Load(base[:7], "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if h.Base != base || h.Target != evil || h.BaseDate != "2026-10-14" {
		t.Errorf("range %s..%s %s", h.Base, h.Target, h.BaseDate)
	}
	if len(h.Commits) != 4 || h.Commits[0].Hash != c1 || h.Commits[1].Hash != c2 {
		t.Fatalf("commits: %+v", h.Commits)
	}
	a := h.Commits[0].Files
	if len(a) != 1 || a[0].Path != "docs/guideline/a.md" || a[0].Diff != "@@ -2,3 +2,3 @@\n\nThe client [-must-]{+shall+} connect.\n\n" {
		t.Errorf("c1 files: %+v", a)
	}
	b := h.Commits[1].Files
	if len(b) != 2 || b[0].OldPath != "docs/guideline/old name.md" || b[0].Path != "docs/guideline/new name.md" || b[0].Diff != "" ||
		b[1].Path != "static/img/guideline/f.png" || b[1].Added != "-" || b[1].Diff != "" {
		t.Errorf("c2 files: %+v", b)
	}
	if len(h.MergesWithChanges) != 1 || h.MergesWithChanges[0] != evil {
		t.Errorf("merges with changes: %v", h.MergesWithChanges)
	}
	if len(h.ADRs) != 1 || h.ADRs[0].ID != "GL-ADR-012" || h.ADRs[0].Title != "ADR 012 – Remove x" {
		t.Errorf("ADRs: %+v", h.ADRs)
	}
	// Only the documents count: a.md, the rename, the figure, p.md and intro.md, not src/site.js.
	if !strings.HasPrefix(h.ShortStat, "5 files changed,") {
		t.Errorf("shortstat %q", h.ShortStat)
	}
	if _, err := (Git{Dir: dir}).Load(c2, c1); err == nil {
		t.Error("base after target accepted")
	}
}
