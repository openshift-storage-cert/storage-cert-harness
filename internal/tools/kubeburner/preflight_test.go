package kubeburner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

func TestPreflightContinuesAfterInvalidParameters(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got, err := (preflight{}).Check(context.Background(), &core.RunCtx{}, core.NewBag(), []core.TestRequirement{
		{ID: "TR-VIRT-010"},
		{ID: "TR-VIRT-027", Params: map[string]any{"replicas": 1, "snapshot_count": 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 9 {
		t.Fatalf("findings = %+v, want all 9 checks", got)
	}
	for i, level := range []string{"error", "info", "error", "error", "skip", "skip", "skip", "skip", "skip"} {
		if got[i].Level != level {
			t.Errorf("check %d = %+v, want %s", i, got[i], level)
		}
	}
	if !strings.Contains(got[1].Message, "TR-VIRT-027") {
		t.Fatal("missing later TR's parameter check")
	}
}

func TestPreflightSuccessfulSnapshot(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$2" in
  volumepopulator) echo '{"items":[]}' ;;
  storageclass) echo '{"provisioner":"test-driver"}' ;;
  storageprofile) echo '{"status":{"snapshotClass":"test-snapshot"}}' ;;
  volumesnapshotclass) echo '{"driver":"test-driver"}' ;;
esac
exit 0
`
	for _, binary := range []string{"kube-burner", "kubectl"} {
		if err := os.WriteFile(filepath.Join(dir, binary), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	bag := core.NewBag()
	got, err := (preflight{}).Check(context.Background(), &core.RunCtx{Backend: &core.ResolvedBackend{StorageClass: "test-sc"}}, bag,
		[]core.TestRequirement{{ID: "TR-VIRT-010", Params: map[string]any{"replicas": 1, "snapshot_count": 1}}})
	if err != nil {
		t.Fatal(err)
	}
	// No snapshot class on the backend, so the class is auto-resolved and
	// Provision will not prepare storage: no "wait --for=create" check here.
	if len(got) != 9 {
		t.Fatalf("findings = %+v, want parameters, storage config, binary, 3 CRDs, storage existence, snapshot class, populator", got)
	}
	for _, f := range got {
		if f.Level != "info" {
			t.Errorf("finding = %+v, want pass", f)
		}
	}
	p, ok := core.GetAs[Params](bag, "params")
	if !ok || p.UsePopulator {
		t.Fatalf("params = %+v, want legacy CDI import fallback", p)
	}
}

// The private StorageProfile that needs "wait --for=create" is only built when
// the backend names a snapshot class; a class resolved from the cluster default
// or the storage profile skips that path entirely. Demanding the newer client
// there would block a run that never calls it.
func TestPreflightSkipsWaitProbeWhenSnapshotClassIsAutoResolved(t *testing.T) {
	dir := t.TempDir()
	// A client that rejects the condition, as pre-1.31 kubectl does.
	script := `#!/bin/sh
case "$2" in
  volumepopulator) echo '{"items":[]}' ;;
  storageclass) echo '{"provisioner":"test-driver"}' ;;
  storageprofile) echo '{"status":{"snapshotClass":"test-snapshot"}}' ;;
  volumesnapshotclass) echo '{"driver":"test-driver"}' ;;
esac
if [ "$1" = "wait" ]; then
  echo 'error: unrecognized condition: "create"' >&2
  exit 1
fi
exit 0
`
	for _, binary := range []string{"kube-burner", "kubectl"} {
		if err := os.WriteFile(filepath.Join(dir, binary), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	trs := []core.TestRequirement{{ID: "TR-VIRT-010", Params: map[string]any{"replicas": 1, "snapshot_count": 1}}}

	// No SnapshotClass on the backend: the class is resolved for reporting, but
	// Provision will not prepare storage, so the old client is irrelevant.
	got, err := (preflight{}).Check(context.Background(),
		&core.RunCtx{Backend: &core.ResolvedBackend{StorageClass: "test-sc"}}, core.NewBag(), trs)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		if f.Level == "error" {
			t.Errorf("auto-resolved snapshot class blocked by an unrelated client check: %+v", f)
		}
		if strings.Contains(f.Message, "wait --for=create") {
			t.Errorf("probed a condition this run never uses: %+v", f)
		}
	}

	// With the class pinned, Provision does prepare storage, so the old client
	// must fail preflight rather than part-way through the run.
	got, err = (preflight{}).Check(context.Background(),
		&core.RunCtx{Backend: &core.ResolvedBackend{StorageClass: "test-sc", SnapshotClass: "test-snapshot"}}, core.NewBag(), trs)
	if err != nil {
		t.Fatal(err)
	}
	var blocked bool
	for _, f := range got {
		if f.Level == "error" && strings.Contains(f.Message, "too old") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("a pinned snapshot class with an old client must fail preflight: %+v", got)
	}
}
