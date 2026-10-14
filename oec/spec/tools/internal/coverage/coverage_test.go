package coverage

import (
	"strings"
	"testing"

	"github.com/OI4/oec/spec/tools/internal/catalog"
	"github.com/OI4/oec/spec/tools/internal/guideline"
	"github.com/OI4/oec/spec/tools/internal/inventory"
)

func TestCompute(t *testing.T) {
	s := &catalog.Spec{
		Areas: catalog.Areas{Baseline: "abc1234", Areas: []catalog.Area{
			{Code: "TOP", Status: "drafted", Sections: []string{"MG 7"}},
			{Code: "CMP", Status: "planned", Sections: []string{"MG 7.2"}},
		}},
		Files:       []catalog.AreaFile{{Area: "TOP", Requirements: []catalog.Requirement{{ID: "REQ-TOP-001", Inventory: []string{"k1"}}}}},
		Informative: catalog.Informative{Items: []catalog.InformativeItem{{Key: "k2", Reason: "example"}}},
	}
	inv := &inventory.File{Baseline: "abc1234", Items: []guideline.Item{
		{Key: "k1", Section: "MG 7.1"}, {Key: "k2", Section: "MG 7.1.4"}, {Key: "k3", Section: "MG 7.1"},
		{Key: "k4", Section: "MG 7.2.1"}, {Key: "k5", Section: "MG 8"},
	}}
	r := Compute(s, inv)
	top, cmp := r.Areas[0], r.Areas[1]
	if top.Total != 3 || top.Covered != 1 || top.Informative != 1 || len(top.Open) != 1 {
		t.Fatalf("TOP stats wrong: %+v", top)
	}
	if cmp.Total != 1 || len(cmp.Open) != 1 {
		t.Fatalf("CMP stats wrong: %+v", cmp)
	}
	if len(r.Unassigned) != 1 || len(r.Problems) != 2 {
		t.Fatalf("expected unassigned MG 8 and open drafted TOP, got %v", r.Problems)
	}
	if !strings.Contains(r.Report(), "## Open items – TOP") {
		t.Fatal("report lacks open items of the drafted area")
	}
}
