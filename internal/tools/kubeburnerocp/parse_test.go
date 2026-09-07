package kubeburnerocp

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

func readFixtures(t *testing.T, workload string, names ...string) map[string][]byte {
	t.Helper()
	base := filepath.Join("testdata", workload+"-results")
	data := map[string][]byte{}
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(base, n))
		if err != nil {
			t.Fatalf("read %s: %v", n, err)
		}
		data[n] = b
	}
	return data
}

func p99s(res core.TestResult) map[string]float64 {
	out := map[string]float64{}
	for _, m := range res.Metrics {
		if m.Percentile == "p99" {
			out[m.Name] = m.Value
		}
	}
	return out
}

func TestParseResults_PVCDensity_Golden(t *testing.T) {
	data := readFixtures(t, "pvc-density",
		"jobSummary.json",
		"pvcLatencyQuantilesMeasurement-pvc-density.json",
		"podLatencyQuantilesMeasurement-pvc-density.json")
	res, err := ParseResults("TR-STOR-006", data)
	if err != nil {
		t.Fatalf("ParseResults: %v", err)
	}
	if res.Native != core.OutcomePass {
		t.Errorf("native=%s, want pass", res.Native)
	}
	want := map[string]float64{"pvcLatency_Bound": 1000, "podLatency_Ready": 1000}
	got := p99s(res)
	for name, v := range want {
		if got[name] != v {
			t.Errorf("%s p99=%v, want %v", name, got[name], v)
		}
	}
}

func TestParseResults_PVCDensity_NativeFail(t *testing.T) {
	data := readFixtures(t, "pvc-density",
		"pvcLatencyQuantilesMeasurement-pvc-density.json",
		"podLatencyQuantilesMeasurement-pvc-density.json")
	data["jobSummary.json"] = mustRead(t, "pvc-density", "jobSummary-fail.json")
	res, err := ParseResults("TR-STOR-006", data)
	if err != nil {
		t.Fatalf("ParseResults: %v", err)
	}
	if res.Native != core.OutcomeFail {
		t.Errorf("native=%s, want fail", res.Native)
	}
}

func TestParseResults_VirtParallel_Golden(t *testing.T) {
	data := readFixtures(t, "virt-parallel",
		"jobSummary.json",
		"vmiLatencyQuantilesMeasurement-virt-parallel-create-vms-0.json")
	res, err := ParseResults("TR-VIRT-019", data)
	if err != nil {
		t.Fatalf("ParseResults: %v", err)
	}
	if res.Native != core.OutcomePass {
		t.Errorf("native=%s, want pass", res.Native)
	}
	want := map[string]float64{"vmiLatency_VMReady": 10}
	got := p99s(res)
	for name, v := range want {
		if got[name] != v {
			t.Errorf("%s p99=%v, want %v", name, got[name], v)
		}
	}
}

func mustRead(t *testing.T, workload, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", workload+"-results", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return b
}
