# Run a Test Suite from a Plan

This is a development smoke run for the QE MTV cluster. It is not a
certification-scale execution.

## Test selection

Use these four IDs in `plans/harness-smoke-plan.yaml` to exercise every
runtime tool shipped in the image:

| Test | Workload | Tool |
| --- | --- | --- |
| `TR-VIRT-001` | Boot Storm | `virtbench` |
| `TR-STOR-006` | PVC provisioning density | `kube-burner-ocp` |
| `TR-VIRT-010` | VM snapshot creation | `kube-burner` |
| `TR-VIRT-019` | Parallel VM lifecycle | `kube-burner-ocp` |

Keep the plan's `backend` and node overrides aligned with the local cluster.
For this environment, `node` and `node_name` must be one of:

```text
openshift-worker-0.ocp-edge3.lab.eng.tlv2.redhat.com
openshift-worker-1.ocp-edge3.lab.eng.tlv2.redhat.com
openshift-worker-2.ocp-edge3.lab.eng.tlv2.redhat.com
```

The plan names the backend `tlvbm`, and the backend configuration must
contain a backend with exactly that name. The supplied local example currently
contains `tlvbm`; rename that entry if `tlvbm` is the intended backend name.

## Run the plan

From `storage-cert-harness`, build the local image and run it against the QE
MTV kubeconfig:

```sh
KUBECONFIG=/home/nlevanon/workspace/csi-betterment/storage-cert-harness/work/kubeconfig \
KUBECTL_INSECURE_SKIP_TLS_VERIFY=true \
make image-build-local
```

Then run the selected plan. The command mounts the repository's work files,
the KB export, the local backend configuration, and a writable output folder:

```sh
podman run --rm --network host --userns=keep-id \
  --user "$(id -u):$(id -g)" \
  -v "$(pwd)/work/kubeconfig:/kubeconfig:ro,Z" \
  -e KUBECONFIG=/kubeconfig \
  -v "$(pwd)/work:/work:ro,Z" \
  -v "$(pwd)/plans:/plans:ro,Z" \
  -v "$(pwd)/work/backends.local.yaml:/backends.yaml:ro,Z" \
  -v "$(pwd)/out/harness-smoke:/output:Z" \
  "$(tr -d '[:space:]' < work/local-image-ref)" run \
  --plan /plans/harness-smoke-plan.yaml \
  --catalog /work/catalog.json \
  --thresholds /work/thresholds.json \
  --backends /backends.yaml \
  --backend tlvbm \
  --workdir /output/work \
  --output /output \
  --no-attestations \
  --verbose
```

Reports and tool artifacts are written to `out/harness-smoke`. The Make target
`KUBECONFIG=... make run-harness-smole` performs the same preflight checks and
runs this plan.

## Validate the cluster first

```sh
export KUBECONFIG=/home/nlevanon/workspace/csi-betterment/storage-cert-harness/work/kubeconfig
export KUBECTL_INSECURE_SKIP_TLS_VERIFY=true
oc whoami --show-server
oc whoami
oc get nodes -l node-role.kubernetes.io/worker
oc get clusterversion
oc get storageclass rhos-san-iscsi
oc get volumesnapshotclass csi-trident-volumesnapshotclass
```

Use `oc config view --minify` to inspect the active context and cluster
endpoint. `oc config get-contexts` lists available contexts, and
`oc config current-context` shows which one the kubeconfig selects. Do not
commit the kubeconfig or backend credentials.
