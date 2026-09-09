package virtbench

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/stages"
)

// The virtbench scenarios all need one shared ssh helper pod: datasource-clone /
// boot-storm ping the VMs through it (they only VALIDATE it exists and fail
// otherwise), and disk-ops runs in-VM checks through it via sshpass. It is a
// single cluster-wide resource, so the harness creates it ONCE for a whole batch
// of virtbench tests (a group-scoped SetupProvider) rather than once per scenario
// — the customer never creates it by hand. Mirrors virtbench's own ensure_ssh_pod
// (disk-ops-benchmark/measure-disk-ops.py). See decisions/0008.
const (
	sshPodName = "ssh-test-pod"
	sshPodNS   = "default"
)

// sshPodManifest is the helper pod virtbench uses for ping and in-VM SSH checks.
// Uses alpine + apk (same as virtbench's own ensure_ssh_pod) so both ping and
// sshpass are available. The pod is left running (persistent) so subsequent runs
// reuse it instantly — our sshpassReady check confirms sshpass is installed before
// declaring it ready. Remove manually when decommissioning the cluster:
//
//	kubectl delete pod ssh-test-pod -n default
const sshPodManifest = `apiVersion: v1
kind: Pod
metadata:
  name: ` + sshPodName + `
  namespace: ` + sshPodNS + `
  labels:
    app: kubevirt-perf-test
    managed-by: storage-cert-harness
spec:
  containers:
  - name: ssh-client
    image: alpine:latest
    command: ["/bin/sh", "-c", "apk add --no-cache bash openssh-client sshpass iputils && tail -f /dev/null"]
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
      limits:
        memory: "256Mi"
        cpu: "200m"
  restartPolicy: Always
`

// sshPodSetup is the group-scoped provider that ensures the shared ssh helper pod.
type sshPodSetup struct {
	createdByUs bool
}

func (*sshPodSetup) SetupInfo() stages.SetupInfo {
	return stages.SetupInfo{Scope: stages.ScopeGroup, Key: setupGroup}
}

// Setup ensures the ssh helper pod exists and has sshpass, creating it if absent.
// Idempotent: reuses a pod that is already usable (so re-runs and a pre-existing
// pod are fine). Called once per run for the whole virtbench group.
func (s *sshPodSetup) Setup(ctx context.Context, rc *core.RunCtx, trs []core.TestRequirement) error {
	// Replay grades pre-collected results with no cluster — no helper pod needed.
	if replayDir(trs) != "" {
		rc.Logger.Info("virtbench: replay mode, skipping ssh helper pod setup")
		return nil
	}
	kubeconfig := kubeconfigOf(trs)

	if sshpassReady(ctx, kubeconfig) {
		rc.Logger.Info("virtbench: reusing existing ssh helper pod", "pod", sshPodNS+"/"+sshPodName)
		return nil
	}
	if !podExists(ctx, kubeconfig) {
		rc.Logger.Info("virtbench: creating shared ssh helper pod", "pod", sshPodNS+"/"+sshPodName)
		if err := kubectlApply(ctx, kubeconfig, sshPodManifest); err != nil {
			return fmt.Errorf("create ssh helper pod: %w", err)
		}
		s.createdByUs = true // tracked for the log message in Teardown
	}

	// Wait for apk install to complete (image pull + apk add openssh sshpass iputils).
	// 300s matches virtbench's own ping timeout and handles slow registries.
	deadline := time.Now().Add(300 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if sshpassReady(ctx, kubeconfig) {
			rc.Logger.Info("virtbench: ssh helper pod ready", "pod", sshPodNS+"/"+sshPodName)
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("ssh helper pod %s/%s not ready after 300s (apk install may have timed out)", sshPodNS, sshPodName)
}

// Teardown removes the ssh helper pod if this run created it. This is the group
// teardown — called once after ALL virtbench jobs in the run complete, so the pod
// is shared across every scenario in a single harness invocation and removed once
// at the end. A pre-existing pod (created by a previous run or by the customer) is
// left alone. Best-effort; never fails the run.
func (s *sshPodSetup) Teardown(ctx context.Context, rc *core.RunCtx, trs []core.TestRequirement) error {
	if !s.createdByUs {
		return nil
	}
	kubeconfig := kubeconfigOf(trs)
	rc.Logger.Info("virtbench: removing shared ssh helper pod", "pod", sshPodNS+"/"+sshPodName)
	_, _, err := runKubectl(ctx, kubeconfig, "delete", "pod", sshPodName, "-n", sshPodNS, "--wait=false", "--ignore-not-found")
	return err
}

// --- kubectl helpers ---------------------------------------------------------

// kubeconfigOf returns the first non-empty "kubeconfig" param among trs. Empty
// means "use the ambient KUBECONFIG / default", so no --kubeconfig flag is added.
func kubeconfigOf(trs []core.TestRequirement) string {
	for _, tr := range trs {
		if kc := strParamOr(tr, "kubeconfig", ""); kc != "" {
			return kc
		}
	}
	return ""
}

// runKubectl runs a kubectl command, returning combined stdout, exit-ok, and any
// exec error. A non-zero exit is reported via ok=false, not err (err is reserved
// for failing to launch kubectl at all).
func runKubectl(ctx context.Context, kubeconfig string, args ...string) (out string, ok bool, err error) {
	full := args
	if kubeconfig != "" {
		full = append([]string{"--kubeconfig", kubeconfig}, args...)
	}
	cmd := exec.CommandContext(ctx, "kubectl", full...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	runErr := cmd.Run()
	if runErr == nil {
		return buf.String(), true, nil
	}
	if _, isExit := runErr.(*exec.ExitError); isExit {
		return buf.String(), false, nil
	}
	return buf.String(), false, runErr
}

// kubectlApply pipes a manifest to `kubectl apply -f -`.
func kubectlApply(ctx context.Context, kubeconfig, manifest string) error {
	args := []string{"apply", "-f", "-"}
	if kubeconfig != "" {
		args = append([]string{"--kubeconfig", kubeconfig}, args...)
	}
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdin = strings.NewReader(manifest)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply: %v: %s", err, strings.TrimSpace(buf.String()))
	}
	return nil
}

// podExists returns true only if the pod exists AND is not Terminating — a
// Terminating pod can't be exec'd into and will be replaced by a fresh one.
func podExists(ctx context.Context, kubeconfig string) bool {
	out, ok, _ := runKubectl(ctx, kubeconfig, "get", "pod", sshPodName, "-n", sshPodNS,
		"-o", "jsonpath={.metadata.deletionTimestamp}")
	if !ok {
		return false // doesn't exist
	}
	return strings.TrimSpace(out) == "" // exists and NOT terminating
}

// sshpassReady checks that the pod has sshpass installed and ready. The alpine
// pod installs it via apk on startup; this exec confirms the install completed.
func sshpassReady(ctx context.Context, kubeconfig string) bool {
	_, ok, _ := runKubectl(ctx, kubeconfig, "exec", "-n", sshPodNS, sshPodName, "--", "which", "sshpass")
	return ok
}

// listNamespacesByPrefix returns namespaces whose name starts with prefix.
func listNamespacesByPrefix(ctx context.Context, kubeconfig, prefix string) ([]string, error) {
	out, ok, err := runKubectl(ctx, kubeconfig, "get", "ns", "-o", "name")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("kubectl get ns failed: %s", strings.TrimSpace(out))
	}
	var names []string
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		name := strings.TrimPrefix(strings.TrimSpace(line), "namespace/")
		if name != "" && strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	return names, nil
}

func deleteNamespace(ctx context.Context, kubeconfig, ns string) error {
	out, ok, err := runKubectl(ctx, kubeconfig, "delete", "ns", ns, "--wait=false", "--ignore-not-found")
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("kubectl delete ns %s: %s", ns, strings.TrimSpace(out))
	}
	return nil
}
