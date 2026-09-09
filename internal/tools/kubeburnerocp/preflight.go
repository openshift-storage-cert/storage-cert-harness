package kubeburnerocp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/clustercheck"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

type preflight struct{}

func (preflight) Check(ctx context.Context, rc *core.RunCtx, bag *core.Bag, trs []core.TestRequirement) ([]core.Finding, error) {
	p := resolveParams(trs)
	if err := p.validate(); err != nil {
		return []core.Finding{{Level: "error", Message: err.Error()}}, nil
	}
	bag.Set("params", p)

	var findings []core.Finding
	findings = append(findings, core.Finding{
		Level:   "info",
		Message: fmt.Sprintf("kube-burner-ocp workload=%s image=%s", p.Workload, DefaultImage),
	})

	if rc.Backend != nil && rc.Backend.StorageClass != "" {
		findings = append(findings, core.Finding{Level: "info", Message: "storage_class=" + rc.Backend.StorageClass})
	}

	for _, f := range findings {
		rc.Logger.Info(f.Message)
	}

	if usingHostBinary() {
		seen := map[string]bool{}
		for _, tr := range trs {
			for _, bin := range workloadSpecs[resolveParams([]core.TestRequirement{tr}).Workload].hostBins {
				if seen[bin] {
					continue
				}
				seen[bin] = true
				if _, err := exec.LookPath(bin); err != nil {
					findings = append(findings, core.Finding{Level: "error",
						Message: fmt.Sprintf("workload %s needs %q on PATH (KUBE_BURNER_OCP_USE_HOST=1); install it or run via the container image", p.Workload, bin)})
				} else {
					findings = append(findings, core.Finding{Level: "info", Message: "host binary found: " + bin})
				}
			}
		}
	}

	cli := clustercheck.KubeCLI()
	if cli == "" {
		findings = append(findings, core.Finding{
			Level:   "warn",
			Message: "neither kubectl nor oc found; skipping cluster prereq checks",
		})
		return findings, nil
	}

	// Run the union of every selected workload's cluster prerequisites, once each.
	seen := map[string]bool{}
	for _, tr := range trs {
		for _, c := range workloadSpecs[resolveParams([]core.TestRequirement{tr}).Workload].prereqs {
			if seen[c.Name] {
				continue
			}
			seen[c.Name] = true
			findings = append(findings, c.Check(ctx, cli))
		}
	}

	return findings, nil
}

type provisioner struct{}

func (provisioner) Provision(_ context.Context, rc *core.RunCtx, bag *core.Bag, _ []core.TestRequirement) error {
	dir := rc.WorkDir
	if dir == "" {
		dir = os.TempDir()
	}
	resultsDir := strings.TrimRight(dir, "/") + "/" + resultsSubdir
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		return fmt.Errorf("kube-burner-ocp: results dir: %w", err)
	}
	bag.Set("results_dir", resultsDir)
	rc.Logger.Info("kube-burner-ocp: provisioned results dir", "dir", resultsDir)
	return nil
}
