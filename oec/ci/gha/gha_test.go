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
