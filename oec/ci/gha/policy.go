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
	// Secrets maps each allowed secret to the only place it may appear:
	// the with key of a step that uses the given action repository.
	Secrets map[string]SecretUse
	// MaxTimeout is the upper bound of timeout-minutes of a job.
	MaxTimeout int
}

// SecretUse is the only place a secret may be used.
type SecretUse struct{ ActionRepo, WithKey string }

var (
	fileRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.yml$`)
	idRe     = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	shaRe    = regexp.MustCompile(`^[0-9a-f]{40}$`)
	secretRe = regexp.MustCompile(`secrets\.([A-Za-z_][A-Za-z0-9_]*)`)
	exprRe   = regexp.MustCompile(`\$\{\{(.*?)\}\}`)
	tagRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/*-]*$`)
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
	if len(w.Permissions) != 1 || w.Permissions[0] != (KV{"contents", "read"}) {
		bad("permissions must be exactly contents: read, got %v", w.Permissions)
	}
	if w.Concurrency.Group == "" || !w.Concurrency.CancelInProgress {
		bad("concurrency needs a group and cancel-in-progress")
	}
	checkValues := func(where string, kvs []KV, step *Step) {
		for _, kv := range kvs {
			checkExpr(where+" "+kv.Key, kv.Value, bad)
			for _, m := range secretRe.FindAllStringSubmatch(kv.Value, -1) {
				use, ok := p.Secrets[m[1]]
				if !ok {
					bad("%s: secret %s is not allowed", where, m[1])
					continue
				}
				if step == nil || step.Uses.Repo != use.ActionRepo || kv.Key != use.WithKey {
					bad("%s: secret %s only in with.%s of %s", where, m[1], use.WithKey, use.ActionRepo)
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
			checkExpr(where+" if", s.If, bad)
			checkValues(where+" env", s.Env, nil)
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
			default:
				checkAction(where, s.Uses, p.Actions, bad)
				checkValues(where+" with", s.With, s)
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
