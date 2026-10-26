package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/OI4/oec/spec/tools/internal/catalog"
	"github.com/OI4/oec/spec/tools/internal/payload"
	"github.com/OI4/oec/spec/tools/internal/platform"
	"github.com/OI4/oec/spec/tools/internal/topic"
)

// xcase is one case of oec/schema/dist/cases.json (written by schematest).
type xcase struct {
	ID       string          `json:"id"`
	Schema   string          `json:"schema"`
	Instance json.RawMessage `json:"instance"`
	Valid    bool            `json:"valid"`
	Topic    string          `json:"topic"`
	Mode     string          `json:"mode"`
	Object   string          `json:"object"`
	File     string          `json:"file"`
	Reqs     []string        `json:"reqs"`
}

// cmdCrosscheck runs the oracle on every schema case (vectors and probes) and
// compares the verdicts (ADR-0019 amendment C-041, P2 exit criteria 4 and 5):
//   - a case an entry schema rejects: the oracle reports every REQ the
//     schema reports (the oracle checks every rule a schema enforces);
//   - a case the generic message.schema.json rejects: the oracle rejects it
//     too; the REQs are not compared, the generic schema does not know the
//     topic and dispatches on MessageType and the payload alone;
//   - a case the schema accepts: every REQ the oracle reports carries the
//     verification d7-validate (it is the rule layer's, not the schema's).
func cmdCrosscheck(args []string) error {
	fs, c := flags("crosscheck")
	casesFile := fs.String("cases", "", "cases file of the schema test suite (default: <spec>/../schema/dist/cases.json)")
	verbose := fs.Bool("v", false, "list every case with an oracle-only REQ")
	_ = fs.Parse(args)
	if *casesFile == "" {
		*casesFile = filepath.Join(c.spec, "..", "schema", "dist", "cases.json")
	}
	data, err := os.ReadFile(*casesFile)
	if err != nil {
		return fmt.Errorf("%w (run oec/schema/check.sh first)", err)
	}
	var cases []xcase
	if err := json.Unmarshal(data, &cases); err != nil {
		return fmt.Errorf("%s: %w", *casesFile, err)
	}
	cat, err := catalog.Load(c.spec)
	if err != nil {
		return err
	}
	ruleLayer := map[string]bool{}
	for _, r := range cat.Requirements() {
		if slices.Contains(r.Verification, "d7-validate") {
			ruleLayer[r.ID] = true
		}
	}
	ts, err := topic.Load(c.spec)
	if err != nil {
		return err
	}
	var failures []string
	agree, ruleOnly, extra, generic := 0, 0, 0, 0
	for _, k := range cases {
		got, err := oracleReqs(ts, k)
		if err != nil {
			return fmt.Errorf("%s: %w", k.ID, err)
		}
		switch {
		case !k.Valid && short(k.Schema) == "message.schema.json":
			if len(got) == 0 {
				failures = append(failures, fmt.Sprintf("%s: schema %s reports %v, the oracle accepts", k.ID, short(k.Schema), k.Reqs))
				continue
			}
			if len(minus(k.Reqs, got)) > 0 {
				generic++
				if *verbose {
					fmt.Printf("%s: generic schema %v, oracle %v\n", k.ID, k.Reqs, got)
				}
			}
			agree++
		case !k.Valid:
			if missing := minus(k.Reqs, got); len(missing) > 0 {
				failures = append(failures, fmt.Sprintf("%s: schema %s reports %v, the oracle does not (oracle: %v)", k.ID, short(k.Schema), missing, got))
				continue
			}
			if len(minus(got, k.Reqs)) > 0 {
				extra++
				if *verbose {
					fmt.Printf("%s: schema %v, oracle also %v\n", k.ID, k.Reqs, minus(got, k.Reqs))
				}
			}
			agree++
		case len(got) == 0:
			agree++
		default:
			var notRule []string
			for _, q := range got {
				if !ruleLayer[q] {
					notRule = append(notRule, q)
				}
			}
			if len(notRule) > 0 {
				failures = append(failures, fmt.Sprintf("%s: schema %s accepts, the oracle reports %v without d7-validate", k.ID, short(k.Schema), notRule))
				continue
			}
			ruleOnly++
			if *verbose {
				fmt.Printf("%s: schema accepts, oracle (rule layer) %v\n", k.ID, got)
			}
		}
	}
	for _, f := range failures {
		fmt.Fprintln(os.Stderr, "FAIL", f)
	}
	fmt.Printf("crosscheck: %d cases; %d agree (%d rejected cases where the oracle reports further REQs, %d rejected by the generic schema with other REQs), %d accepted by the schema with rule-layer REQs of the oracle, %d disagreements\n",
		len(cases), agree, extra, generic, ruleOnly, len(failures))
	if len(failures) > 0 {
		return fmt.Errorf("%d disagreement(s) between oracle and schema", len(failures))
	}
	return nil
}

// oracleReqs returns the sorted REQ ids the oracle reports for a case.
func oracleReqs(ts *topic.Spec, k xcase) ([]string, error) {
	doc, err := payload.Parse(k.Instance)
	if err != nil {
		return nil, err
	}
	mode := payload.Strict
	if k.Mode != "" {
		if mode, err = payload.ParseMode(k.Mode); err != nil {
			return nil, err
		}
	}
	var problems []payload.Problem
	switch {
	case k.File == "broker.json":
		problems = platform.CheckBroker(doc)
	case k.Object == "DataSetMetaDataType":
		problems = payload.CheckMetaDataType(doc, mode)
	case k.Topic != "":
		res, err := ts.Analyze(k.Topic)
		if err != nil {
			return nil, err
		}
		if !res.Valid {
			return nil, fmt.Errorf("topic invalid (%s %s)", res.Failure, res.Detail)
		}
		p := res.Parsed
		problems = payload.CheckMode(payload.Topic{
			ServiceType: p.ServiceType, AppIL: p.AppIDIL, Method: p.Method,
			Resource: p.Resource, SourceIL: p.SourceIL, Filter: p.Filter,
		}, doc, mode)
	default:
		return nil, fmt.Errorf("case without topic, object or file")
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range problems {
		if !seen[p.Req] {
			seen[p.Req] = true
			out = append(out, p.Req)
		}
	}
	sort.Strings(out)
	return out, nil
}

// minus returns the elements of a that are not in b.
func minus(a, b []string) []string {
	var out []string
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

func short(id string) string {
	if i := strings.Index(id, "/oec/v2/"); i >= 0 {
		return id[i+len("/oec/v2/"):]
	}
	return id
}
