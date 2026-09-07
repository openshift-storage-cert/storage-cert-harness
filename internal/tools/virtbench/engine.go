// Package virtbench integrates portworx/kubevirt-benchmark ("virtbench") as a
// tool. virtbench ships no upstream container image, so phase 1 runs the released
// CLI locally via os/exec; an in-cluster Job model is a documented follow-up
// (decisions/0007).
//
// virtbench exposes several scenarios (datasource-clone, its --boot-storm
// variant, disk-ops, …), each a distinct KB automation_tool and a distinct TR.
// They share ALL the plumbing here — preflight, provision, exec, collect,
// teardown, and replay — and differ only in a small Scenario config
// (scenario.go): the automation_tool string, the TR it provides, the result file
// name, how to build the CLI args, and how to parse the output. Adding a scenario
// should be a new table row, never an engine edit (decisions/0008).
//
// Bag contract (see decisions/0003):
//   - provisioner writes "tr_id" (string), "replay" (bool), "results_root" (string)
//   - runner      reads  "replay", "results_root"; execs virtbench unless replay
//   - collector   reads  "results_root"; finds + reads the scenario's result file
//   - parser      reads  "tr_id"
//
// Replay mode: a TR param "results_dir" points at a directory of already-collected
// virtbench results. Preflight skips binary/cluster checks, the runner skips exec,
// and the harness grades the pre-collected file — enabling offline e2e and
// re-grading without a cluster.
package virtbench

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/stages"
)

const (
	// binary is the virtbench CLI, expected on PATH (installed per
	// https://portworx.github.io/kubevirt-benchmark/install/).
	binary = "virtbench"

	// defaultVMName is the VM metadata.name in virtbench's bundled datasource-clone
	// template (examples/vm-templates/rhel9-vm-datasource.yaml). datasource-clone does
	// NOT substitute --vm-name into that template; it only polls the VM by that name.
	// So --vm-name MUST equal the template's name or the run hangs forever waiting for
	// a VM that never exists. Override vm_name only together with a matching custom
	// --vm-template. See decisions/0008.
	defaultVMName = "rhel-9-vm"
)

// --- Preflight ---------------------------------------------------------------

type preflight struct{ sc Scenario }

func (p preflight) Check(_ context.Context, rc *core.RunCtx, _ *core.Bag, trs []core.TestRequirement) ([]core.Finding, error) {
	// Replay grades pre-collected results — no binary or cluster needed.
	if dir := replayDir(trs); dir != "" {
		if _, err := os.Stat(dir); err != nil {
			return []core.Finding{{Level: "error", Message: fmt.Sprintf("virtbench: replay results_dir %q not accessible: %v", dir, err)}}, nil
		}
		return []core.Finding{{Level: "info", Message: "virtbench: replay mode (" + dir + "), skipping live-exec preflight"}}, nil
	}

	var findings []core.Finding
	if _, err := exec.LookPath(binary); err != nil {
		findings = append(findings, core.Finding{Level: "error", Message: "virtbench CLI not found on PATH (install per https://portworx.github.io/kubevirt-benchmark/install/, or use replay mode via the results_dir param)"})
	} else {
		findings = append(findings, core.Finding{Level: "info", Message: "virtbench CLI found on PATH"})
	}
	for _, tr := range trs {
		if storageClass(rc, tr) == "" {
			findings = append(findings, core.Finding{Level: "error", Message: fmt.Sprintf("virtbench: %s has no storage_class (select a --backend or set the TR 'storage_class' param)", tr.ID)})
		}
	}
	return findings, nil
}

// --- Provision ---------------------------------------------------------------

type provisioner struct{ sc Scenario }

func (p provisioner) Provision(_ context.Context, rc *core.RunCtx, bag *core.Bag, trs []core.TestRequirement) error {
	if len(trs) > 0 {
		bag.Set("tr_id", trs[0].ID)
		// Stash what Teardown's safety-net namespace sweep needs (Teardown gets no
		// TRs). namespace_prefix mirrors the default in the scenario's BuildArgs.
		bag.Set("ns_prefix", strParamOr(trs[0], "namespace_prefix", p.sc.NSPrefix))
		bag.Set("kubeconfig", strParamOr(trs[0], "kubeconfig", ""))
	}
	if dir := replayDir(trs); dir != "" {
		bag.Set("replay", true)
		bag.Set("results_root", dir)
		rc.Logger.Info("virtbench: replay mode", "results_dir", dir)
		return nil
	}
	root := rc.WorkDir
	if root == "" {
		root = "."
	}
	root = filepath.Join(root, "virtbench")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("virtbench: create results dir: %w", err)
	}
	bag.Set("replay", false)
	bag.Set("results_root", root)
	rc.Logger.Debug("virtbench: provisioned results dir", "results_root", root)
	return nil
}

// --- Run ---------------------------------------------------------------------

type runner struct{ sc Scenario }

func (r runner) Run(ctx context.Context, rc *core.RunCtx, bag *core.Bag, trs []core.TestRequirement) (stages.RunHandle, error) {
	replay, _ := core.GetAs[bool](bag, "replay")
	root, _ := core.GetAs[string](bag, "results_root")
	if replay {
		rc.Logger.Info("virtbench: replay — skipping exec", "results_root", root)
		return stages.RunHandle{ID: "virtbench-replay"}, nil
	}
	if len(trs) == 0 {
		return stages.RunHandle{}, fmt.Errorf("virtbench: no TRs to run")
	}
	args, err := r.sc.BuildArgs(rc, trs[0], root)
	if err != nil {
		return stages.RunHandle{}, err
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = root
	cmd.Stdout = logWriter{log: rc.Logger, stream: "stdout"}
	cmd.Stderr = logWriter{log: rc.Logger, stream: "stderr"}
	rc.Logger.Info("virtbench: exec", "cmd", binary, "args", args, "dir", root)
	if err := cmd.Run(); err != nil {
		return stages.RunHandle{}, fmt.Errorf("virtbench: run failed: %w", err)
	}
	return stages.RunHandle{ID: "virtbench-" + rc.RunID}, nil
}

// --- Collect -----------------------------------------------------------------

type collector struct{ sc Scenario }

func (c collector) Collect(_ context.Context, rc *core.RunCtx, bag *core.Bag, h stages.RunHandle) (core.LogBundle, error) {
	root, _ := core.GetAs[string](bag, "results_root")
	if root == "" {
		return core.LogBundle{}, fmt.Errorf("virtbench: no results_root in bag")
	}
	path, err := findResult(root, c.sc.ResultFile)
	if err != nil {
		return core.LogBundle{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return core.LogBundle{}, fmt.Errorf("virtbench: read results: %w", err)
	}
	rc.Logger.Info("virtbench: collected results", "path", path)
	return core.LogBundle{
		Refs: []string{path},
		Data: map[string][]byte{c.sc.ResultFile: data},
	}, nil
}

// findResult returns the path to the scenario's result file: root/<name> if
// present, else the first match found walking root (virtbench may nest results in
// a subdir).
func findResult(root, name string) (string, error) {
	direct := filepath.Join(root, name)
	if _, err := os.Stat(direct); err == nil {
		return direct, nil
	}
	var found string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("virtbench: search %s: %w", root, err)
	}
	if found == "" {
		return "", fmt.Errorf("virtbench: %s not found under %s", name, root)
	}
	return found, nil
}

// --- Parse -------------------------------------------------------------------

type parser struct{ sc Scenario }

func (p parser) Parse(_ context.Context, _ *core.RunCtx, bag *core.Bag, logs core.LogBundle) ([]core.TestResult, error) {
	trID, _ := core.GetAs[string](bag, "tr_id")
	if trID == "" {
		trID = p.sc.ProvidesTR
	}
	return p.sc.Parse(logs.Data[p.sc.ResultFile], trID)
}

// --- Teardown ----------------------------------------------------------------

type teardown struct{ sc Scenario }

// Teardown is a best-effort safety net. Normal runs pass --cleanup(-on-failure)
// so virtbench removes its own namespaces/VMs; this only catches the case where
// the virtbench process was killed mid-run and couldn't self-clean. It never
// fails the run.
func (t teardown) Teardown(ctx context.Context, rc *core.RunCtx, bag *core.Bag) error {
	if replay, _ := core.GetAs[bool](bag, "replay"); replay {
		return nil
	}
	prefix, _ := core.GetAs[string](bag, "ns_prefix")
	if prefix == "" {
		prefix = t.sc.NSPrefix
	}
	kubeconfig, _ := core.GetAs[string](bag, "kubeconfig")

	leftovers, err := listNamespacesByPrefix(ctx, kubeconfig, prefix+"-")
	if err != nil {
		rc.Logger.Warn("virtbench: teardown could not list namespaces; verify none leftover", "prefix", prefix+"-", "err", err)
		return nil
	}
	if len(leftovers) == 0 {
		return nil
	}
	rc.Logger.Warn("virtbench: cleaning up leftover namespaces (process likely killed mid-run)", "namespaces", leftovers)
	for _, ns := range leftovers {
		if err := deleteNamespace(ctx, kubeconfig, ns); err != nil {
			rc.Logger.Warn("virtbench: failed to delete leftover namespace", "namespace", ns, "err", err)
		}
	}
	return nil
}

// --- shared arg helpers ------------------------------------------------------

// datasourceCloneArgs builds the CLI for the datasource-clone family. The
// boot-storm scenario is the same subcommand plus the --boot-storm flag; it reads
// ONLY params/backend config — never a threshold/SLA value — so nothing sensitive
// can reach a command line. resultsRoot is where virtbench must write results so
// the collector can find them.
func datasourceCloneArgs(bootStorm bool, nsPrefix string) func(rc *core.RunCtx, tr core.TestRequirement, resultsRoot string) ([]string, error) {
	return func(rc *core.RunCtx, tr core.TestRequirement, resultsRoot string) ([]string, error) {
		sc := storageClass(rc, tr)
		if sc == "" {
			return nil, fmt.Errorf("virtbench: %s: storage_class required (backend or 'storage_class' param)", tr.ID)
		}
		start := intParam(tr, "start", 1)
		end := intParam(tr, "end", intParam(tr, "count", 100))

		args := clusterArgs(tr)
		args = append(args, "datasource-clone",
			"--start", strconv.Itoa(start),
			"--end", strconv.Itoa(end),
			"--vm-name", strParamOr(tr, "vm_name", defaultVMName),
			"--namespace-prefix", strParamOr(tr, "namespace_prefix", nsPrefix),
			"--storage-class", sc,
			"--save-results",
			"--results-folder", resultsRoot,
		)
		if tmpl := strParamOr(tr, "vm_template", ""); tmpl != "" {
			staged, err := stageTemplate(resultsRoot, tmpl)
			if err != nil {
				return nil, fmt.Errorf("virtbench: %s: %w", tr.ID, err)
			}
			args = append(args, "--vm-template", staged)
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
		if bootStorm {
			args = append(args, "--boot-storm")
		}
		// Let virtbench remove the namespaces/VMs it created, on success and on its
		// own failures; --yes skips the interactive confirmation. The adapter's
		// Teardown is only a safety net for a killed-mid-run process.
		args = append(args, "--cleanup", "--cleanup-on-failure", "--yes")
		return args, nil
	}
}

// diskOpsArgs builds the CLI for the disk-ops scenario (a different subcommand
// with its own flags — note --results-dir vs the clone family's --results-folder,
// and disk-ops templates its own VM name so no --vm-name fix is needed). Same
// secret-hygiene rule as datasourceCloneArgs.
func diskOpsArgs(nsPrefix string) func(rc *core.RunCtx, tr core.TestRequirement, resultsRoot string) ([]string, error) {
	return func(rc *core.RunCtx, tr core.TestRequirement, resultsRoot string) ([]string, error) {
		sc := storageClass(rc, tr)
		if sc == "" {
			return nil, fmt.Errorf("virtbench: %s: storage_class required (backend or 'storage_class' param)", tr.ID)
		}
		start := intParam(tr, "start", 1)
		end := intParam(tr, "end", intParam(tr, "count", 100))

		args := clusterArgs(tr)
		args = append(args, "disk-ops",
			"--start", strconv.Itoa(start),
			"--end", strconv.Itoa(end),
			"--namespace-prefix", strParamOr(tr, "namespace_prefix", nsPrefix),
			"--storage-class", sc,
			"--operation", strParamOr(tr, "operation", "hotplug"),
			"--save-results",
			"--results-dir", resultsRoot,
		)
		if tmpl := strParamOr(tr, "vm_template", ""); tmpl != "" {
			staged, err := stageTemplate(resultsRoot, tmpl)
			if err != nil {
				return nil, fmt.Errorf("virtbench: %s: %w", tr.ID, err)
			}
			args = append(args, "--vm-template", staged)
		}
		if disks := intParam(tr, "disks", 0); disks > 0 {
			args = append(args, "--disks", strconv.Itoa(disks))
		}
		// disk-ops needs --create-vms to provision the VMs it will hotplug into, and
		// --cleanup to remove them afterwards.
		args = append(args, "--create-vms", "--cleanup")
		return args, nil
	}
}

// stageTemplate copies the VM template into resultsRoot and returns the copy's
// absolute path. virtbench substitutes {{STORAGE_CLASS_NAME}} into the template
// file in place, so it must run against a throwaway copy in the workdir — never
// the repo template, which would get mutated (and its placeholder clobbered).
func stageTemplate(resultsRoot, tmpl string) (string, error) {
	data, err := os.ReadFile(tmpl)
	if err != nil {
		return "", fmt.Errorf("read vm_template %q: %w", tmpl, err)
	}
	dst := filepath.Join(resultsRoot, filepath.Base(tmpl))
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", fmt.Errorf("stage vm_template: %w", err)
	}
	return filepath.Abs(dst)
}

// clusterArgs emits the optional global connection flags shared by every
// scenario, before the subcommand.
func clusterArgs(tr core.TestRequirement) []string {
	var args []string
	if kc := strParamOr(tr, "kubeconfig", ""); kc != "" {
		args = append(args, "--kubeconfig", kc)
	}
	if uuid := strParamOr(tr, "uuid", ""); uuid != "" {
		args = append(args, "--uuid", uuid)
	}
	return args
}

// --- misc helpers ------------------------------------------------------------

// logWriter forwards a subprocess stream to the run logger, one write at a time.
type logWriter struct {
	log    *slog.Logger
	stream string
}

func (w logWriter) Write(p []byte) (int, error) {
	w.log.Info("virtbench", "stream", w.stream, "line", strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// storageClass resolves the storage class: the active backend wins, else the
// TR's 'storage_class' param.
func storageClass(rc *core.RunCtx, tr core.TestRequirement) string {
	if rc != nil && rc.Backend != nil && rc.Backend.StorageClass != "" {
		return rc.Backend.StorageClass
	}
	return strParamOr(tr, "storage_class", "")
}

// replayDir returns the first non-empty results_dir param among trs, if any.
func replayDir(trs []core.TestRequirement) string {
	for _, tr := range trs {
		if d := strParamOr(tr, "results_dir", ""); d != "" {
			return d
		}
	}
	return ""
}

func strParamOr(tr core.TestRequirement, key, def string) string {
	if tr.Params == nil {
		return def
	}
	if v, ok := tr.Params[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return def
}

func intParam(tr core.TestRequirement, key string, def int) int {
	if tr.Params == nil {
		return def
	}
	v, ok := tr.Params[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
	}
	return def
}
