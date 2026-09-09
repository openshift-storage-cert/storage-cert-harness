# Cluster Setup Guide for Partners

This guide walks a storage partner through preparing an OpenShift cluster to
run the storage certification harness from scratch.

---

## Prerequisites Checklist

Complete every item before you begin. The harness `preflight` command also
verifies most of these automatically.

- [ ] OpenShift cluster **4.20 or later**, multi-node (3 control-plane + ≥ 3 workers)
- [ ] SNO cluster **not** used
- [ ] Worker nodes meet hardware minimums (see [cluster-requirements.md](cluster-requirements.md#hardware-minimums))
- [ ] `cluster-admin` credentials available (kubeconfig or token)
- [ ] `oc` / `kubectl` installed on the machine that will run the harness
- [ ] `podman` (≥ 4.x) installed on the machine that will run the harness
- [ ] Storage array is cabled / routed; all worker nodes can reach it
- [ ] `multipathd` enabled and configured on all worker nodes (iSCSI and FC plans)
- [ ] `iscsid` enabled on all worker nodes (iSCSI plans only)
- [ ] CSI driver installed and **all driver pods are Running/Ready**
- [ ] `StorageClass` for the storage under test exists in the cluster
- [ ] `VolumeSnapshotClass` created and exists in the cluster
- [ ] CDI scratch space `StorageClass` (filesystem-mode, backed by the storage under test) created and configured (VM / CDI test plans)
- [ ] `thresholds.json` obtained out-of-band from the Red Hat partner engineering team
- [ ] `backends.yaml` prepared with your array's details (copy `backends.example.yaml`)
- [ ] A test-plan YAML chosen from `plans/` (or a custom plan)
- [ ] Image registry reachable from the execution host (or internal mirror configured)

---

## Step 1 — Confirm a Healthy Qualifying Cluster

Ensure you have an OCP cluster that satisfies all requirements in
[cluster-requirements.md](cluster-requirements.md) before proceeding. Cluster
provisioning itself is out of scope here — refer to the
[official OpenShift installation documentation](https://docs.openshift.com/container-platform/latest/installing/index.html)
if you still need to deploy one.

Verify the cluster is fully healthy:

```bash
export KUBECONFIG=/path/to/kubeconfig

# All nodes Ready
oc get nodes

# No degraded cluster operators
oc get co | grep -v "True.*False.*False"

# No failing Pods in kube-system / openshift-*
oc get pods -A | grep -Ev "(Running|Completed|Succeeded)"
```

Expected: all nodes `Ready`, all cluster operators `Available=True / Degraded=False / Progressing=False`. Resolve any issues before continuing.

---

## Step 2 — Configure Storage Transport Prerequisites

> Skip if your test plan uses NFS or NVMe-oF/TCP only.

For **iSCSI** and **FC** plans, `multipathd` (and `iscsid` for iSCSI) must be
active on all worker nodes before running certification. Configure them via
whichever method suits your setup:

- **Vendor CSI auto-configuration** — some drivers (e.g. HPE / NetApp)
can deploy the required `MachineConfig` objects automatically.
Check your CSI driver's documentation first; if supported, this can be
handled as part of [Step 3](#step-3--install-the-csi-driver-and-define-required-storageclasses).
- **Manual** `MachineConfig` — if your driver does not handle it, apply the
appropriate `MachineConfig` objects yourself. See the
[OCP documentation on enabling iSCSI initiators](https://docs.openshift.com/container-platform/latest/storage/persistent_storage/persistent-storage-iscsi.html)
and your CSI driver's multipath configuration guide for the exact settings.

---

## Step 3 — Install the CSI Driver and Define Required StorageClasses

Install your storage vendor's CSI driver following the vendor's documentation.
After installation, confirm the driver is healthy:

```bash
# All CSI driver pods should be Running
oc get pods -n <csi-driver-namespace>
```

### StorageClass under test

Create the `StorageClass`(es) that the certification plan will target. The
class name(s) must match what you configure in `backends.yaml` (Step 5). Refer
to your CSI driver's documentation for the correct provisioner name and
parameters.

```bash
# Confirm the StorageClass is present and has the correct provisioner
oc get storageclass <your-storage-class> -o wide
```

Annotate a default storage class for OCP/CNV:

```bash
oc annotate storageclass <your-storage-class> \
  storageclass.kubevirt.io/is-default-virt-storageclass="true"
```

### VolumeSnapshotClass

Create a `VolumeSnapshotClass` backed by the same CSI driver and confirm it
exists:

```bash
oc get volumesnapshotclass
```

### CDI scratch space (VM / CDI test plans only)

CDI requires a filesystem-mode `StorageClass` for scratch volumes during import
— block-mode classes are not usable. Create a filesystem-mode `StorageClass`
backed by the storage under test, then set it via `CDIConfig` or through the
`HyperConverged` CR if OpenShift Virtualization is installed:

```bash
# Via CDIConfig directly
oc patch cdiconfig config --type=merge \
  -p '{"spec":{"scratchSpaceStorageClass":"<filesystem-storageclass-name>"}}'
```

```yaml
# Via HyperConverged CR (OpenShift Virtualization)
apiVersion: hco.kubevirt.io/v1beta1
kind: HyperConverged
metadata:
  name: kubevirt-hyperconverged
  namespace: openshift-cnv
spec:
  scratchSpaceStorageClass: "<filesystem-storageclass-name>"
```

**Optional smoke test** — launch a pod with a PVC attached and confirm it mounts
cleanly before investing time in a full certification run.

---

## Step 4 — Configure Node Remediation (HA test plans only)

Install and configure node remediation with your preferred remediation provider.
See [HA Testing — Node Remediation](cluster-requirements.md#ha-testing--node-remediation)
in the cluster requirements for operator choices and guidance.

---

## Step 5 — Prepare the Backend Configuration

Copy the example file and fill in your array's details:

```bash
cp backends.example.yaml backends.yaml
```

Edit `backends.yaml`:

```yaml
backends:
  - name: my-array          # referenced by --backend or plan backend:
    vendor: <vendor-name>
    storage_class: <storage-class-name>
    snapshot_class: <snapshot-class-name>
    params:
      # vendor-specific connection params (e.g. management endpoint, SVM name)
    secrets:
      # use file refs or k8s Secret refs — never plaintext credentials here
      username:
        from_file: /run/secrets/storage-username
      password:
        from_file: /run/secrets/storage-password
```

See [ADR-0005](../decisions/0005-test-plans-and-backend-config.md) for the full
credential-reference syntax. **Do not commit** `backends.yaml` — it is
gitignored.

---

## Step 6 — Obtain `thresholds.json`

The SLA threshold file is sensitive and is provided by the Red Hat partner
engineering team. Place it at a path of your choice (e.g.
`/run/secrets/thresholds.json`) and reference it with `--thresholds` at run
time. Do **not** commit it. See
[ADR-0004](../decisions/0004-threshold-secret-handling.md).

---

## Step 7 — Choose a Test Plan

Pick a pre-made plan from `plans/` or author a custom one:

```bash
ls plans/
# example-smoke.yaml     — quick sanity run, subset of TRs
# example-performance.yaml — full performance tier
```

Open the plan and confirm the `backend:` field matches the name you chose in
`backends.yaml`.

---

## Step 8 — Run Preflight

The `preflight` sub-command validates connectivity, CSI health, and
configuration before committing to a full run:

```bash
bin/harness preflight \
  --kubeconfig "$KUBECONFIG" \
  --plan plans/example-smoke.yaml \
  --backends backends.yaml \
  --backend my-array \
  --thresholds /run/secrets/thresholds.json
```

All preflight checks must pass (exit 0) before proceeding.

---

## Step 9 — Run Certification

```bash
bin/harness run \
  --kubeconfig "$KUBECONFIG" \
  --plan plans/example-performance.yaml \
  --backends backends.yaml \
  --backend my-array \
  --thresholds /run/secrets/thresholds.json \
  --report-dir ./reports
```

When the run completes, `reports/` will contain a machine-readable `report.json`
and a human-readable `report.md`.

---
