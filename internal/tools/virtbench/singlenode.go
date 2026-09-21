// Package virtbench
// TR-VIRT-013 (Per-Node Scale Ceiling) rides the generic engine like every
// other virtbench scenario, but needs its own BuildArgs: unlike
// datasourceCloneArgs' peers (TR-001/002/018), it always passes --single-node,
// optionally pins a --node-name, and scales via the attempt_vms param instead
// of num_vms/iteration_clones. Grading needs a small custom Evaluator too,
// because max_luns_per_node (successful VMs × num_disks) isn't in virtbench's
// output and num_disks is a TR param the Parser has no access to.
package virtbench

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

const (
	singleNodeNSPrefix     = "virtbench-single-node"
	singleNodeDefaultCount = 100 // TR-VIRT-013 catalog default for attempt_vms
)

// --- BuildArgs -----------------------------------------------------------

// singleNodeCeilingArgs builds the CLI for the single-node ceiling scenario:
// the same datasource-clone subcommand as TR-001/002/018, plus --single-node
// (always) and --node-name (when pinned). Kept separate from
// datasourceCloneArgs rather than adding a fourth override branch there, so
// the three scenarios already depending on it are untouched. Reads only
// params/backend, never a threshold.
func singleNodeCeilingArgs(nsPrefix string) func(rc *core.RunCtx, tr core.TestRequirement, resultsRoot string) ([]string, error) {
	return func(rc *core.RunCtx, tr core.TestRequirement, resultsRoot string) ([]string, error) {
		sc := storageClass(rc, tr)
		if sc == "" {
			return nil, fmt.Errorf("virtbench: %s: storage_class required (backend or 'storage_class' param)", tr.ID)
		}
		start := intParam(tr, "start", 1)
		end := intParam(tr, "attempt_vms", singleNodeDefaultCount)

		args := clusterArgs(tr)
		args = append(args, "datasource-clone",
			"--start", strconv.Itoa(start),
			"--end", strconv.Itoa(end),
			"--vm-name", strParamOr(tr, "vm_name", defaultVMName),
			"--namespace-prefix", strParamOr(tr, "namespace_prefix", nsPrefix),
			"--storage-class", sc,
			"--single-node",
			"--save-results",
			"--results-folder", resultsRoot,
		)
		if node := strParamOr(tr, "node_name", ""); node != "" {
			args = append(args, "--node-name", node)
		}
		if driver := strParamOr(tr, "storage_driver", ""); driver != "" {
			args = append(args, "--storage-driver", driver)
		}
		if batch := intParam(tr, "namespace_batch_size", 0); batch > 0 {
			args = append(args, "--namespace-batch-size", strconv.Itoa(batch))
		}
		if disks := intParam(tr, "num_disks", 0); disks > 0 {
			args = append(args, "--num-disks", strconv.Itoa(disks))
		}
		// Let virtbench remove the namespaces/VMs it created, on success and on its
		// own failures (a failure partway through is the expected outcome here);
		// --yes skips the interactive confirmation.
		args = append(args, "--cleanup", "--cleanup-on-failure", "--yes")
		return args, nil
	}
}

// --- Validate --------------------------------------------------------------

// singleNodeCeilingValidate checks the scale params before setup creates
// cluster resources.
func singleNodeCeilingValidate(tr core.TestRequirement) error {
	if n := intParam(tr, "attempt_vms", singleNodeDefaultCount); n < 1 {
		return fmt.Errorf("virtbench: %s: attempt_vms must be >= 1", tr.ID)
	}
	if n := intParam(tr, "num_disks", 1); n < 1 {
		return fmt.Errorf("virtbench: %s: num_disks must be >= 1", tr.ID)
	}
	return nil
}

// --- Preflight hook ----------------------------------------------------------

// singleNodeCeilingPreflight adds one check on top of the generic ones
// (binary, storage_class): when node_name is pinned, it must name a real,
// schedulable worker (listWorkers is shared with the drain scenario,
// drain.go). Left unpinned, virtbench itself picks a random worker.
func singleNodeCeilingPreflight(ctx context.Context, _ *core.RunCtx, trs []core.TestRequirement) []core.Finding {
	var findings []core.Finding
	for _, tr := range trs {
		node := strParamOr(tr, "node_name", "")
		if node == "" {
			findings = append(findings, core.Finding{Level: "info", Message: fmt.Sprintf("virtbench: %s: no node_name pinned; virtbench will select a random worker", tr.ID)})
			continue
		}
		schedulable, _, err := listWorkers(ctx, kubeconfigOf(trs))
		if err != nil {
			findings = append(findings, core.Finding{Level: "error", Message: fmt.Sprintf("virtbench: cannot list worker nodes: %v", err)})
			continue
		}
		if !slices.Contains(schedulable, node) {
			findings = append(findings, core.Finding{Level: "error", Message: fmt.Sprintf("virtbench: %s: node_name %q is not a schedulable worker", tr.ID, node)})
		}
	}
	return findings
}

// --- DeriveMetrics -----------------------------------------------------------

// singleNodeLunMetric derives max_luns_per_node = successful VMs × num_disks.
// virtbench doesn't report it and num_disks is a TR param the parser never
// sees, so the default evaluator injects it before grading. It has no
// threshold bar, so the grader publishes it as a report-only value; the
// ceiling itself (max_vms_per_node) is graded against its real >= 50 bar like
// any other SLA — no custom pass/fail logic anywhere.
func singleNodeLunMetric(tr core.TestRequirement, res core.TestResult) []core.Metric {
	disks := intParam(tr, "num_disks", 1)
	vms := vmCount(res, "max_vms_per_node")
	return []core.Metric{{Name: "max_luns_per_node", Value: float64(vms * disks), Unit: "count"}}
}
