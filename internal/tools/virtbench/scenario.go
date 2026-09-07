package virtbench

import (
	"context"
	"fmt"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/grader"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/registry"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/stages"
)

// Scenario is the per-subcommand config that the generic engine (engine.go) is
// parameterized over. Everything that differs between virtbench scenarios lives
// here; everything that is the same lives in the engine. Adding a scenario is a
// new row in scenarios below — no engine edits (decisions/0008).
type Scenario struct {
	// AutomationTool is the KB automation_tool string verbatim; it is the registry
	// key so orchestrator's registry.Get(tr.AutomationTool) resolves this scenario.
	AutomationTool string
	// ProvidesTR is the single TR id this scenario runs.
	ProvidesTR string
	// ResultFile is the JSON file virtbench writes for this scenario.
	ResultFile string
	// NSPrefix is the default namespace prefix virtbench uses for this scenario's
	// test namespaces (overridable per-TR via the "namespace_prefix" param). Used
	// both to build the CLI and by Teardown's leftover-namespace sweep.
	NSPrefix string
	// BuildArgs assembles the CLI from params/backend only (never a threshold).
	// resultsRoot is the harness workdir virtbench must write results into, passed
	// via the scenario's results flag so the collector can find the output.
	BuildArgs func(rc *core.RunCtx, tr core.TestRequirement, resultsRoot string) ([]string, error)
	// Parse turns the result file into normalized TestResults for the TR.
	Parse func(data []byte, trID string) ([]core.TestResult, error)
}

// Result file names virtbench writes with --save-results (see the tool's
// save_results()). Each scenario has its own; SummaryFileName is kept exported for
// the existing datasource-clone golden test.
const (
	SummaryFileName   = "summary_vm_creation_results.json"
	BootStormFileName = "summary_boot_storm_results.json"
	DiskOpsFileName   = "disk_ops_results.json"
)

// scenarios is the registry-of-scenarios. Each becomes one ToolIntegration.
var scenarios = []Scenario{
	{
		AutomationTool: "virtbench datasource-clone",
		ProvidesTR:     "TR-VIRT-002",
		ResultFile:     SummaryFileName,
		NSPrefix:       "virtbench",
		BuildArgs:      datasourceCloneArgs(false, "virtbench"),
		Parse:          ParseSummary,
		// all SLAs (clone_duration / time_to_running / time_to_ping) are measured.
	},
	{
		AutomationTool: "virtbench datasource-clone --boot-storm",
		ProvidesTR:     "TR-VIRT-001",
		ResultFile:     BootStormFileName,
		NSPrefix:       "virtbench",
		BuildArgs:      datasourceCloneArgs(true, "virtbench"),
		Parse:          ParseBootStorm,
	},
	{
		// VDI Ready badge (TR-VIRT-018): clone VMs at scale with 2 disks each. Same
		// datasource-clone engine/parser as TR-VIRT-002; the scale (count), the
		// 2-disk template, and num_disks are plan inputs. Only gate: clone_duration.
		AutomationTool: "virtbench datasource-clone --num-disks 2",
		ProvidesTR:     "TR-VIRT-018",
		ResultFile:     SummaryFileName,
		NSPrefix:       "virtbench",
		BuildArgs:      datasourceCloneArgs(false, "virtbench"),
		Parse:          ParseSummary,
	},
	{
		AutomationTool: "virtbench disk-ops",
		ProvidesTR:     "TR-STOR-001",
		ResultFile:     DiskOpsFileName,
		NSPrefix:       "disk-ops",
		BuildArgs:      diskOpsArgs("disk-ops"),
		Parse:          ParseDiskOps,
	},
}

// setupGroup is the shared-setup group key every virtbench scenario belongs to.
// Its provider (sshPodSetup, setup.go) ensures the single ssh helper pod once for
// a whole batch of virtbench tests rather than once per scenario (decisions/0008).
const setupGroup = "virtbench"

func init() {
	registry.RegisterSetup(&sshPodSetup{})
	for _, sc := range scenarios {
		registry.Register(stages.ToolIntegration{
			Name:         sc.AutomationTool,
			Image:        "", // no upstream image; phase-1 local-exec (decisions/0007)
			Provides:     []string{sc.ProvidesTR},
			Preflight:    preflight{sc},
			Provisioner:  provisioner{sc},
			Runner:       runner{sc},
			LogCollector: collector{sc},
			ResultParser: parser{sc},
			Teardown:     teardown{sc},

			// virtbench drives a whole cluster's storage/virt stack at scale; never
			// overlap it with another job.
			ParallelSafe:      false,
			ExclusivityGroups: []string{"cluster"},

			// All virtbench scenarios share the ssh helper pod (created once).
			SetupGroups: []string{setupGroup},

			Evaluators: map[string]stages.Evaluator{
				sc.ProvidesTR: scenarioEvaluator{sc},
			},
		})
	}
}

// scenarioEvaluator adds the one piece of virtbench-specific scoring the generic
// grader can't know: a run passes only if every VM succeeded, so a native failure
// fails the TR on its own line and skips SLA grading (an average over an
// incomplete run is meaningless). Measurability (which SLAs this scenario can
// produce) is now data — the catalog's measured_by — so the generic grader
// (grader.GradeSLAs) handles skips and when-scoped bar selection (ADR-0013).
type scenarioEvaluator struct{ sc Scenario }

func (e scenarioEvaluator) Evaluate(_ context.Context, tr core.TestRequirement, res core.TestResult) ([]core.Verdict, error) {
	if res.Native == core.OutcomeFail {
		return []core.Verdict{{
			TR:      tr.ID,
			Item:    "native:all_vms_succeeded",
			Outcome: core.OutcomeFail,
			Reason:  fmt.Sprintf("run incomplete: %d of %d VMs failed", vmCount(res, "vms_failed"), vmCount(res, "vms_total")),
		}}, nil
	}
	var out []core.Verdict
	out = append(out, grader.GradeSLAs(tr, res)...)
	out = append(out, grader.GradeChecks(tr, res)...)
	return out, nil
}

// vmCount reads a run-level count the summary parser attaches (vms_failed /
// vms_total), returning 0 when absent.
func vmCount(res core.TestResult, name string) int {
	for _, m := range res.Metrics {
		if m.Name == name {
			return int(m.Value)
		}
	}
	return 0
}
