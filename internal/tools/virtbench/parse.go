// This file holds the output-ingest adapters: turning virtbench's result JSON
// into normalized core.Metrics whose names match the KB SLA metrics for the TR
// under test. Each parser is pure so it is golden-testable with no cluster or
// binary (the pattern every ResultParser follows — decisions/0003, 0006).
//
// Two output shapes exist (decisions/0008):
//   - the shared "summary" shape (datasource-clone, its --boot-storm variant,
//     migration): parseSummary, keyed by a per-scenario metric map.
//   - the disk-ops nested shape: ParseDiskOps.
package virtbench

import (
	"encoding/json"
	"fmt"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

// summaryFile is the on-disk shape virtbench writes for the datasource-clone
// family (utils/common.py save_results). Averages may be null when a phase
// produced no samples, so aggregate values are pointers.
type summaryFile struct {
	TotalVMs          int             `json:"total_vms"`
	Successful        int             `json:"successful"`
	Failed            int             `json:"failed"`
	TotalTestDuration *float64        `json:"total_test_duration_sec"`
	Metrics           []summaryMetric `json:"metrics"`
}

type summaryMetric struct {
	Metric string   `json:"metric"`
	Avg    *float64 `json:"avg"`
	Max    *float64 `json:"max"`
	Min    *float64 `json:"min"`
	Count  int      `json:"count"`
}

// metric-map + emit-order for datasource-clone. These names MUST match
// TR-VIRT-002's sla[].metric exactly or the grader can't find the measured value.
var (
	cloneMetricBase = map[string]string{
		"clone_duration_sec": "clone_duration",
		"running_time_sec":   "time_to_running",
		"ping_time_sec":      "time_to_ping",
	}
	cloneMetricOrder = []string{"clone_duration_sec", "running_time_sec", "ping_time_sec"}
)

// metric-map + emit-order for the boot-storm variant. It reports no clone phase;
// its gate (vm_boot_time) is derived from the whole-storm duration in
// ParseBootStorm below.
var (
	bootStormBase = map[string]string{
		"running_time_sec": "time_to_running",
		"ping_time_sec":    "time_to_ping",
	}
	bootStormOrder = []string{"running_time_sec", "ping_time_sec"}
)

// ParseSummary converts a virtbench datasource-clone summary into one normalized
// TestResult for trID.
func ParseSummary(data []byte, trID string) ([]core.TestResult, error) {
	return parseSummary(cloneMetricBase, cloneMetricOrder, data, trID)
}

// ParseBootStorm parses the boot-storm summary and, on top of the shared summary
// metrics, exposes vm_boot_time ← total_test_duration_sec: the KB gate is "100
// VMs booted and SSH-ready in < N min", i.e. the whole storm's wall-clock. The
// grader converts s→min/h (see internal/grader/units.go), so vm_boot_time keeps
// unit "s" here (decisions/0008).
func ParseBootStorm(data []byte, trID string) ([]core.TestResult, error) {
	res, err := parseSummary(bootStormBase, bootStormOrder, data, trID)
	if err != nil {
		return nil, err
	}
	for _, m := range res[0].Metrics {
		if m.Name == "total_test_duration" {
			res[0].Metrics = append(res[0].Metrics, core.Metric{Name: "vm_boot_time", Value: m.Value, Unit: "s"})
			break
		}
	}
	return res, nil
}

// parseSummary is the shared summary parser. Averages become SLA-scored metrics
// (unit "s"); max/min and run-level counts are attached as informative metrics.
// Native is fail if any VM failed.
func parseSummary(metricBase map[string]string, metricOrder []string, data []byte, trID string) ([]core.TestResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("virtbench: empty summary payload")
	}
	var s summaryFile
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("virtbench: parse summary: %w", err)
	}
	if s.TotalVMs == 0 {
		return nil, fmt.Errorf("virtbench: summary reports 0 VMs (no data to grade)")
	}

	byName := make(map[string]summaryMetric, len(s.Metrics))
	for _, m := range s.Metrics {
		byName[m.Metric] = m
	}

	var metrics []core.Metric
	for _, key := range metricOrder {
		m, ok := byName[key]
		if !ok {
			continue
		}
		base := metricBase[key]
		if m.Avg != nil {
			metrics = append(metrics, core.Metric{Name: base, Value: *m.Avg, Unit: "s"})
		}
		if m.Max != nil {
			metrics = append(metrics, core.Metric{Name: base + "_max", Value: *m.Max, Unit: "s"})
		}
		if m.Min != nil {
			metrics = append(metrics, core.Metric{Name: base + "_min", Value: *m.Min, Unit: "s"})
		}
	}

	// Run-level metrics (informative; available for a TR that wants a success-rate
	// gate).
	metrics = append(metrics,
		core.Metric{Name: "vms_total", Value: float64(s.TotalVMs)},
		core.Metric{Name: "vms_successful", Value: float64(s.Successful)},
		core.Metric{Name: "vms_failed", Value: float64(s.Failed)},
		core.Metric{Name: "vm_success_rate", Value: float64(s.Successful) / float64(s.TotalVMs) * 100, Unit: "%"},
	)
	if s.TotalTestDuration != nil {
		metrics = append(metrics, core.Metric{Name: "total_test_duration", Value: *s.TotalTestDuration, Unit: "s"})
	}

	native := core.OutcomePass
	if s.Failed > 0 {
		native = core.OutcomeFail
	}

	return []core.TestResult{{
		TRID:    trID,
		Metrics: metrics,
		Native:  native,
	}}, nil
}

// --- disk-ops (Parser B) -----------------------------------------------------

// diskOpsFile is the disk-ops nested shape (disk-ops-benchmark/measure-disk-ops.py
// save_results) — a DIFFERENT shape from the summary family: per-operation
// aggregates under named keys. We grade the hotplug attach path.
type diskOpsFile struct {
	Summary struct {
		TotalVMs        int `json:"total_vms"`
		TotalOperations int `json:"total_operations"`
	} `json:"summary"`
	Hotplug *diskOpAgg `json:"hotplug"`
}

type diskOpAgg struct {
	Successful       int      `json:"successful"`
	Failed           int      `json:"failed"`
	AvgAPIAttachTime *float64 `json:"avg_api_attach_time"`
	MaxAPIAttachTime *float64 `json:"max_api_attach_time"`
	MinAPIAttachTime *float64 `json:"min_api_attach_time"`
	AvgVolumeReady   *float64 `json:"avg_volume_ready_time"`
	AvgValidation    *float64 `json:"avg_validation_time"`
}

// ParseDiskOps converts a virtbench disk-ops result into one normalized
// TestResult. It maps hotplug's average API attach time to pvc_attach_time (the
// TR-STOR-001 gate) and exposes volume-ready/validation timings plus counts as
// informative metrics. Native is fail if any hotplug operation failed.
func ParseDiskOps(data []byte, trID string) ([]core.TestResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("virtbench: empty disk-ops payload")
	}
	var d diskOpsFile
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("virtbench: parse disk-ops: %w", err)
	}
	if d.Hotplug == nil {
		return nil, fmt.Errorf("virtbench: disk-ops results have no hotplug section (no attach data to grade)")
	}
	h := d.Hotplug

	var metrics []core.Metric
	if h.AvgAPIAttachTime != nil {
		metrics = append(metrics, core.Metric{Name: "pvc_attach_time", Value: *h.AvgAPIAttachTime, Unit: "s"})
	}
	if h.MaxAPIAttachTime != nil {
		metrics = append(metrics, core.Metric{Name: "pvc_attach_time_max", Value: *h.MaxAPIAttachTime, Unit: "s"})
	}
	if h.MinAPIAttachTime != nil {
		metrics = append(metrics, core.Metric{Name: "pvc_attach_time_min", Value: *h.MinAPIAttachTime, Unit: "s"})
	}
	if h.AvgVolumeReady != nil {
		metrics = append(metrics, core.Metric{Name: "volume_ready_time", Value: *h.AvgVolumeReady, Unit: "s"})
	}
	if h.AvgValidation != nil {
		metrics = append(metrics, core.Metric{Name: "validation_time", Value: *h.AvgValidation, Unit: "s"})
	}

	total := h.Successful + h.Failed
	metrics = append(metrics,
		core.Metric{Name: "ops_total", Value: float64(total)},
		core.Metric{Name: "ops_successful", Value: float64(h.Successful)},
		core.Metric{Name: "ops_failed", Value: float64(h.Failed)},
	)
	if total > 0 {
		metrics = append(metrics, core.Metric{Name: "attach_success_rate", Value: float64(h.Successful) / float64(total) * 100, Unit: "%"})
	}

	if len(metrics) == 0 {
		return nil, fmt.Errorf("virtbench: disk-ops hotplug section has no timing data")
	}

	native := core.OutcomePass
	if h.Failed > 0 {
		native = core.OutcomeFail
	}

	return []core.TestResult{{
		TRID:    trID,
		Metrics: metrics,
		Native:  native,
	}}, nil
}
