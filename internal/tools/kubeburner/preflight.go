package kubeburner

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/clustercheck"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

// snapshotCRD is the KubeVirt VirtualMachineSnapshot capability this tool needs.
var snapshotCRD = clustercheck.CRD("virtualmachinesnapshots.snapshot.kubevirt.io")

type preflight struct{}

func (preflight) Check(ctx context.Context, rc *core.RunCtx, bag *core.Bag, trs []core.TestRequirement) ([]core.Finding, error) {
	// Validate every selected TR's params, not just the first.
	for i := range trs {
		if err := resolveParams(trs[i : i+1]).validate(); err != nil {
			return []core.Finding{{Level: "error", Message: err.Error()}}, nil
		}
	}
	p := resolveParams(trs)
	bag.Set("params", p)

	sc := ""
	if rc.Backend != nil {
		sc = rc.Backend.StorageClass
	}
	if sc == "" {
		return []core.Finding{{
			Level:   "error",
			Message: "kube-burner: storage_class required (pass --backends/--backend)",
		}}, nil
	}

	var findings []core.Finding
	findings = append(findings, core.Finding{
		Level:   "info",
		Message: fmt.Sprintf("kube-burner vm-snapshot replicas=%d snapshot_count=%d image=%s vm_image=%s", p.Replicas, p.SnapshotCount, imageRef(), p.VMImage),
	})
	if sc != "" {
		findings = append(findings, core.Finding{Level: "info", Message: "storage_class=" + sc})
	}
	for _, f := range findings {
		rc.Logger.Info(f.Message)
	}

	caps := []clustercheck.Capability{clustercheck.KubeVirt, clustercheck.CDI, snapshotCRD}
	findings = append(findings, clustercheck.Preflight(ctx, caps, sc)...)
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
		return fmt.Errorf("kube-burner: results dir: %w", err)
	}
	bag.Set("results_dir", resultsDir)
	rc.Logger.Info("kube-burner: provisioned results dir", "dir", resultsDir)
	return nil
}
