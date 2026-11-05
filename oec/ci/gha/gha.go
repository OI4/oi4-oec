// Package gha models GitHub Actions workflows as far as the OEC pipelines
// need them (ADR-0032 §3), renders them as YAML without a YAML library and
// checks them against the CI policy of ADR-0032 §4 (Check).
//
// The model can only express what the policy allows: the triggers push (of
// branches or tags), pull_request and workflow_dispatch, workflow-level
// permissions, bash as the only shell, jobs that need earlier jobs, and
// steps that either run a command or use an action.
package gha

import "strings"

// KV is one entry of an ordered map (env, with, permissions, matrix entry).
type KV struct{ Key, Value string }

// Get returns the value of key in kvs and whether it is present.
func Get(kvs []KV, key string) (string, bool) {
	for _, kv := range kvs {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return "", false
}

// Workflow is one file under .github/workflows.
type Workflow struct {
	File    string   // file name, e.g. gate.yml
	Comment []string // header comment lines (without "# ")
	Name    string

	PushBranches []string // push trigger on these branches
	PushTags     []string // push trigger on these tag patterns (release.yml, C-109)
	PullRequest  bool     // pull_request trigger (never pull_request_target)
	Dispatch     bool     // workflow_dispatch trigger

	Permissions []KV // workflow level, the jobs inherit them
	Concurrency Concurrency
	Env         []KV
	Jobs        []Job
}

// Concurrency is the concurrency group of a workflow.
type Concurrency struct {
	Group            string
	CancelInProgress bool
}

// Job is one job of a workflow.
type Job struct {
	ID             string
	Name           string
	Needs          []string // ids of earlier jobs this job waits for
	RunsOn         string   // a runner label or "${{ matrix.os }}"
	Matrix         []Entry  // strategy.matrix.include; empty: no matrix
	FailFast       bool     // strategy.fail-fast (only with a matrix)
	TimeoutMinutes int
	Env            []KV
	Steps          []Step
}

// Entry is one entry of strategy.matrix.include.
type Entry []KV

// Step is one step of a job: either Uses or Run is set.
type Step struct {
	Name string
	If   string
	Uses Action // zero: a run step
	With []KV
	Env  []KV
	Run  string
}

// Action is an action pinned to a full commit SHA; Tag is the release the
// SHA belongs to (rendered as a comment).
type Action struct{ Repo, SHA, Tag string }

// IsZero reports whether a is the zero Action (a run step).
func (a Action) IsZero() bool { return a == Action{} }

// Ref is the uses: value.
func (a Action) Ref() string { return a.Repo + "@" + a.SHA }

// MatrixOS returns the os values of the matrix of j, or RunsOn without a
// matrix.
func (j Job) MatrixOS() []string {
	if len(j.Matrix) == 0 {
		return []string{j.RunsOn}
	}
	var oses []string
	for _, e := range j.Matrix {
		if os, ok := Get(e, "os"); ok {
			oses = append(oses, os)
		}
	}
	return oses
}

// Commands returns the shell commands of all run steps of w, one per line.
func (w Workflow) Commands() []string {
	var cmds []string
	for _, j := range w.Jobs {
		for _, s := range j.Steps {
			for _, l := range strings.Split(s.Run, "\n") {
				if l = strings.TrimSpace(l); l != "" {
					cmds = append(cmds, l)
				}
			}
		}
	}
	return cmds
}
