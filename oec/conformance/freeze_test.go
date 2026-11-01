package conformance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// frozen are the SHA-256 digests (line ends as LF) of the frozen report
// schemas: report version 1 of oec-check since the P5 exit (C-069) and of
// oec-infra since the P5b exit (C-075). A change of a key or of its
// meaning – and so any change of a frozen schema – is a new report version
// with a new schema (ADR-0025 amendment C-075).
var frozen = []struct {
	name   string
	schema []byte
	sha256 string
}{
	{"report.schema.json (oec-check, report 1)", ReportSchema, "6b9a8c256e9677f428b0e60f7bf76753905f010695c5ccd2b0ddfb5a194684b8"},
	{"infra-report.schema.json (oec-infra, report 1)", InfraReportSchema, "7a81091c913ab0d8a7bb1765428c80c28fedc351a5b432566091352bc50b18ab"},
}

func TestReportFreeze(t *testing.T) {
	for _, f := range frozen {
		sum := sha256.Sum256(bytes.ReplaceAll(f.schema, []byte("\r\n"), []byte("\n")))
		if got := hex.EncodeToString(sum[:]); got != f.sha256 {
			t.Errorf("%s is frozen, but its digest is %s: a change needs a new report version and schema (ADR-0025 amendment C-075)", f.name, got)
		}
	}
}
