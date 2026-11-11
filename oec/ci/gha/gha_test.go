package gha

import (
	"strings"
	"testing"
)

func TestQuote(t *testing.T) {
	cases := map[string]string{
		"gate":                            "gate",
		"ubuntu-24.04":                    "ubuntu-24.04",
		"${{ matrix.os }}":                "${{ matrix.os }}",
		"oec/check.sh (${{ matrix.os }})": "oec/check.sh (${{ matrix.os }})",
		"oec-${{ hashFiles('a', 'b/**/Cargo.lock') }}": "oec-${{ hashFiles('a', 'b/**/Cargo.lock') }}",
		"1.89.0":     "1.89.0",
		"":           `""`,
		"true":       `"true"`,
		"On":         `"On"`,
		"no":         `"no"`,
		"null":       `"null"`,
		"~":          `"~"`,
		"0":          `"0"`,
		"14":         `"14"`,
		"1.26":       `"1.26"`,
		"1e3":        `"1e3"`,
		"0x1F":       `"0x1F"`,
		"2026-11-20": `"2026-11-20"`,
		"a: b":       `"a: b"`,
		"a #b":       `"a #b"`,
		"-x":         `"-x"`,
		"*x":         `"*x"`,
		"&x":         `"&x"`,
		"!x":         `"!x"`,
		"{x}":        `"{x}"`,
		"[x]":        `"[x]"`,
		".x":         `".x"`,
		"x ":         `"x "`,
		" x":         `" x"`,
		"a\tb":       `"a\tb"`,
		`say "hi"`:   `"say \"hi\""`,
		"%x":         `"%x"`,
		"a<b>&c":     "a<b>&c",
	}
	for in, want := range cases {
		if got := Quote(in); got != want {
			t.Errorf("Quote(%q) = %s, want %s", in, got, want)
		}
	}
}

func sample() Workflow {
	co := Action{Repo: "actions/checkout", SHA: strings.Repeat("a", 40), Tag: "v1.0.0"}
	return Workflow{
		File:         "x.yml",
		Comment:      []string{"generated", ""},
		Name:         "x",
		PushBranches: []string{"main"},
		PullRequest:  true,
		Dispatch:     true,
		Permissions:  []KV{{"contents", "read"}},
		Concurrency:  Concurrency{Group: "${{ github.workflow }}-${{ github.ref }}", CancelInProgress: true},
		Env:          []KV{{"GOTOOLCHAIN", "auto"}},
		Jobs: []Job{{
			ID: "build", Name: "build (${{ matrix.os }})", RunsOn: "${{ matrix.os }}",
			Matrix:         []Entry{{{"os", "r1"}, {"cc", "clang"}}, {{"os", "r2"}, {"cc", "cc"}}},
			TimeoutMinutes: 30,
			Env:            []KV{{"CC", "${{ matrix.cc }}"}},
			Steps: []Step{
				{Name: "Check out", Uses: co, With: []KV{{"persist-credentials", "false"}, {"token", "${{ secrets.TOKEN || github.token }}"}}},
				{Name: "Cache", Uses: co, With: []KV{{"path", "~/a\n~/b\n"}, {"persist-credentials", "false"}}},
				{Name: "Run", Run: "a.sh\nb.sh"},
			},
		}},
	}
}

func samplePolicy() Policy {
	return Policy{
		Actions:    []Action{{Repo: "actions/checkout", SHA: strings.Repeat("a", 40), Tag: "v1.0.0"}},
		Runners:    map[string]bool{"r1": true, "r2": true},
		Secrets:    map[string]SecretUse{"TOKEN": {ActionRepo: "actions/checkout", WithKey: "token"}},
		MaxTimeout: 60,
	}
}

func TestRender(t *testing.T) {
	got, err := Render(sample())
	if err != nil {
		t.Fatal(err)
	}
	want := `# generated
#
name: x
on:
  push:
    branches:
      - main
  pull_request:
  workflow_dispatch:
permissions:
  contents: read
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
defaults:
  run:
    shell: bash
env:
  GOTOOLCHAIN: auto
jobs:
  build:
    name: build (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    timeout-minutes: 30
    strategy:
      fail-fast: false
      matrix:
        include:
          - os: r1
            cc: clang
          - os: r2
            cc: cc
    env:
      CC: ${{ matrix.cc }}
    steps:
      - name: Check out
        uses: actions/checkout@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          persist-credentials: "false"
          token: ${{ secrets.TOKEN || github.token }}
      - name: Cache
        uses: actions/checkout@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: |
            ~/a
            ~/b
          persist-credentials: "false"
      - name: Run
        run: |-
          a.sh
          b.sh
`
	if string(got) != want {
		t.Errorf("Render:\n%s\nwant:\n%s", got, want)
	}
	if errs := Check(sample(), samplePolicy()); len(errs) > 0 {
		t.Errorf("Check(sample) = %v", errs)
	}
}

// release is a workflow with a tag trigger and a job that needs another.
func release() Workflow {
	w := sample()
	w.PushBranches, w.PullRequest = nil, false
	w.PushTags = []string{"oec-v*"}
	w.Jobs[0].Steps = w.Jobs[0].Steps[2:]
	w.Jobs = append(w.Jobs, Job{
		ID: "all", Name: "all", RunsOn: "r1", TimeoutMinutes: 10, Needs: []string{"build"},
		Steps: []Step{{Name: "Run", Run: "c.sh"}},
	})
	return w
}

func TestRenderTagsNeeds(t *testing.T) {
	got, err := Render(release())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"on:\n  push:\n    tags:\n      - oec-v*\n  workflow_dispatch:\npermissions:\n",
		"  all:\n    name: all\n    needs:\n      - build\n    runs-on: r1\n    timeout-minutes: 10\n",
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("Render lacks\n%s\ngot:\n%s", want, got)
		}
	}
	if strings.Contains(string(got), "branches:") || strings.Contains(string(got), "pull_request") {
		t.Errorf("Render of a tag workflow has a branch or pull request trigger:\n%s", got)
	}
	if errs := Check(release(), samplePolicy()); len(errs) > 0 {
		t.Errorf("Check(release) = %v", errs)
	}
	tagsOnly := release()
	tagsOnly.Dispatch = false
	if errs := Check(tagsOnly, samplePolicy()); len(errs) > 0 {
		t.Errorf("Check(tag trigger only) = %v", errs)
	}
}

// TestRenderPathsIgnore: paths-ignore goes under push and pull_request
// (C-111 D-3 (a): documentation-only pushes run no CI).
func TestRenderPathsIgnore(t *testing.T) {
	w := sample()
	w.PathsIgnore = []string{"project/**"}
	got, err := Render(w)
	if err != nil {
		t.Fatal(err)
	}
	want := "on:\n  push:\n    branches:\n      - main\n    paths-ignore:\n      - project/**\n" +
		"  pull_request:\n    paths-ignore:\n      - project/**\n  workflow_dispatch:\npermissions:\n"
	if !strings.Contains(string(got), want) {
		t.Errorf("Render lacks\n%s\ngot:\n%s", want, got)
	}
	if errs := Check(w, samplePolicy()); len(errs) > 0 {
		t.Errorf("Check(paths-ignore) = %v", errs)
	}
	for name, edit := range map[string]func(w *Workflow){
		"tags only": func(w *Workflow) {
			w.PushBranches, w.PullRequest, w.PushTags = nil, false, []string{"oec-v*"}
		},
		"empty":    func(w *Workflow) { w.PathsIgnore = []string{""} },
		"negation": func(w *Workflow) { w.PathsIgnore = []string{"!project/x"} },
		"absolute": func(w *Workflow) { w.PathsIgnore = []string{"/project/**"} },
	} {
		w := sample()
		w.PathsIgnore = []string{"project/**"}
		edit(&w)
		found := false
		for _, err := range Check(w, samplePolicy()) {
			if strings.Contains(err.Error(), "paths-ignore") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: Check accepted paths-ignore %q", name, w.PathsIgnore)
		}
	}
}

// publishing is release() with a publishing job behind an environment that
// gets an environment secret in the env of its run step (C-110).
func publishing() Workflow {
	w := release()
	w.Jobs = append(w.Jobs, Job{
		ID: "publish", Name: "publish", Needs: []string{"all"}, If: "github.ref_type == 'tag'",
		RunsOn: "r1", Environment: "release", TimeoutMinutes: 10,
		Steps: []Step{{Name: "Publish", Env: []KV{{"DEPLOY_TOKEN", "${{ secrets.DEPLOY }}"}}, Run: "d.sh"}},
	})
	return w
}

func publishingPolicy() Policy {
	p := samplePolicy()
	p.Secrets["DEPLOY"] = SecretUse{Environment: "release", EnvKey: "DEPLOY_TOKEN"}
	return p
}

func TestRenderEnvironment(t *testing.T) {
	got, err := Render(publishing())
	if err != nil {
		t.Fatal(err)
	}
	want := "  publish:\n    name: publish\n    needs:\n      - all\n    if: github.ref_type == 'tag'\n    runs-on: r1\n" +
		"    environment: release\n    timeout-minutes: 10\n    steps:\n      - name: Publish\n        env:\n" +
		"          DEPLOY_TOKEN: ${{ secrets.DEPLOY }}\n        run: d.sh\n"
	if !strings.Contains(string(got), want) {
		t.Errorf("Render lacks\n%s\ngot:\n%s", want, got)
	}
	if errs := Check(publishing(), publishingPolicy()); len(errs) > 0 {
		t.Errorf("Check(publishing) = %v", errs)
	}
	cases := map[string]struct {
		edit func(w *Workflow)
		want string
	}{
		"no environment":    {func(w *Workflow) { w.Jobs[2].Environment = "" }, "secret DEPLOY only in"},
		"other environment": {func(w *Workflow) { w.Jobs[2].Environment = "test" }, "secret DEPLOY only in"},
		"other env key":     {func(w *Workflow) { w.Jobs[2].Steps[0].Env[0].Key = "TOKEN" }, "secret DEPLOY only in"},
		"job env":           {func(w *Workflow) { w.Jobs[2].Env = w.Jobs[2].Steps[0].Env }, "secret DEPLOY only in"},
		"action with": {func(w *Workflow) {
			w.Jobs[2].Steps[0] = Step{Name: "Use", Uses: samplePolicy().Actions[0],
				With: []KV{{"persist-credentials", "false"}, {"DEPLOY_TOKEN", "${{ secrets.DEPLOY }}"}}}
		}, "secret DEPLOY only in"},
		"environment name": {func(w *Workflow) { w.Jobs[2].Environment = "Release now" }, "environment"},
		"pull request":     {func(w *Workflow) { w.PullRequest = true }, "environment in a workflow with a pull_request trigger"},
		"job if context":   {func(w *Workflow) { w.Jobs[2].If = "${{ github.event.ref }}" }, "github.event"},
		"job if needs":     {func(w *Workflow) { w.Jobs[2].If = "needs.all.result == 'success'" }, "needs"},
		"with secret in environment job": {func(w *Workflow) {
			w.Jobs[2].Steps[0].Env[0] = KV{"T", "${{ secrets.TOKEN }}"}
		}, "secret TOKEN only in"},
	}
	for name, c := range cases {
		w := publishing()
		c.edit(&w)
		found := false
		errs := Check(w, publishingPolicy())
		for _, err := range errs {
			if strings.Contains(err.Error(), c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want an error with %q, got %v", name, c.want, errs)
		}
	}
}

func TestRenderRejects(t *testing.T) {
	for name, run := range map[string]string{
		"leading space":   " a\nb",
		"trailing space":  "a \nb",
		"trailing blanks": "a\nb\n\n",
	} {
		w := sample()
		w.Jobs[0].Steps[2].Run = run
		if _, err := Render(w); err == nil {
			t.Errorf("%s: Render accepted %q", name, run)
		}
	}
}

func TestCheckRejects(t *testing.T) {
	other := Action{Repo: "evil/action", SHA: strings.Repeat("b", 40), Tag: "v1"}
	cases := map[string]struct {
		edit func(w *Workflow)
		want string
	}{
		"file name":        {func(w *Workflow) { w.File = "X.yaml" }, "file name"},
		"no trigger":       {func(w *Workflow) { w.PushBranches, w.PullRequest, w.Dispatch = nil, false, false }, "no trigger"},
		"write permission": {func(w *Workflow) { w.Permissions = []KV{{"contents", "write"}} }, "permissions"},
		"more permissions": {func(w *Workflow) { w.Permissions = append(w.Permissions, KV{"packages", "read"}) }, "permissions"},
		"no concurrency":   {func(w *Workflow) { w.Concurrency.CancelInProgress = false }, "concurrency"},
		"no timeout":       {func(w *Workflow) { w.Jobs[0].TimeoutMinutes = 0 }, "timeout-minutes"},
		"long timeout":     {func(w *Workflow) { w.Jobs[0].TimeoutMinutes = 61 }, "timeout-minutes"},
		"job id":           {func(w *Workflow) { w.Jobs[0].ID = "Build" }, "id must match"},
		"duplicate job":    {func(w *Workflow) { w.Jobs = append(w.Jobs, w.Jobs[0]) }, "unique"},
		"self-hosted":      {func(w *Workflow) { w.Jobs[0].Matrix[1][0].Value = "self-hosted" }, "not an allowed hosted runner"},
		"runs-on literal":  {func(w *Workflow) { w.Jobs[0].RunsOn = "r1" }, "runs-on must be"},
		"matrix keys":      {func(w *Workflow) { w.Jobs[0].Matrix[1] = Entry{{"os", "r2"}, {"cxx", "c"}} }, "other keys"},
		"fail-fast alone":  {func(w *Workflow) { w.Jobs[0].Matrix, w.Jobs[0].RunsOn, w.Jobs[0].FailFast = nil, "r1", true }, "fail-fast"},
		"expr in run":      {func(w *Workflow) { w.Jobs[0].Steps[2].Run = "echo ${{ matrix.os }}" }, "no expression in run"},
		"event context": {func(w *Workflow) {
			w.Jobs[0].Steps[2].Env = []KV{{"T", "${{ github.event.pull_request.title }}"}}
		}, "github.event"},
		"head_ref":       {func(w *Workflow) { w.Jobs[0].Name = "${{ github.head_ref }}" }, "github.head_ref"},
		"unterminated":   {func(w *Workflow) { w.Jobs[0].Env[0].Value = "${{ matrix.cc" }, "unterminated"},
		"unpinned":       {func(w *Workflow) { w.Jobs[0].Steps[0].Uses.SHA = "v1" }, "full commit SHA"},
		"not in pins":    {func(w *Workflow) { w.Jobs[0].Steps[0].Uses.Tag = "v2.0.0" }, "pin list"},
		"third party":    {func(w *Workflow) { w.Jobs[0].Steps[1].Uses = other }, "first-party"},
		"both uses+run":  {func(w *Workflow) { w.Jobs[0].Steps[1].Run = "x" }, "exactly one"},
		"neither":        {func(w *Workflow) { w.Jobs[0].Steps[2].Run = "" }, "exactly one"},
		"with on run":    {func(w *Workflow) { w.Jobs[0].Steps[2].With = []KV{{"a", "b"}} }, "with on a run step"},
		"credentials":    {func(w *Workflow) { w.Jobs[0].Steps[0].With[0].Value = "true" }, "persist-credentials"},
		"unknown secret": {func(w *Workflow) { w.Jobs[0].Steps[0].With[1].Value = "${{ secrets.OTHER }}" }, "secret OTHER is not allowed"},
		"secret in env":  {func(w *Workflow) { w.Jobs[0].Env[0].Value = "${{ secrets.TOKEN }}" }, "secret TOKEN only in"},
		"secret in run step env": {func(w *Workflow) {
			w.Jobs[0].Steps[2].Env = []KV{{"T", "${{ secrets.TOKEN }}"}}
		}, "secret TOKEN only in"},
		"secret other key": {func(w *Workflow) { w.Jobs[0].Steps[0].With[0] = KV{"ssh-key", "${{ secrets.TOKEN }}"} }, "secret TOKEN only in"},
		"step name":        {func(w *Workflow) { w.Jobs[0].Steps[2].Name = "" }, "no name"},
		"no steps":         {func(w *Workflow) { w.Jobs[0].Steps = nil }, "no steps"},
		"no jobs":          {func(w *Workflow) { w.Jobs = nil }, "no jobs"},
		"tag pattern":      {func(w *Workflow) { w.PushTags = []string{"v* x"} }, "tag pattern"},
		"needs unknown":    {func(w *Workflow) { w.Jobs[0].Needs = []string{"nope"} }, "needs nope"},
		"needs itself":     {func(w *Workflow) { w.Jobs[0].Needs = []string{"build"} }, "needs build"},
		"needs later job": {func(w *Workflow) {
			w.Jobs = append(w.Jobs, Job{ID: "later", Name: "later", RunsOn: "r1", TimeoutMinutes: 1,
				Steps: []Step{{Name: "Run", Run: "x"}}})
			w.Jobs[0].Needs = []string{"later"}
		}, "needs later"},
		"needs expression": {func(w *Workflow) { w.Jobs[0].Steps[2].Env = []KV{{"X", "${{ needs.a.result }}"}} }, "expression uses needs"},
		"pipe to shell": {func(w *Workflow) {
			w.Jobs[0].Steps[2].Run = "curl -sSf https://sh.example.org |\n  sh -s -- -y"
		}, "pipes a download into a shell"},
	}
	for name, c := range cases {
		w := sample()
		c.edit(&w)
		errs := Check(w, samplePolicy())
		found := false
		for _, err := range errs {
			if strings.Contains(err.Error(), c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want an error with %q, got %v", name, c.want, errs)
		}
	}
}

// TestPipesToShell (C-113 RT-3): a download piped into a shell runs code
// without an integrity check.
func TestPipesToShell(t *testing.T) {
	for s, want := range map[string]bool{
		"curl -sSf https://sh.rustup.rs | sh":             true,
		"curl -sSf https://x.example |sh -s -- -y":        true,
		"curl -sSf https://x.example |\n\tsh -s -- -y":    true,
		"curl -sSf https://x.example | \\\n  bash":        true,
		"wget -qO- https://x.example | sudo -E bash":      true,
		"curl -fsSL https://x.example | /bin/sh":          true,
		"curl -fsSL https://x.example | zsh":              true,
		"a | b\ncurl x.example | dash -s":                 true,
		"sha256sum f | cut -d' ' -f1":                     false,
		"shasum -a 256 f | cut -d' ' -f1":                 false,
		"x | shasum -a 256":                               false,
		"command -v rustup || sh ./install.sh":            false,
		"sh -c 'curl -fsS http://127.0.0.1/api | jq .'":   false,
		"oec/ci/setup-tools.sh --toolchain\noec/check.sh": false,
	} {
		if got := PipesToShell(s); got != want {
			t.Errorf("PipesToShell(%q) = %v, want %v", s, got, want)
		}
	}
}
