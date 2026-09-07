package virtbench

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

// update regenerates the golden file: go test ./internal/tools/virtbench -update
var update = flag.Bool("update", false, "update golden files")

// TestParseSummary is the golden-file parser test: feed the virtbench summary
// fixture, parse, compare normalized output to a golden file. No cluster or
// binary required (see decisions/0003, 0006).
func TestParseSummary(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", SummaryFileName))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	got, err := ParseSummary(in, "TR-VIRT-002")
	if err != nil {
		t.Fatalf("ParseSummary: %v", err)
	}
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	goldenPath := filepath.Join("testdata", "parse.golden.json")
	if *update {
		if err := os.WriteFile(goldenPath, append(gotJSON, '\n'), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if strings.TrimSpace(string(gotJSON)) != strings.TrimSpace(string(want)) {
		t.Errorf("parsed output differs from golden:\n got: %s\nwant: %s", gotJSON, want)
	}
}

// TestParseSummaryMetricNamesMatchKB guards the contract that the emitted base
// metric names equal the KB SLA metric names for TR-VIRT-002 — otherwise the
// grader's findMetric can't match the measured value to its SLA.
func TestParseSummaryMetricNamesMatchKB(t *testing.T) {
	in, _ := os.ReadFile(filepath.Join("testdata", SummaryFileName))
	got, err := ParseSummary(in, "TR-VIRT-002")
	if err != nil {
		t.Fatalf("ParseSummary: %v", err)
	}
	names := map[string]bool{}
	for _, m := range got[0].Metrics {
		names[m.Name] = true
	}
	for _, want := range []string{"clone_duration", "time_to_running", "time_to_ping"} {
		if !names[want] {
			t.Errorf("missing SLA-scored metric %q; got %v", want, names)
		}
	}
	if got[0].Native != core.OutcomePass {
		t.Errorf("native = %q, want pass (fixture has 0 failed VMs)", got[0].Native)
	}
}

// goldenParse runs a pure parser against a fixture and compares to a golden file,
// regenerating with -update. It is the shared harness for every scenario parser.
func goldenParse(t *testing.T, parse func([]byte, string) ([]core.TestResult, error), fixture, golden, trID string) {
	t.Helper()
	in, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	got, err := parse(in, trID)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	goldenPath := filepath.Join("testdata", golden)
	if *update {
		if err := os.WriteFile(goldenPath, append(gotJSON, '\n'), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if strings.TrimSpace(string(gotJSON)) != strings.TrimSpace(string(want)) {
		t.Errorf("parsed output differs from golden:\n got: %s\nwant: %s", gotJSON, want)
	}
}

func TestParseBootStorm(t *testing.T) {
	goldenParse(t, ParseBootStorm, BootStormFileName, "parse_bootstorm.golden.json", "TR-VIRT-001")
}

// TestParseBootStormEmitsVMBootTime guards the documented mapping vm_boot_time ←
// total_test_duration_sec (the whole-storm wall-clock the KB gate targets).
func TestParseBootStormEmitsVMBootTime(t *testing.T) {
	in, _ := os.ReadFile(filepath.Join("testdata", BootStormFileName))
	got, err := ParseBootStorm(in, "TR-VIRT-001")
	if err != nil {
		t.Fatalf("ParseBootStorm: %v", err)
	}
	var vmBoot, total *core.Metric
	for i := range got[0].Metrics {
		switch got[0].Metrics[i].Name {
		case "vm_boot_time":
			vmBoot = &got[0].Metrics[i]
		case "total_test_duration":
			total = &got[0].Metrics[i]
		}
	}
	if vmBoot == nil || total == nil {
		t.Fatalf("want both vm_boot_time and total_test_duration; got %+v", got[0].Metrics)
	}
	if vmBoot.Value != total.Value || vmBoot.Unit != "s" {
		t.Errorf("vm_boot_time = %v %s, want %v s", vmBoot.Value, vmBoot.Unit, total.Value)
	}
}

func TestParseDiskOps(t *testing.T) {
	goldenParse(t, ParseDiskOps, DiskOpsFileName, "parse_diskops.golden.json", "TR-STOR-001")
}

// TestParseDiskOpsMetricNamesMatchKB guards the pvc_attach_time mapping.
func TestParseDiskOpsMetricNamesMatchKB(t *testing.T) {
	in, _ := os.ReadFile(filepath.Join("testdata", DiskOpsFileName))
	got, err := ParseDiskOps(in, "TR-STOR-001")
	if err != nil {
		t.Fatalf("ParseDiskOps: %v", err)
	}
	names := map[string]bool{}
	for _, m := range got[0].Metrics {
		names[m.Name] = true
	}
	if !names["pvc_attach_time"] {
		t.Errorf("missing SLA-scored metric pvc_attach_time; got %v", names)
	}
}

func TestParseDiskOpsErrors(t *testing.T) {
	if _, err := ParseDiskOps(nil, "TR-STOR-001"); err == nil {
		t.Error("empty payload: want error, got nil")
	}
	if _, err := ParseDiskOps([]byte(`{"summary":{"total_vms":1}}`), "TR-STOR-001"); err == nil {
		t.Error("no hotplug section: want error, got nil")
	}
}

// TestScenarioEvaluatorNativeFail: a native failure (not every VM succeeded)
// fails the TR on its own line and short-circuits SLA grading — the one piece of
// virtbench-specific scoring that survives ADR-0013 (measurability is now data).
func TestScenarioEvaluatorNativeFail(t *testing.T) {
	ev := scenarioEvaluator{}
	tr := core.TestRequirement{ID: "TR-STOR-001", AutomationTool: "virtbench disk-ops",
		SLAs: []core.SLA{{Metric: "pvc_attach_time", MeasuredBy: []string{"virtbench disk-ops"}}}}
	res := core.TestResult{TRID: "TR-STOR-001", Native: core.OutcomeFail,
		Metrics: []core.Metric{{Name: "vms_failed", Value: 2}, {Name: "vms_total", Value: 10}}}
	got, err := ev.Evaluate(context.Background(), tr, res)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(got) != 1 || got[0].Item != "native:all_vms_succeeded" || got[0].Outcome != core.OutcomeFail {
		t.Fatalf("native failure should yield a single fail verdict, got %+v", got)
	}
}

func TestParseSummaryErrors(t *testing.T) {
	if _, err := ParseSummary(nil, "TR-VIRT-002"); err == nil {
		t.Error("empty payload: want error, got nil")
	}
	if _, err := ParseSummary([]byte(`{"total_vms":0,"metrics":[]}`), "TR-VIRT-002"); err == nil {
		t.Error("zero VMs: want error, got nil")
	}
}

// TestBuildArgs verifies the CLI is assembled from params/backend only.
func TestBuildArgs(t *testing.T) {
	tr := core.TestRequirement{
		ID: "TR-VIRT-002",
		Params: map[string]any{
			"end":              100,
			"vm-name":          "ignored", // wrong key, ensure default used
			"vm_name":          "vb",
			"namespace_prefix": "vbench",
			"storage_class":    "ocs-storagecluster-ceph-rbd",
			"storage_driver":   "csi",
		},
	}
	got, err := datasourceCloneArgs(false, "virtbench")(&core.RunCtx{}, tr, "/work/res")
	if err != nil {
		t.Fatalf("buildArgs: %v", err)
	}
	want := []string{
		"datasource-clone",
		"--start", "1",
		"--end", "100",
		"--vm-name", "vb",
		"--namespace-prefix", "vbench",
		"--storage-class", "ocs-storagecluster-ceph-rbd",
		"--save-results",
		"--results-folder", "/work/res",
		"--storage-driver", "csi",
		"--cleanup", "--cleanup-on-failure", "--yes",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildArgs =\n  %v\nwant\n  %v", got, want)
	}
}

// TestBuildArgsVMNameDefault: with no vm_name param, the default MUST be the
// bundled template's VM name (rhel-9-vm) — otherwise virtbench's Running poll
// never matches the VM it created and the run hangs (the live-run bug).
func TestBuildArgsVMNameDefault(t *testing.T) {
	tr := core.TestRequirement{ID: "TR-VIRT-001", Params: map[string]any{"storage_class": "sc"}}
	got, err := datasourceCloneArgs(true, "virtbench")(&core.RunCtx{}, tr, "/work/res")
	if err != nil {
		t.Fatalf("buildArgs: %v", err)
	}
	if !strings.Contains(strings.Join(got, " "), "--vm-name rhel-9-vm") {
		t.Errorf("default vm-name must be rhel-9-vm; got %v", got)
	}
}

// TestBuildArgsBootStorm: the boot-storm scenario is the same subcommand plus the
// --boot-storm flag — a new table row, no engine change.
func TestBuildArgsBootStorm(t *testing.T) {
	tr := core.TestRequirement{ID: "TR-VIRT-001", Params: map[string]any{
		"end":           100,
		"storage_class": "ocs-storagecluster-ceph-rbd",
	}}
	got, err := datasourceCloneArgs(true, "virtbench")(&core.RunCtx{}, tr, "/work/res")
	if err != nil {
		t.Fatalf("buildArgs: %v", err)
	}
	joined := strings.Join(got, " ")
	if !strings.HasPrefix(joined, "datasource-clone ") {
		t.Errorf("boot storm must use datasource-clone subcommand; got %q", joined)
	}
	for _, want := range []string{"--boot-storm", "--results-folder /work/res", "--cleanup"} {
		if !strings.Contains(joined, want) {
			t.Errorf("boot storm args missing %q; got %q", want, joined)
		}
	}
}

// TestBuildArgsDiskOps: disk-ops is a different subcommand with its own flags
// (note --results-dir, not --results-folder).
func TestBuildArgsDiskOps(t *testing.T) {
	tr := core.TestRequirement{ID: "TR-STOR-001", Params: map[string]any{
		"end":           50,
		"storage_class": "ocs-storagecluster-ceph-rbd",
	}}
	got, err := diskOpsArgs("disk-ops")(&core.RunCtx{}, tr, "/work/res")
	if err != nil {
		t.Fatalf("buildArgs: %v", err)
	}
	joined := strings.Join(got, " ")
	if !strings.HasPrefix(joined, "disk-ops ") {
		t.Errorf("disk-ops subcommand expected; got %q", joined)
	}
	for _, want := range []string{"--operation hotplug", "--end 50", "--storage-class ocs-storagecluster-ceph-rbd", "--results-dir /work/res", "--create-vms"} {
		if !strings.Contains(joined, want) {
			t.Errorf("disk-ops args missing %q; got %q", want, joined)
		}
	}
}

func TestBuildArgsRequiresStorageClass(t *testing.T) {
	if _, err := datasourceCloneArgs(false, "virtbench")(&core.RunCtx{}, core.TestRequirement{ID: "TR-VIRT-002"}, "/work/res"); err == nil {
		t.Error("no storage class: want error, got nil")
	}
	if _, err := diskOpsArgs("disk-ops")(&core.RunCtx{}, core.TestRequirement{ID: "TR-STOR-001"}, "/work/res"); err == nil {
		t.Error("disk-ops no storage class: want error, got nil")
	}
}

func TestBuildArgsBackendStorageClassWins(t *testing.T) {
	tr := core.TestRequirement{ID: "TR-VIRT-002", Params: map[string]any{"storage_class": "from-param"}}
	rc := &core.RunCtx{Backend: &core.ResolvedBackend{StorageClass: "from-backend"}}
	got, err := datasourceCloneArgs(false, "virtbench")(rc, tr, "/work/res")
	if err != nil {
		t.Fatalf("buildArgs: %v", err)
	}
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "--storage-class from-backend") {
		t.Errorf("backend storage class should win; got %q", joined)
	}
}

func TestReplayDir(t *testing.T) {
	trs := []core.TestRequirement{
		{ID: "TR-VIRT-002", Params: map[string]any{"results_dir": "/tmp/x"}},
	}
	if d := replayDir(trs); d != "/tmp/x" {
		t.Errorf("replayDir = %q, want /tmp/x", d)
	}
	if d := replayDir([]core.TestRequirement{{ID: "TR-VIRT-002"}}); d != "" {
		t.Errorf("replayDir = %q, want empty", d)
	}
}
