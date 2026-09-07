package kubeburner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/clustercheck"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/stages"
)

// imageRef is the kube-burner runner image, overridable via KUBE_BURNER_IMAGE.
func imageRef() string {
	if v := os.Getenv("KUBE_BURNER_IMAGE"); v != "" {
		return v
	}
	return DefaultImage
}

type runner struct{}

const cleanupTimeout = 30 * time.Second

func (runner) Run(ctx context.Context, rc *core.RunCtx, bag *core.Bag, trs []core.TestRequirement) (stages.RunHandle, error) {
	if len(trs) != 1 {
		return stages.RunHandle{}, fmt.Errorf("kube-burner: expected exactly 1 TR, got %d", len(trs))
	}
	tr := trs[0]
	if tr.ID == "" {
		return stages.RunHandle{}, fmt.Errorf("kube-burner: TR has empty id")
	}
	resultsDir, _ := core.GetAs[string](bag, "results_dir")
	if resultsDir == "" {
		return stages.RunHandle{}, fmt.Errorf("kube-burner: results_dir missing (provision first)")
	}
	p := resolveParams(trs)
	if err := p.validate(); err != nil {
		return stages.RunHandle{}, err
	}
	sc := ""
	if rc.Backend != nil {
		sc = rc.Backend.StorageClass
	}
	if sc == "" {
		return stages.RunHandle{}, fmt.Errorf("kube-burner: storage_class required (pass --backends/--backend)")
	}

	subdir := filepath.Join(resultsDir, tr.ID)
	if err := os.RemoveAll(subdir); err != nil {
		return stages.RunHandle{}, err
	}
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		return stages.RunHandle{}, err
	}
	if _, err := writeConfig(subdir, p); err != nil {
		return stages.RunHandle{}, err
	}
	containerIDFile := filepath.Join(subdir, "kube-burner.cid")
	cmd, err := buildRunCmd(ctx, rc, buildEnv(p, sc), resultsDir, tr.ID, containerIDFile)
	if err != nil {
		return stages.RunHandle{}, err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	rc.Logger.Info("kube-burner: running", "tr", tr.ID, "replicas", p.Replicas, "snapshot_count", p.SnapshotCount)
	runErr := ""
	if err := cmd.Run(); err != nil {
		runErr = err.Error()
		rc.Logger.Warn("kube-burner: run failed", "tr", tr.ID, "err", err)
	}
	if ctx.Err() != nil {
		stopContainer(containerIDFile, rc.Logger)
	}
	bag.Set("kb_subdir", subdir)
	bag.Set("kb_trid", tr.ID)
	bag.Set("kb_runerr", runErr)
	return stages.RunHandle{ID: "kube-burner"}, nil
}

// buildRunCmd runs kube-burner init -c via the released image (podman, ADR-0003), or the host binary if KUBE_BURNER_USE_HOST=1.
func buildRunCmd(ctx context.Context, rc *core.RunCtx, env []string, resultsDir, subdir, containerIDFile string) (*exec.Cmd, error) {
	absResults, err := filepath.Abs(resultsDir)
	if err != nil {
		return nil, fmt.Errorf("kube-burner: results dir: %w", err)
	}

	if os.Getenv("KUBE_BURNER_USE_HOST") == "1" {
		path, err := exec.LookPath("kube-burner")
		if err != nil {
			return nil, fmt.Errorf("kube-burner: KUBE_BURNER_USE_HOST=1 but kube-burner not on PATH: %w", err)
		}
		cfg := filepath.Join(absResults, subdir, "config.yaml")
		cmd := exec.CommandContext(ctx, path, "init", "-c", cfg)
		cmd.Dir = filepath.Join(absResults, subdir)
		cmd.Env = append(os.Environ(), env...)
		rc.Logger.Info("kube-burner: running host binary", "path", path)
		return cmd, nil
	}

	// Generic kube-burner uses client-go + KUBECONFIG (no kubectl binary), so we mount only results + kubeconfig.
	podmanArgs := []string{
		"run", "--rm", "--network", "host",
		"--cidfile", containerIDFile,
		"-v", absResults + ":/work/results:z",
		"-e", "KUBECONFIG=/kube/config",
		"-w", "/work/results/" + subdir,
	}
	for _, e := range env {
		podmanArgs = append(podmanArgs, "-e", e)
	}
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		absKC, _ := filepath.Abs(kc)
		podmanArgs = append(podmanArgs, "-v", absKC+":/kube/config:ro,z")
	} else if home, err := os.UserHomeDir(); err == nil {
		podmanArgs = append(podmanArgs, "-v", home+"/.kube/config:/kube/config:ro,z")
	}
	podmanArgs = append(podmanArgs, imageRef(),
		"init", "-c", "/work/results/"+subdir+"/config.yaml")
	rc.Logger.Info("kube-burner: running via podman", "image", imageRef())
	return exec.CommandContext(ctx, "podman", podmanArgs...), nil
}

func stopContainer(containerIDFile string, logger *slog.Logger) {
	b, err := os.ReadFile(containerIDFile)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("kube-burner: read container ID", "err", err)
		}
		return
	}
	containerID := strings.TrimSpace(string(b))
	if containerID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "podman", "stop", "--time", "10", containerID).CombinedOutput(); err != nil {
		logger.Warn("kube-burner: stop container", "id", containerID, "err", err, "output", strings.TrimSpace(string(output)))
	}
}

type collector struct{}

func (collector) Collect(_ context.Context, rc *core.RunCtx, bag *core.Bag, _ stages.RunHandle) (core.LogBundle, error) {
	subdir, _ := core.GetAs[string](bag, "kb_subdir")
	trID, _ := core.GetAs[string](bag, "kb_trid")
	m, err := findCollectedMetrics(subdir)
	if err != nil {
		rc.Logger.Warn("kube-burner: no results collected", "tr", trID, "err", err)
		return core.LogBundle{}, nil // Parse emits a fail verdict
	}
	rc.Logger.Info("kube-burner: collected results", "tr", trID, "files", len(m))
	return core.LogBundle{Refs: []string{subdir}, Data: m}, nil
}

func findCollectedMetrics(root string) (map[string][]byte, error) {
	data := map[string][]byte{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return walkErr
		}
		name := info.Name()
		if name == "jobSummary.json" || strings.Contains(name, "volumeSnapshotLatency") {
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
		return nil, fmt.Errorf("kube-burner: no collected-metrics under %s", root)
	}
	return data, nil
}

type parser struct{}

func (parser) Parse(_ context.Context, _ *core.RunCtx, bag *core.Bag, logs core.LogBundle) ([]core.TestResult, error) {
	trID, _ := core.GetAs[string](bag, "kb_trid")
	if _, ok := logs.Data["jobSummary.json"]; !ok {
		reason := "kube-burner produced no jobSummary"
		if e, _ := core.GetAs[string](bag, "kb_runerr"); e != "" {
			reason = e
		}
		raw, _ := json.Marshal(map[string]string{"error": reason})
		return []core.TestResult{{TRID: trID, Native: core.OutcomeFail, Raw: raw}}, nil
	}
	p, ok := core.GetAs[Params](bag, "params")
	if !ok {
		return nil, fmt.Errorf("kube-burner: params missing (preflight first)")
	}
	res, err := parseResults(trID, logs.Data, p.SnapshotCount)
	if err != nil {
		return nil, err
	}
	return []core.TestResult{res}, nil
}

type teardown struct{}

// Teardown best-effort deletes the run namespace (VMs, DataVolumes, snapshots).
func (teardown) Teardown(ctx context.Context, rc *core.RunCtx, bag *core.Bag) error {
	p, ok := core.GetAs[Params](bag, "params")
	if !ok || p.NS == "" {
		return nil
	}
	cli := clustercheck.KubeCLI()
	if cli == "" {
		return nil
	}
	rc.Logger.Info("kube-burner: removing run namespace", "ns", p.NS)
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	if output, err := exec.CommandContext(cleanupCtx, cli, "delete", "ns", p.NS, "--wait=false", "--ignore-not-found").CombinedOutput(); err != nil {
		rc.Logger.Warn("kube-burner: remove run namespace", "ns", p.NS, "err", err, "output", strings.TrimSpace(string(output)))
	}
	return nil
}
