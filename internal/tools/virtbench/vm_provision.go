package virtbench

import (
	"context"
	_ "embed"
	"fmt"
	"math/rand"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

//go:embed templates/rhel9-vm-failure-recovery.yaml
var failureRecoveryVMTemplate string

// provisionFailureRecoveryVMs creates vm_count VMs (one per namespace) on the
// target node using requiredDuringSchedulingIgnoredDuringExecution affinity —
// they MUST land on the node at creation but CAN be live-migrated away on failure.
// Namespaces follow the pattern {nsPrefix}-{i}, which virtbench picks up via
// --namespace-prefix and Teardown sweeps via listNamespacesByPrefix.
// If kill_node: true, kills the target node after all VMs are Running so virtbench
// starts in mode: monitor against an already-NotReady node.
func provisionFailureRecoveryVMs(ctx context.Context, rc *core.RunCtx, bag *core.Bag, trs []core.TestRequirement) error {
	if len(trs) == 0 {
		return nil
	}
	tr := trs[0]
	kubeconfig, _ := core.GetAs[string](bag, "kubeconfig")

	node := strParamOr(tr, "node", "")
	hostnameLabel := strParamOr(tr, "node_hostname_label", "")
	if node == "" {
		picked, label, err := pickWorkerNode(ctx, kubeconfig)
		if err != nil {
			return err
		}
		node, hostnameLabel = picked, label
		rc.Logger.Info("virtbench: auto-selected worker node", "node", node, "hostname_label", hostnameLabel)
	} else if hostnameLabel == "" {
		if label, ok, _ := runKubectl(ctx, kubeconfig, "get", "node", node,
			`-o`, `jsonpath={.metadata.labels.kubernetes\.io/hostname}`); ok && strings.TrimSpace(label) != "" {
			hostnameLabel = strings.TrimSpace(label)
		} else {
			hostnameLabel = node
		}
	}
	bag.Set("picked_node", node)
	sc := storageClass(rc, tr)
	if sc == "" {
		return fmt.Errorf("virtbench: TR-VIRT-007: storage_class required (select a --backend or set the 'storage_class' param)")
	}

	nsPrefix := strParamOr(tr, "namespace_prefix", "failure-recovery")
	vmName := strParamOr(tr, "vm_name", defaultVMName)
	count := intParam(tr, "vm_count", 3)
	createTimeout := intParam(tr, "vm_create_timeout", 300)

	rc.Logger.Info("virtbench: creating VMs on target node", "node", node, "count", count, "storage_class", sc)

	for i := 1; i <= count; i++ {
		ns := nsPrefix + "-" + strconv.Itoa(i)
		if err := ensureNamespace(ctx, kubeconfig, ns); err != nil {
			return fmt.Errorf("virtbench: create namespace %s: %w", ns, err)
		}
		manifest := strings.NewReplacer(
			"{{VM_NAME}}", vmName,
			"{{NAMESPACE}}", ns,
			"{{NODE_NAME}}", node,
			"{{HOSTNAME_LABEL}}", hostnameLabel,
			"{{STORAGE_CLASS_NAME}}", sc,
		).Replace(failureRecoveryVMTemplate)
		if err := kubectlApply(ctx, kubeconfig, manifest); err != nil {
			return fmt.Errorf("virtbench: apply VM in %s: %w", ns, err)
		}
		rc.Logger.Info("virtbench: VM applied", "namespace", ns, "vm", vmName)
	}

	rc.Logger.Info("virtbench: waiting for VMs to reach Running", "count", count, "timeout_s", createTimeout)
	for i := 1; i <= count; i++ {
		ns := nsPrefix + "-" + strconv.Itoa(i)
		if err := waitForVMRunning(ctx, kubeconfig, ns, vmName, createTimeout); err != nil {
			return fmt.Errorf("virtbench: VM %s/%s not ready: %w", ns, vmName, err)
		}
		rc.Logger.Info("virtbench: VM Running", "namespace", ns, "vm", vmName)
	}

	if boolParam(tr, "kill_node") {
		bag.Set("kill_node", true)
		bag.Set("node_restore_timeout", intParam(tr, "node_restore_timeout", 600))
		// Stored so killNodeFromBag can patch VM affinities after the cordon.
		bag.Set("kill_vm_name", vmName)
		bag.Set("kill_vm_count", count)
	}
	return nil
}

// killNodeFromBag is the Scenario.KillNode hook for failure-recovery. It waits
// 15 s after virtbench starts (giving it time to discover the VMs), then:
//  1. Cordons the node (VMIs with evictionStrategy:None stay put — no premature migration)
//  2. Removes the required-node affinity from the VM objects (so KubeVirt can
//     reschedule VMIs on surviving workers after the failure; the running VMIs are
//     NOT restarted by this patch — KubeVirt defers spec-change restarts)
//  3. Reboots the node immediately (before KubeVirt can act on the spec change)
func killNodeFromBag(ctx context.Context, rc *core.RunCtx, bag *core.Bag) {
	if kill, _ := core.GetAs[bool](bag, "kill_node"); !kill {
		return
	}
	node, _ := core.GetAs[string](bag, "picked_node")
	if node == "" {
		return
	}
	kubeconfig, _ := core.GetAs[string](bag, "kubeconfig")
	rc.Logger.Info("virtbench: waiting 15s for virtbench to discover VMs before killing node", "node", node)
	select {
	case <-ctx.Done():
		return
	case <-time.After(15 * time.Second):
	}

	cordonNode(ctx, kubeconfig, node, rc)

	vmName, _ := core.GetAs[string](bag, "kill_vm_name")
	nsPrefix, _ := core.GetAs[string](bag, "ns_prefix")
	count, _ := core.GetAs[int](bag, "kill_vm_count")
	if vmName != "" && nsPrefix != "" && count > 0 {
		removeVMAffinities(ctx, kubeconfig, nsPrefix, vmName, count, rc)
	}

	if err := rebootNode(ctx, kubeconfig, node, rc); err != nil {
		rc.Logger.Warn("virtbench: node reboot failed", "node", node, "err", err)
		return
	}
	bag.Set("killed_node", node)
}

// pickWorkerNode returns a randomly chosen Ready worker node name from the
// cluster, plus its kubernetes.io/hostname label value (used for node affinity
// in the VM template — in OpenShift the node name can be an FQDN while the
// label is the short hostname, so they must be fetched separately).
func pickWorkerNode(ctx context.Context, kubeconfig string) (name, hostnameLabel string, err error) {
	out, ok, err := runKubectl(ctx, kubeconfig, "get", "nodes",
		"-l", "node-role.kubernetes.io/worker",
		"-o", "jsonpath={.items[*].metadata.name}")
	if !ok || err != nil {
		return "", "", fmt.Errorf("virtbench: list worker nodes: %w", err)
	}
	allNodes := strings.Fields(strings.TrimSpace(out))
	if len(allNodes) == 0 {
		return "", "", fmt.Errorf("virtbench: no worker nodes found (label node-role.kubernetes.io/worker)")
	}
	var ready []string
	for _, n := range allNodes {
		cond, condOK, _ := runKubectl(ctx, kubeconfig, "get", "node", n,
			"-o", `jsonpath={.status.conditions[?(@.type=="Ready")].status}`)
		if !condOK || strings.TrimSpace(cond) != "True" {
			continue
		}
		sched, _, _ := runKubectl(ctx, kubeconfig, "get", "node", n,
			"-o", `jsonpath={.metadata.labels.kubevirt\.io/schedulable}`)
		if strings.TrimSpace(sched) != "true" {
			continue
		}
		ready = append(ready, n)
	}
	if len(ready) == 0 {
		return "", "", fmt.Errorf("virtbench: no Ready+schedulable worker nodes found (all %d workers are not Ready or kubevirt.io/schedulable != true)", len(allNodes))
	}
	name = ready[rand.Intn(len(ready))] //nolint:gosec
	label, labelOK, _ := runKubectl(ctx, kubeconfig, "get", "node", name,
		`-o`, `jsonpath={.metadata.labels.kubernetes\.io/hostname}`)
	if labelOK && strings.TrimSpace(label) != "" {
		hostnameLabel = strings.TrimSpace(label)
	} else {
		hostnameLabel = name
	}
	return name, hostnameLabel, nil
}

// waitForVMRunning polls until the VMI reaches phase Running or timeoutSec elapses.
func waitForVMRunning(ctx context.Context, kubeconfig, ns, vmName string, timeoutSec int) error {
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		out, ok, _ := runKubectl(ctx, kubeconfig, "get", "vmi", vmName, "-n", ns,
			"-o", "jsonpath={.status.phase}")
		if ok && strings.TrimSpace(out) == "Running" {
			return nil
		}
		time.Sleep(10 * time.Second)
	}
	return fmt.Errorf("VMI %s/%s not Running after %ds", ns, vmName, timeoutSec)
}

// ensureNamespace creates a namespace if it does not already exist.
func ensureNamespace(ctx context.Context, kubeconfig, ns string) error {
	return kubectlApply(ctx, kubeconfig, "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: "+ns+"\n")
}

// cordonNode marks the node unschedulable. The flag survives reboots (stored in
// etcd), so rescheduled VMIs cannot land back on it.
func cordonNode(ctx context.Context, kubeconfig, node string, rc *core.RunCtx) {
	rc.Logger.Info("virtbench: cordoning node before reboot", "node", node)
	if _, _, err := runKubectl(ctx, kubeconfig, "cordon", node); err != nil {
		rc.Logger.Warn("virtbench: cordon failed — VMs may reschedule on same node", "node", node, "err", err)
	}
}

// removeVMAffinities clears the required-node affinity from each VM object.
// Called after cordon so the cordoned node is already excluded from scheduling;
// KubeVirt defers spec-change restarts, so the running VMIs are not affected
// until the node fails and they are recreated from the updated VM spec.
func removeVMAffinities(ctx context.Context, kubeconfig, nsPrefix, vmName string, count int, rc *core.RunCtx) {
	for i := 1; i <= count; i++ {
		ns := nsPrefix + "-" + strconv.Itoa(i)
		if _, _, err := runKubectl(ctx, kubeconfig, "patch", "vm", vmName, "-n", ns,
			"--type=merge", "-p", `{"spec":{"template":{"spec":{"affinity":null}}}}`); err != nil {
			rc.Logger.Warn("virtbench: could not clear VM affinity", "namespace", ns, "vm", vmName, "err", err)
		}
	}
	rc.Logger.Info("virtbench: released node affinity from VMs", "count", count)
}

// rebootNode forces an immediate reboot via `oc debug` then polls until the node
// reports NotReady so virtbench starts monitoring an already-downed node.
func rebootNode(ctx context.Context, kubeconfig, node string, rc *core.RunCtx) error {
	rc.Logger.Info("virtbench: forcing reboot on node", "node", node,
		"cmd", "oc debug node/"+node+" -- chroot /host systemctl reboot -ff")
	args := []string{"debug", "node/" + node, "--", "chroot", "/host", "systemctl", "reboot", "-ff"}
	if kubeconfig != "" {
		args = append([]string{"--kubeconfig", kubeconfig}, args...)
	}
	cmd := exec.CommandContext(ctx, "oc", args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("virtbench: start node reboot for %s: %w (is 'oc' on PATH?)", node, err)
	}
	go func() { _ = cmd.Wait() }() // connection drops when node reboots — non-zero exit expected

	// Poll until node is NotReady so virtbench sees a downed node immediately.
	rc.Logger.Info("virtbench: waiting for node NotReady", "node", node)
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		out, ok, _ := runKubectl(ctx, kubeconfig, "get", "node", node,
			"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
		if ok && strings.TrimSpace(out) == "False" {
			rc.Logger.Info("virtbench: node NotReady — handing off to virtbench", "node", node)
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	// Not fatal — virtbench's own --node-timeout will handle the wait.
	rc.Logger.Warn("virtbench: node not yet NotReady after 120s; virtbench will continue waiting", "node", node)
	return nil
}

// waitForNodeReady polls until the node reports Ready=True or timeoutSec elapses.
// Called from Teardown so the cluster is healthy before namespace/PVC deletion.
func waitForNodeReady(ctx context.Context, kubeconfig, node string, timeoutSec int, rc *core.RunCtx) {
	rc.Logger.Info("virtbench: waiting for node to recover after reboot", "node", node, "timeout_s", timeoutSec)
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return
		}
		out, ok, _ := runKubectl(ctx, kubeconfig, "get", "node", node,
			"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
		if ok && strings.TrimSpace(out) == "True" {
			rc.Logger.Info("virtbench: node is Ready — proceeding with cleanup", "node", node)
			return
		}
		time.Sleep(10 * time.Second)
	}
	rc.Logger.Warn("virtbench: node not Ready after timeout; proceeding with cleanup anyway (PVCs may need manual force-detach)", "node", node, "timeout_s", timeoutSec)
}

// uncordonNodeLogged removes the cordon set before the reboot so the node resumes
// accepting workloads after the test.
func uncordonNodeLogged(ctx context.Context, kubeconfig, node string, rc *core.RunCtx) {
	if _, _, err := runKubectl(ctx, kubeconfig, "uncordon", node); err != nil {
		rc.Logger.Warn("virtbench: uncordon failed — node may still be unschedulable", "node", node, "err", err)
		return
	}
	rc.Logger.Info("virtbench: uncordoned node", "node", node)
}
