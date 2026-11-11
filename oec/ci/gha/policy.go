package gha

import (
	"fmt"
	"regexp"
	"strings"
)

// Policy is what Check allows beyond the fixed rules.
type Policy struct {
	Actions []Action        // the pinned actions (exact repo, SHA and tag)
	Runners map[string]bool // the allowed runner labels (hosted runners only)
	// Secrets maps each allowed secret to the only place it may appear.
	Secrets map[string]SecretUse
	// MaxTimeout is the upper bound of timeout-minutes of a job.
	MaxTimeout int
}

// SecretUse is the only place a secret may be used: the with key WithKey of
// a step that uses the action repository ActionRepo, or (an environment
// secret, C-110) the env key EnvKey of a run step of a job with the
// deployment environment Environment.
type SecretUse struct{ ActionRepo, WithKey, Environment, EnvKey string }

func (u SecretUse) String() string {
	if u.Environment != "" {
		return fmt.Sprintf("env.%s of a run step of a job with environment %s", u.EnvKey, u.Environment)
	}
	return fmt.Sprintf("with.%s of %s", u.WithKey, u.ActionRepo)
}

var (
	fileRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.yml$`)
	idRe     = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	shaRe    = regexp.MustCompile(`^[0-9a-f]{40}$`)
	secretRe = regexp.MustCompile(`secrets\.([A-Za-z_][A-Za-z0-9_]*)`)
	exprRe   = regexp.MustCompile(`\$\{\{(.*?)\}\}`)
	tagRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/*-]*$`)
	pathRe   = regexp.MustCompile(`^[A-Za-z0-9._][A-Za-z0-9._/*-]*$`) // relative, no negation (C-111)
)

// untrusted are contexts an attacker controls in a pull request (script
// injection); they must not appear anywhere in a workflow.
var untrusted = []string{"github.event", "github.head_ref", "inputs.", "steps.", "needs."}

// Check returns every violation of the CI policy (ADR-0032 §4) in w.
func Check(w Workflow, p Policy) []error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(w.File+": "+format, a...)) }

	if !fileRe.MatchString(w.File) {
		bad("file name must match %s", fileRe)
	}
	if w.Name == "" {
		bad("no name")
	}
	if len(w.Comment) == 0 {
		bad("no header comment")
	}
	if len(w.PushBranches) == 0 && len(w.PushTags) == 0 && !w.PullRequest && !w.Dispatch {
		bad("no trigger")
	}
	for _, tag := range w.PushTags {
		if !tagRe.MatchString(tag) {
			bad("tag pattern %q must match %s", tag, tagRe)
		}
	}
	if len(w.PathsIgnore) > 0 && len(w.PushBranches) == 0 && !w.PullRequest {
		bad("paths-ignore needs a branch push or a pull_request trigger")
	}
	for _, p := range w.PathsIgnore {
		if !pathRe.MatchString(p) {
			bad("paths-ignore pattern %q must match %s", p, pathRe)
		}
	}
	if len(w.Permissions) != 1 || w.Permissions[0] != (KV{"contents", "read"}) {
		bad("permissions must be exactly contents: read, got %v", w.Permissions)
	}
	if w.Concurrency.Group == "" || !w.Concurrency.CancelInProgress {
		bad("concurrency needs a group and cancel-in-progress")
	}
	// checkValues checks the values of kvs; allowed reports whether the
	// secret use may be at key (nil: nowhere).
	checkValues := func(where string, kvs []KV, allowed func(key string, use SecretUse) bool) {
		for _, kv := range kvs {
			checkExpr(where+" "+kv.Key, kv.Value, bad)
			for _, m := range secretRe.FindAllStringSubmatch(kv.Value, -1) {
				use, ok := p.Secrets[m[1]]
				if !ok {
					bad("%s: secret %s is not allowed", where, m[1])
					continue
				}
				if allowed == nil || !allowed(kv.Key, use) {
					bad("%s: secret %s only in %s", where, m[1], use)
				}
			}
		}
	}
	checkValues("env", w.Env, nil)
	checkExpr("concurrency", w.Concurrency.Group, bad)

	ids := map[string]bool{}
	for _, j := range w.Jobs {
		where := "job " + j.ID
		if !idRe.MatchString(j.ID) || ids[j.ID] {
			bad("%s: id must match %s and be unique", where, idRe)
		}
		ids[j.ID] = true
		if j.Name == "" {
			bad("%s: no name", where)
		}
		for _, n := range j.Needs {
			if !ids[n] || n == j.ID {
				bad("%s: needs %s, which is not an earlier job", where, n)
			}
		}
		checkExpr(where+" name", j.Name, bad)
		checkCond(where+" if", j.If, bad)
		if j.Environment != "" {
			if !idRe.MatchString(j.Environment) {
				bad("%s: environment must match %s", where, idRe)
			}
			if w.PullRequest {
				bad("%s: environment in a workflow with a pull_request trigger", where)
			}
		}
		if j.TimeoutMinutes < 1 || j.TimeoutMinutes > p.MaxTimeout {
			bad("%s: timeout-minutes must be 1 … %d", where, p.MaxTimeout)
		}
		if len(j.Matrix) > 0 {
			if j.RunsOn != "${{ matrix.os }}" {
				bad("%s: with a matrix runs-on must be ${{ matrix.os }}", where)
			}
			for i, e := range j.Matrix {
				if len(e) != len(j.Matrix[0]) {
					bad("%s: matrix entry %d has other keys than entry 0", where, i)
				}
				for k, kv := range e {
					if k < len(j.Matrix[0]) && kv.Key != j.Matrix[0][k].Key {
						bad("%s: matrix entry %d has other keys than entry 0", where, i)
					}
				}
				checkValues(where+" matrix", e, nil)
			}
		} else if j.FailFast {
			bad("%s: fail-fast without a matrix", where)
		}
		for _, os := range j.MatrixOS() {
			if !p.Runners[os] {
				bad("%s: runner %q is not an allowed hosted runner", where, os)
			}
		}
		checkValues(where+" env", j.Env, nil)
		if len(j.Steps) == 0 {
			bad("%s: no steps", where)
		}
		for i := range j.Steps {
			s := &j.Steps[i]
			where := fmt.Sprintf("%s step %d (%s)", where, i, s.Name)
			if s.Name == "" {
				bad("%s: no name", where)
			}
			checkCond(where+" if", s.If, bad)
			var envSecret func(string, SecretUse) bool
			if s.Run != "" && j.Environment != "" {
				envSecret = func(key string, use SecretUse) bool {
					return use.Environment == j.Environment && use.EnvKey == key
				}
			}
			checkValues(where+" env", s.Env, envSecret)
			switch {
			case s.Uses.IsZero() == (s.Run == ""):
				bad("%s: exactly one of uses and run", where)
			case s.Run != "":
				if strings.Contains(s.Run, "${{") {
					bad("%s: no expression in run, pass it through env", where)
				}
				if len(s.With) > 0 {
					bad("%s: with on a run step", where)
				}
				if PipesToShell(s.Run) {
					bad("%s: run pipes a download into a shell; download, check its SHA-256, then run it", where)
				}
			default:
				checkAction(where, s.Uses, p.Actions, bad)
				checkValues(where+" with", s.With, func(key string, use SecretUse) bool {
					return use.ActionRepo != "" && use.ActionRepo == s.Uses.Repo && use.WithKey == key
				})
				if s.Uses.Repo == "actions/checkout" {
					if v, _ := Get(s.With, "persist-credentials"); v != "false" {
						bad("%s: checkout needs persist-credentials: false", where)
					}
				}
			}
		}
	}
	if len(w.Jobs) == 0 {
		bad("no jobs")
	}
	return errs
}

func checkAction(where string, a Action, pinned []Action, bad func(string, ...any)) {
	if !strings.HasPrefix(a.Repo, "actions/") {
		bad("%s: only first-party actions (actions/*), got %s", where, a.Repo)
	}
	if !shaRe.MatchString(a.SHA) || a.Tag == "" {
		bad("%s: %s must be pinned to a full commit SHA with its tag", where, a.Repo)
	}
	for _, p := range pinned {
		if p == a {
			return
		}
	}
	bad("%s: %s@%s (%s) is not in the pin list", where, a.Repo, a.SHA, a.Tag)
}

// checkCond checks an if: condition, which is an expression with or
// without ${{ }}.
func checkCond(where, v string, bad func(string, ...any)) {
	if v != "" && !strings.Contains(v, "${{") {
		v = "${{ " + v + " }}"
	}
	checkExpr(where, v, bad)
}

func checkExpr(where, v string, bad func(string, ...any)) {
	for _, m := range exprRe.FindAllStringSubmatch(v, -1) {
		for _, u := range untrusted {
			if strings.Contains(m[1], u) {
				bad("%s: expression uses %s", where, strings.TrimSuffix(u, "."))
			}
		}
	}
	if strings.Count(v, "${{") != len(exprRe.FindAllString(v, -1)) {
		bad("%s: unterminated expression", where)
	}
}

// pipeToShellRe is a pipe (not ||) into a shell, also across a line break,
// through sudo or env, or by path (C-113 RT-3).
var pipeToShellRe = regexp.MustCompile(`(?:^|[^|])\|[\s\\]*(?:sudo\s+(?:-\S+\s+)*)?(?:\S*/)?(?:env\s+)?(?:\S*/)?(?:ba|da|k|z)?sh(?:\s|$|[;&|)])`)

// PipesToShell reports whether script pipes into a shell, as in
// curl … | sh: the downloaded code runs without an integrity check.
func PipesToShell(script string) bool { return pipeToShellRe.MatchString(script) }
