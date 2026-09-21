// Package virtbench
// TR-VIRT-018 ("VM Cloning at Scale", 2 disks per VM) rides the generic
// engine like every other datasource-clone scenario, but needs its own
// BuildArgs: virtbench's --num-disks flag only labels output, it never adds
// a disk to the VM (upstream measure-vm-creation-time.py uses it purely for
// the results-directory name and reporting) — the real disk count comes
// entirely from the vm_template. So "2 disks each" requires an actual
// 2-disk template, not a plan param; this scenario embeds one and ignores
// any num_disks/vm_template a plan might set, since there's nothing left for
// a plan to get wrong.
package virtbench

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strconv"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/safefs"
)

const twoDiskVMTemplateFile = "rhel9-vm-2disk.yaml"

//go:embed templates/rhel9-vm-2disk.yaml
var twoDiskVMTemplate string

// cloneAtScaleArgs is TR-VIRT-018's BuildArgs: the same shape as
// datasourceCloneArgs (start/end, storage class, namespace-prefix,
// save-results, cleanup), but the 2-disk template and --num-disks 2 are
// fixed, not plan-configurable.
func cloneAtScaleArgs(nsPrefix string) func(rc *core.RunCtx, tr core.TestRequirement, resultsRoot string) ([]string, error) {
	return func(rc *core.RunCtx, tr core.TestRequirement, resultsRoot string) ([]string, error) {
		sc := storageClass(rc, tr)
		if sc == "" {
			return nil, fmt.Errorf("virtbench: %s: storage_class required (backend or 'storage_class' param)", tr.ID)
		}
		staged, err := stageTwoDiskTemplate(resultsRoot)
		if err != nil {
			return nil, fmt.Errorf("virtbench: %s: %w", tr.ID, err)
		}
		start := intParam(tr, "start", 1)
		end := intParam(tr, "iteration_clones", 100)

		args := clusterArgs(tr)
		args = append(args, "datasource-clone",
			"--start", strconv.Itoa(start),
			"--end", strconv.Itoa(end),
			"--vm-name", defaultVMName,
			"--namespace-prefix", strParamOr(tr, "namespace_prefix", nsPrefix),
			"--storage-class", sc,
			"--vm-template", staged,
			"--num-disks", "2",
			"--save-results",
			"--results-folder", resultsRoot,
			"--cleanup", "--cleanup-on-failure", "--yes",
		)
		return args, nil
	}
}

// stageTwoDiskTemplate writes the embedded 2-disk template into resultsRoot
// (virtbench substitutes {{STORAGE_CLASS_NAME}} into the file in place, so it
// must run against a throwaway copy, never a shared source — same reason
// stageTemplate/stageFIOTemplate in engine.go copy rather than pass through).
func stageTwoDiskTemplate(resultsRoot string) (string, error) {
	dst := filepath.Join(resultsRoot, twoDiskVMTemplateFile)
	if err := safefs.WriteFile(dst, []byte(twoDiskVMTemplate), 0o600); err != nil {
		return "", fmt.Errorf("stage embedded 2-disk vm template: %w", err)
	}
	return filepath.Abs(dst)
}
