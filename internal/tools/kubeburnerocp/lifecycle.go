package kubeburnerocp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

type virtParallelEvidence struct {
	Outcome    core.Outcome         `json:"outcome"`
	Reason     string               `json:"reason"`
	InitialVMs int                  `json:"initial_vms"`
	Increment  int                  `json:"increment"`
	Iterations int                  `json:"iterations"`
	Expected   int                  `json:"expected_migrations"`
	Completed  int                  `json:"completed_migrations"`
	Created    int                  `json:"created_vms"`
	Observed   []lifecycleIteration `json:"observed_iterations"`
}

// lifecycleIteration preserves diagnostics for observed iterations even when a
// run stops before its configured iteration limit. Counts are distinct VM IDs.
type lifecycleIteration struct {
	Index    int                     `json:"index"`
	Stages   map[string]core.Outcome `json:"stages"`
	Created  int                     `json:"created_vms"`
	Migrated int                     `json:"migrated_vms"`
}

// Individual vmimLatencyMeasurement documents are emitted by kube-burner
// only for migrations that reached Succeeded. Quantiles cannot establish
// either a population count or which test VM actually migrated.
type lifecycleSample struct {
	MetricName       string   `json:"metricName"`
	UUID             string   `json:"uuid"`
	Namespace        string   `json:"namespace"`
	VMIName          string   `json:"vmiName"`
	MigrationName    string   `json:"vmimName"`
	JobName          string   `json:"jobName"`
	SucceededLatency *float64 `json:"succeededLatency"`
}

func (e *virtParallelEvidence) reject(out core.Outcome, reason string) {
	// Missing/malformed evidence takes precedence over an observed failure.
	if e.Outcome == core.OutcomeError || (e.Outcome == core.OutcomeFail && out == core.OutcomeFail) {
		return
	}
	e.Outcome = out
	e.Reason = reason
}

// verifyTRVirt019Lifecycle applies TR-VIRT-019's full-lifecycle certification
// policy: no stage may be skipped, and every created VM must migrate. This is
// deliberately specific to this requirement; other tests may need a different
// stage policy. Missing iterations fail the check while observed evidence is
// still inspected to retain useful creation, migration, and stage diagnostics.
func verifyTRVirt019Lifecycle(summaries []jobSummary, data map[string][]byte) virtParallelEvidence {
	e := virtParallelEvidence{Outcome: core.OutcomePass, Reason: "all lifecycle stages succeeded and every test VM completed migration"}
	jobs := map[string]jobSummary{}
	for _, s := range summaries {
		if _, exists := jobs[s.JobConfig.Name]; exists {
			e.reject(core.OutcomeError, "duplicate lifecycle job summary: "+s.JobConfig.Name)
		}
		jobs[s.JobConfig.Name] = s
	}
	first, ok := jobs["virt-parallel-create-vms-0"]
	if !ok {
		e.reject(core.OutcomeFail, "missing VM creation stage for iteration 0")
		return e
	}
	var err error
	e.InitialVMs, err = strconv.Atoi(first.WorkloadFlags["initialVms"])
	if err != nil || e.InitialVMs <= 0 {
		e.reject(core.OutcomeError, "missing or invalid initialVms in lifecycle workload flags")
		return e
	}
	e.Increment, err = strconv.Atoi(first.WorkloadFlags["increment"])
	if err != nil || e.Increment < 0 {
		e.reject(core.OutcomeError, "missing or invalid increment in lifecycle workload flags")
		return e
	}
	e.Iterations, err = strconv.Atoi(first.WorkloadFlags["maxIterations"])
	if err != nil || e.Iterations <= 0 {
		e.reject(core.OutcomeError, "missing or unbounded maxIterations in lifecycle workload flags")
		return e
	}
	e.Expected, err = virtParallelTotal(e.InitialVMs, e.Increment, e.Iterations)
	if err != nil {
		e.reject(core.OutcomeError, err.Error())
		return e
	}
	// Derive the finite iteration set from summaries instead of looping over an
	// untrusted maxIterations value in the tool output.
	creates := 0
	observed := map[int]bool{}
	for name := range jobs {
		if strings.HasPrefix(name, "virt-parallel-create-vms-") {
			creates++
		}
		if strings.HasPrefix(name, virtParallelJobPrefix) {
			suffix := name[strings.LastIndex(name, "-")+1:]
			if i, err := strconv.Atoi(suffix); err == nil && i >= 0 {
				observed[i] = true
			}
		}
	}
	if creates != e.Iterations {
		e.reject(core.OutcomeFail, fmt.Sprintf("expected %d lifecycle iterations, found %d VM creation stages", e.Iterations, creates))
	}
	indices := make([]int, 0, len(observed))
	for i := range observed {
		indices = append(indices, i)
	}
	sort.Ints(indices)
	for _, i := range indices {
		if i >= e.Iterations {
			e.reject(core.OutcomeError, fmt.Sprintf("unexpected lifecycle iteration: %d", i))
			continue
		}
		e.Observed = append(e.Observed, lifecycleIteration{Index: i, Stages: map[string]core.Outcome{}})
		iteration := &e.Observed[len(e.Observed)-1]
		count := e.InitialVMs + i*e.Increment
		for _, stage := range []string{"create-vms", "resize-volumes", "restart-vms", "snapshot-vms", "migrate-vms", "delete-vms"} {
			name := fmt.Sprintf("virt-parallel-%s-%d", stage, i)
			s, exists := jobs[name]
			iteration.Stages[stage] = core.OutcomePass
			if !exists {
				iteration.Stages[stage] = core.OutcomeFail
				e.reject(core.OutcomeFail, "missing required lifecycle stage: "+name)
				continue
			}
			if !s.Passed || s.ExecutionErrors != "" {
				iteration.Stages[stage] = core.OutcomeFail
				e.reject(core.OutcomeFail, "lifecycle stage failed: "+name)
			}
			if s.UUID == "" || s.UUID != first.UUID {
				iteration.Stages[stage] = core.OutcomeError
				e.reject(core.OutcomeError, "inconsistent lifecycle run UUID: "+name)
			}
			for _, flag := range []string{"initialVms", "increment", "maxIterations"} {
				if s.WorkloadFlags[flag] != first.WorkloadFlags[flag] {
					iteration.Stages[stage] = core.OutcomeError
					e.reject(core.OutcomeError, "inconsistent lifecycle workload flags: "+name)
				}
			}
			for _, flag := range []string{"skipMigrationJob", "skipResizeJob", "skipRestartJob", "skipSnapshotJob"} {
				if s.WorkloadFlags[flag] == "true" {
					if iteration.Stages[stage] != core.OutcomeError {
						iteration.Stages[stage] = core.OutcomeFail
					}
					e.reject(core.OutcomeFail, "required lifecycle stage disabled: "+flag)
				}
			}
		}
		created, err := readLifecycleSamples(data, fmt.Sprintf("virt-parallel-create-vms-%d", i), "vmiLatencyMeasurement", first.UUID)
		if err != nil {
			e.reject(core.OutcomeError, err.Error())
			continue
		}
		guests := map[string]bool{}
		for _, sample := range created {
			guests[sample.Namespace+"/"+sample.VMIName] = true
		}
		iteration.Created = len(guests)
		e.Created += len(guests)
		if len(guests) != count {
			e.reject(core.OutcomeFail, fmt.Sprintf("iteration %d: expected %d distinct test VMs, found %d", i, count, len(guests)))
		}
		migrated, err := readLifecycleSamples(data, fmt.Sprintf("virt-parallel-migrate-vms-%d", i), "vmimLatencyMeasurement", first.UUID)
		if err != nil {
			e.reject(core.OutcomeError, err.Error())
			continue
		}
		completed := map[string]bool{}
		for _, sample := range migrated {
			key := sample.Namespace + "/" + sample.VMIName
			if !guests[key] {
				e.reject(core.OutcomeError, "migration evidence references a VM outside this iteration: "+key)
				continue
			}
			if sample.MigrationName == "" || sample.SucceededLatency == nil || *sample.SucceededLatency < 0 {
				e.reject(core.OutcomeError, "migration evidence has no successful completion: "+key)
				continue
			}
			completed[key] = true
		}
		iteration.Migrated = len(completed)
		e.Completed += len(completed)
		if len(completed) != count {
			e.reject(core.OutcomeFail, fmt.Sprintf("iteration %d: expected %d migrated VMs, observed %d successful distinct VM migrations", i, count, len(completed)))
		}
	}
	return e
}

func readLifecycleSamples(data map[string][]byte, job, metric, uuid string) ([]lifecycleSample, error) {
	var samples []lifecycleSample
	found := false
	for name, blob := range data {
		if filepath.Base(name) != metric+"-"+job+".json" {
			continue
		}
		if found {
			return nil, fmt.Errorf("duplicate evidence for %s", job)
		}
		found = true
		if err := json.Unmarshal(blob, &samples); err != nil {
			return nil, fmt.Errorf("malformed evidence for %s: %w", job, err)
		}
		for _, s := range samples {
			if s.MetricName != metric || s.JobName != job || s.UUID != uuid || s.Namespace == "" || s.VMIName == "" {
				return nil, fmt.Errorf("wrong run, iteration, or VM identity in evidence for %s", job)
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("missing individual measurement evidence for %s", job)
	}
	return samples, nil
}

// virtParallelTotal sums the arithmetic progression of VM populations:
// initial*iterations + increment*iterations*(iterations-1)/2. Divide one factor
// before multiplying, and check each product against the remaining int range
// so even untrusted workload flags cannot overflow the expected migration count.
func virtParallelTotal(initial, increment, iterations int) (int, error) {
	maxInt := int(^uint(0) >> 1)
	if initial <= 0 || increment < 0 || iterations <= 0 || initial > maxInt/iterations {
		return 0, fmt.Errorf("invalid or overflowing lifecycle VM count")
	}
	total := initial * iterations
	if increment == 0 || iterations == 1 {
		return total, nil
	}
	left, right := iterations, iterations-1
	if left%2 == 0 {
		left /= 2
	} else {
		right /= 2
	}
	if right > (maxInt-total)/left {
		return 0, fmt.Errorf("lifecycle VM count overflow")
	}
	triangle := left * right
	if increment > (maxInt-total)/triangle {
		return 0, fmt.Errorf("lifecycle VM count overflow")
	}
	return total + triangle*increment, nil
}
