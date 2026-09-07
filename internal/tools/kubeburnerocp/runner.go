package kubeburnerocp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/clustercheck"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/stages"
)

// kboRun records one workload execution (one TR) so Collect/Parse can handle a
// job that bundles several kube-burner TRs.
type kboRun struct {
	TRID     string
	Workload string
	Subdir   string
	RunErr   string
	Skipped  string
}

type runner struct{}

func (runner) Run(ctx context.Context, rc *core.RunCtx, bag *core.Bag, trs []core.TestRequirement) (stages.RunHandle, error) {
	if len(trs) == 0 {
		return stages.RunHandle{}, fmt.Errorf("kube-burner-ocp: no TRs to run")
	}

	resultsDir, _ := core.GetAs[string](bag, "results_dir")
	if resultsDir == "" {
		return stages.RunHandle{}, fmt.Errorf("kube-burner-ocp: results_dir missing (provision first)")
	}
	sc := ""
	if rc.Backend != nil {
		sc = rc.Backend.StorageClass
	}

	// Per-TR errors are isolated (recorded, not fatal); a repeated workload is skipped (shared namespace).
	runs := make([]kboRun, len(trs))
	cmds := make([]*exec.Cmd, len(trs))
	seen := map[string]bool{}
	for i, tr := range trs {
		p := resolveParams([]core.TestRequirement{tr})
		if err := p.validate(); err != nil {
			runs[i] = kboRun{TRID: tr.ID, RunErr: err.Error()}
			continue
		}
		runs[i] = kboRun{TRID: tr.ID, Workload: p.Workload}
		if seen[p.Workload] {
			runs[i].Skipped = "duplicate workload in job; only the first runs (shared namespace)"
			continue
		}
		seen[p.Workload] = true

		subdir := filepath.Join(resultsDir, tr.ID)
		if err := os.MkdirAll(subdir, 0o755); err != nil {
			runs[i].RunErr = err.Error()
			continue
		}
		args, err := buildCLIArgs(p, sc)
		if err != nil {
			runs[i].RunErr = err.Error()
			continue
		}
		cmd, err := buildRunCmd(ctx, rc, args, resultsDir, tr.ID)
		if err != nil {
			runs[i].RunErr = err.Error()
			continue
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmds[i] = cmd
		runs[i].Subdir = subdir
	}

	var wg sync.WaitGroup
	for i := range runs {
		if cmds[i] == nil {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rc.Logger.Info("kube-burner-ocp: running", "tr", runs[i].TRID, "workload", runs[i].Workload)
			if err := cmds[i].Run(); err != nil {
				runs[i].RunErr = err.Error()
				rc.Logger.Warn("kube-burner-ocp: run failed", "tr", runs[i].TRID, "err", err)
			}
		}(i)
	}
	wg.Wait()

	bag.Set("kbo_runs", runs)
	return stages.RunHandle{ID: "kube-burner-ocp"}, nil
}

// buildRunCmd runs the wrapper image via podman (ADR-0003), or the host binary if KUBE_BURNER_OCP_USE_HOST=1, writing into resultsDir/<subdir>.
// usingHostBinary reports whether the adapter runs kube-burner-ocp from the host
// PATH instead of the container image (KUBE_BURNER_OCP_USE_HOST=1). In host mode
// the workload's host prerequisites (e.g. virtctl) must be on PATH too.
func usingHostBinary() bool { return os.Getenv("KUBE_BURNER_OCP_USE_HOST") == "1" }

func buildRunCmd(ctx context.Context, rc *core.RunCtx, args []string, resultsDir, subdir string) (*exec.Cmd, error) {
	if usingHostBinary() {
		path, err := exec.LookPath("kube-burner-ocp")
		if err != nil {
			return nil, fmt.Errorf("kube-burner-ocp: KUBE_BURNER_OCP_USE_HOST=1 but kube-burner-ocp not on PATH: %w", err)
		}
		cmd := exec.CommandContext(ctx, path, args...)
		cmd.Dir = filepath.Join(resultsDir, subdir)
		rc.Logger.Info("kube-burner-ocp: running host binary", "path", path)
		return cmd, nil
	}
	return buildPodmanCmd(ctx, rc, args, resultsDir, subdir)
}

func buildPodmanCmd(ctx context.Context, rc *core.RunCtx, args []string, resultsDir, subdir string) (*exec.Cmd, error) {
	absResults, err := filepath.Abs(resultsDir)
	if err != nil {
		return nil, fmt.Errorf("kube-burner-ocp: results dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(absResults, subdir), 0o755); err != nil {
		return nil, err
	}

	kubectlPath, err := clustercheck.ResolveKubeCLIPath()
	if err != nil {
		return nil, err
	}

	podmanArgs := []string{
		"run", "--rm", "--network", "host",
		"-v", absResults + ":/work/results:z",
		"-v", kubectlPath + ":/usr/local/bin/kubectl:ro,z",
		"-e", "KUBECONFIG=/kube/config",
		"-w", "/work/results/" + subdir,
	}
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		absKC, _ := filepath.Abs(kc)
		podmanArgs = append(podmanArgs, "-v", absKC+":/kube/config:ro,z")
	} else if home, err := os.UserHomeDir(); err == nil {
		podmanArgs = append(podmanArgs, "-v", home+"/.kube/config:/kube/config:ro,z")
	}

	podmanArgs = append(podmanArgs, DefaultImage)
	podmanArgs = append(podmanArgs, args...)
	rc.Logger.Info("kube-burner-ocp: running via podman", "image", DefaultImage, "kubectl", kubectlPath)
	return exec.CommandContext(ctx, "podman", podmanArgs...), nil
}

type collector struct{}

func (collector) Collect(_ context.Context, rc *core.RunCtx, bag *core.Bag, _ stages.RunHandle) (core.LogBundle, error) {
	runs, _ := core.GetAs[[]kboRun](bag, "kbo_runs")

	data := map[string][]byte{}
	var refs []string
	for _, r := range runs {
		if r.Skipped != "" {
			continue
		}
		m, err := findCollectedMetrics(r.Subdir)
		if err != nil {
			rc.Logger.Warn("kube-burner-ocp: no results collected", "tr", r.TRID, "err", err)
			continue // Parse emits a fail verdict for this TR
		}
		refs = append(refs, r.Subdir)
		for name, content := range m {
			data[r.TRID+"/"+name] = content
		}
	}
	rc.Logger.Info("kube-burner-ocp: collected results", "trs", len(runs))
	return core.LogBundle{Refs: refs, Data: data}, nil
}

func findCollectedMetrics(root string) (map[string][]byte, error) {
	data := make(map[string][]byte)
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return walkErr
		}
		name := info.Name()
		if name == "jobSummary.json" || strings.Contains(name, "Quantiles") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			data[name] = content
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("kube-burner-ocp: no collected-metrics under %s", root)
	}
	return data, nil
}

type parser struct{}

func (parser) Parse(_ context.Context, _ *core.RunCtx, bag *core.Bag, logs core.LogBundle) ([]core.TestResult, error) {
	runs, _ := core.GetAs[[]kboRun](bag, "kbo_runs")
	var out []core.TestResult
	for _, r := range runs {
		if r.Skipped != "" {
			raw, _ := json.Marshal(map[string]string{"skipped": r.Skipped})
			out = append(out, core.TestResult{TRID: r.TRID, Native: core.OutcomeSkip, Raw: raw})
			continue
		}
		sub := map[string][]byte{}
		prefix := r.TRID + "/"
		for k, v := range logs.Data {
			if strings.HasPrefix(k, prefix) {
				sub[strings.TrimPrefix(k, prefix)] = v
			}
		}

		if _, ok := sub["jobSummary.json"]; !ok {
			reason := "kube-burner produced no jobSummary"
			if r.RunErr != "" {
				reason = r.RunErr
			}
			raw, _ := json.Marshal(map[string]string{"error": reason})
			out = append(out, core.TestResult{TRID: r.TRID, Native: core.OutcomeFail, Raw: raw})
			continue
		}

		res, err := ParseResults(r.TRID, sub)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, nil
}

type teardown struct{}

// Teardown relies on each workload's native gc/cleanup.
func (teardown) Teardown(_ context.Context, _ *core.RunCtx, _ *core.Bag) error {
	return nil
}
