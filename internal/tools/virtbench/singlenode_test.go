package virtbench

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

func TestParseSingleNodeCeiling(t *testing.T) {
	goldenParse(t, ParseSingleNodeCeiling, "summary_single_node_results.json", "parse_single_node.golden.json", "TR-VIRT-013")
}

// TestParseSingleNodeCeilingNativePassDespiteFailures guards the behavior that
// sets this scenario apart from every other summary-based one: the fixture has
// 38 failed VMs (expected — the run pushed past the node's ceiling), yet
// Native must stay pass so a real capacity shortfall is only ever reported via
// the max_vms_per_node SLA bar, never as a native run failure.
func TestParseSingleNodeCeilingNativePassDespiteFailures(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "summary_single_node_results.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	got, err := ParseSingleNodeCeiling(in, "TR-VIRT-013")
	if err != nil {
		t.Fatalf("ParseSingleNodeCeiling: %v", err)
	}
	if got[0].Native != core.OutcomePass {
		t.Errorf("native = %q, want pass (partial failure is the expected outcome)", got[0].Native)
	}
	if v := metricValue(got[0].Metrics, "max_vms_per_node"); v != 62 {
		t.Errorf("max_vms_per_node = %v, want 62 (the fixture's successful count)", v)
	}
}

// tr013SLAs and tr013Bars mirror the real catalog/thresholds entries for
// TR-VIRT-013: a real >= 50 bar for max_vms_per_node, and no bar at all for
// max_luns_per_node (it is published, never gated).
func tr013SLAs() []core.SLA {
	measuredBy := []string{"virtbench datasource-clone --single-node"}
	return []core.SLA{
		{Metric: "max_vms_per_node", Type: "count", Unit: "count", MeasuredBy: measuredBy},
		{Metric: "max_luns_per_node", Type: "count", Unit: "count", MeasuredBy: measuredBy},
	}
}

func tr013Bars() []core.SLA {
	v := 50.0
	return []core.SLA{
		{Metric: "max_vms_per_node", Operator: ">=", Value: &v, Unit: "count"},
	}
}

// singleNodeEvaluator returns the exact evaluator the single-node scenario
// registers: the default scenarioEvaluator with the max_luns_per_node
// DeriveMetrics hook wired in.
func singleNodeEvaluator() scenarioEvaluator {
	return scenarioEvaluator{sc: Scenario{
		AutomationTool: "virtbench datasource-clone --single-node",
		DeriveMetrics:  singleNodeLunMetric,
	}}
}

// TestSingleNodeCeilingComputesLuns checks the one piece of custom logic the
// scenario adds on top of the standard grader: the DeriveMetrics hook derives
// max_luns_per_node from num_disks × the achieved VM count, then the default
// evaluator grades both metrics with the ordinary rules (no custom pass/fail).
func TestSingleNodeCeilingComputesLuns(t *testing.T) {
	tr := core.TestRequirement{
		ID:             "TR-VIRT-013",
		AutomationTool: "virtbench datasource-clone --single-node",
		SLAs:           tr013SLAs(),
		Bars:           tr013Bars(),
		Params:         map[string]any{"num_disks": 2},
	}
	res := core.TestResult{TRID: tr.ID, Native: core.OutcomePass, Metrics: []core.Metric{
		{Name: "max_vms_per_node", Value: 62, Unit: "count"},
	}}

	verdicts, err := singleNodeEvaluator().Evaluate(context.Background(), tr, res)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	byItem := map[string]core.Verdict{}
	for _, v := range verdicts {
		byItem[v.Item] = v
	}

	if v, ok := byItem["sla:max_vms_per_node"]; !ok || v.Outcome != core.OutcomePass {
		t.Errorf("sla:max_vms_per_node = %+v, want pass (62 >= 50)", v)
	}
	v, ok := byItem["sla:max_luns_per_node"]
	if !ok || v.Outcome != core.OutcomeSkip {
		t.Fatalf("sla:max_luns_per_node = %+v, want a report-only skip (no threshold bar)", v)
	}
	if v.Actual != "124 count" {
		t.Errorf("sla:max_luns_per_node Actual = %q, want %q (62 VMs x 2 disks)", v.Actual, "124 count")
	}
}

// TestSingleNodeCeilingBelowThresholdFails proves there's no special-casing
// hiding a real shortfall: a low VM count fails max_vms_per_node exactly like
// any other SLA compared against its bar.
func TestSingleNodeCeilingBelowThresholdFails(t *testing.T) {
	tr := core.TestRequirement{
		ID:             "TR-VIRT-013",
		AutomationTool: "virtbench datasource-clone --single-node",
		SLAs:           tr013SLAs(),
		Bars:           tr013Bars(),
	}
	res := core.TestResult{TRID: tr.ID, Native: core.OutcomePass, Metrics: []core.Metric{
		{Name: "max_vms_per_node", Value: 3, Unit: "count"},
	}}

	verdicts, err := singleNodeEvaluator().Evaluate(context.Background(), tr, res)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for _, v := range verdicts {
		if v.Item == "sla:max_vms_per_node" {
			if v.Outcome != core.OutcomeFail {
				t.Errorf("sla:max_vms_per_node = %+v, want fail (3 < 50)", v)
			}
			return
		}
	}
	t.Fatal("no sla:max_vms_per_node verdict produced")
}
