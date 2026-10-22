# OpenShift Virtualization CSI Partner Certification Guide

## Level 3: Performance Validated

This guide describes the end-to-end process for running the storage
certification harness and preparing the results for certification review.

All harness commands run in the published container image
[`quay.io/virtarraycert/storage-cert-harness:main`](https://quay.io/repository/virtarraycert/storage-cert-harness?tab=tags).
You do not need to build the harness locally. Clone this repository on the
machine that runs Podman so you can mount `catalog.json`, `thresholds.json`,
`plans/`, and your `backends.yaml` into the container.

The harness runs the Test Requirements selected by a test plan, collects tool
results, evaluates them against `thresholds.json`, and writes a certification
report.

### Pull and verify the harness image

On the host that will run certification, confirm you can pull the `main` image
from [Quay](https://quay.io/repository/virtarraycert/storage-cert-harness?tab=tags):

```bash
podman pull quay.io/virtarraycert/storage-cert-harness:main
```

> **Note:** We recommend using Podman as the container execution tool. If you
> encounter issues using Docker or Buildah, report them to us.

### Host logs (`logs/`)

Every partner harness command invoked through `make partner-*` or
`./ci/scripts/run-partner-*.sh` writes a **host-side** log under the
`logs/` directory at the repository root. The file captures validation,
`podman pull`, and all stdout/stderr from the wrapper and container for that
invocation.

| Command | Log file |
| --- | --- |
| `make partner-preflight` | `logs/run-partner-preflight.log` |
| `make partner-cert-load-80` | `logs/run-partner-cert-load-80.log` |
| `make partner-cert-no-load-80` | `logs/run-partner-cert-no-load-80.log` |

Before each run, an existing log with the same name is renamed with a UTC
timestamp (for example `logs/run-partner-preflight-20261022T120000Z.log`) so
the latest attempt is always the basename without a timestamp.

Inside the container, `harness run` also writes **`run.log`** under each
profile’s `certification-results/<profile>/report/` directory. For failures,
send both the host log in `logs/` and that profile’s `report/run.log`.

> **Note:** Run **only one harness execution at a time** on a given cluster
> (preflight, certification runs, or any other harness job). Concurrent runs
> against the same cluster are not supported. **This is currently your
> responsibility** to enforce.

---

## 1. Prepare an OpenShift cluster with a storage backend

Prepare a cluster that meets
[`docs/cluster-requirements.md`](docs/cluster-requirements.md):

- OpenShift Virtualization and CDI when required by the certification plan.
- The CSI driver for the array under test installed and healthy.
- A `StorageClass` for the CSI driver under test.
- A `VolumeSnapshotClass` backed by that driver when the plan requires snapshots.
- A filesystem-mode CDI scratch `StorageClass` for VM image and DataVolume workflows when required.

Follow [`docs/cluster-setup-guide.md`](docs/cluster-setup-guide.md) for detailed
cluster preparation.

---

## 2. Prepare the backends file

Create `backends.yaml` in the repository root with one backend entry:

Level 3 certification needs only four fields:

```yaml
schema_version: "1"
backends:
  - name: partner-storage
    storage_class: <storage-class-name>
    snapshot_class: <volume-snapshot-class-name>
```

Use the same `name` as `BACKEND` at run time (section 3). Class names must
match the cluster. Do not add other keys unless Red Hat asks for them.

---

## 3. Customize the Level 3 plans for your partner backend

Three steps: define `backends.yaml`, set `export BACKEND=<name>`, and edit
cluster placeholders in the two Level 3 plan files. Put `storage_class` and
`snapshot_class` only in `backends.yaml` — not in the plan.

**Before the first run:**

1. `BACKEND` matches the `name` in `backends.yaml`.
2. `storage_class` and `snapshot_class` exist in the cluster.
3. Node and snapshot placeholders in `plans/level3-customer-no-load-80.yaml`
   are filled in (below).

All `make partner-*` commands use `BACKEND` and `./backends.yaml` by default.
The shipped plans do not set `backend:`; every partner uses the same plan files.

### What to edit in the plan files (cluster layout)

Edit the shipped plans **in your clone** (same paths the Makefile uses). Do
not change `select.ids` or remove Test Requirements unless Red Hat instructs
you to run a reduced scope.

#### `plans/level3-customer-load-80.yaml` (80% load)

| Location | What to set |
| --- | --- |
| `overrides.TR-STOR-002` | `profile: load-80` is already set. |
| External load | You maintain ~80% load outside the harness during this run only. |

#### `plans/level3-customer-no-load-80.yaml` (no load)

Replace angle-bracket placeholders with values from **your** cluster. Use
worker node names from `oc get nodes` (nodes that will run VMs or be drained).

| Test | Plan field | Purpose |
| --- | --- | --- |
| TR-VIRT-007 | `overrides.TR-VIRT-007.node` | Worker targeted for failure / recovery (FAR). |
| TR-VIRT-007 | `overrides.TR-VIRT-007.far_config` | Leave `/far-config.json` unless Red Hat gives you a different mount path. |
| TR-VIRT-008 | `overrides.TR-VIRT-008.target_node` | Worker to drain for evacuation test. |
| TR-VIRT-013 | `overrides.TR-VIRT-013.node_name` | Worker used for single-node VM ceiling sweep. |
| TR-VIRT-010 | `overrides.TR-VIRT-010` | Uncomment / set `replicas` and `snapshot_count` per certification batch size. |

Example fragment after customization (names illustrative):

```yaml
overrides:
  TR-VIRT-007:
    mode: far-operator
    far_config: /far-config.json
    node: worker-2.example.com
  TR-VIRT-008:
    target_node: worker-1.example.com
    num_vms: 50
    start: 1
    end: 50
  TR-VIRT-013:
    node_name: worker-3.example.com
  TR-VIRT-010:
    replicas: 10
    snapshot_count: 10
```

Leave other `overrides` and `select.ids` as shipped unless Red Hat tells you
otherwise.

### Example

`backends.yaml`:

```yaml
schema_version: "1"
backends:
  - name: acme-afs-csi
    storage_class: acme-csi-sc
    snapshot_class: acme-csi-snapclass
```

```bash
export KUBECONFIG=/path/to/kubeconfig
export BACKEND=acme-afs-csi
```

Then edit node placeholders in `plans/level3-customer-no-load-80.yaml`.

### Preflight after customization

Run preflight against the **no-load** plan (widest tool surface) after backend
and plan edits:

```bash
export PLAN=plans/level3-customer-no-load-80.yaml
make partner-preflight
```

Host validation checks that `storage_class` / `snapshot_class` from
`BACKEND` exist in the cluster and that configured plan nodes exist.

---

## 4. Set kubeconfig in your environment

The harness uses the cluster credentials from `KUBECONFIG` on the host and
inside the container (the run scripts mount your kubeconfig read-only).

```bash
export KUBECONFIG=/path/to/kubeconfig
```

Use a kubeconfig whose identity can create namespaces, workloads, VMs, PVCs, and
snapshots as required by the selected plan.

---

## 5. Confirm you can access the cluster

On the same host that will run Podman, verify API access and node health:

```bash
oc --kubeconfig "$KUBECONFIG" whoami --show-server
oc --kubeconfig "$KUBECONFIG" get nodes
oc --kubeconfig "$KUBECONFIG" get storageclass <storage-class-name>
oc --kubeconfig "$KUBECONFIG" get volumesnapshotclass <volume-snapshot-class-name>
```

The partner helper scripts (`make partner-preflight`, `make partner-cert-*`)
perform the same connectivity and cluster-health checks (API reachable, every
node `Ready`, storage and snapshot classes from `backends.yaml` present) before
starting the container.

---

## 6. Required inputs (from this repository)

Clone the repository and run all commands from its root (example:
`/home/nlevanon/workspace/csi-betterment/storage-cert-harness`).

### Level 3 plans (fixed paths in the repo)

| Profile | Plan file |
| --- | --- |
| **80% load** (data-path under load) | `plans/level3-customer-load-80.yaml` |
| **No load** (all other Level-3 criteria) | `plans/level3-customer-no-load-80.yaml` |

The helper scripts and Makefile targets always use these paths. You do not pass
a different plan for certification unless you are debugging with manual
`podman run` invocations.

### Thresholds (one file for both runs)

Both the load-80 and no-load-80 runs grade against the **same**
`thresholds.json` at the **repository root** (next to `catalog.json`). The
container mounts that file as `--thresholds /inputs/thresholds.json`. Do not
copy or edit thresholds for partner runs; use the file shipped with the repo.

| File | Purpose |
| --- | --- |
| `catalog.json` | Test Requirement metadata and metric descriptors. |
| `thresholds.json` | SLA thresholds for **both** Level-3 runs. |
| `plans/level3-customer-load-80.yaml` | Load-80 certification plan. |
| `plans/level3-customer-no-load-80.yaml` | No-load certification plan. |
| `backends.yaml` | Your backend name, `storage_class`, and `snapshot_class`. |

The catalog and `thresholds.json` must come from the same export. The `run`
command validates their schema version, KB commit, and Test Requirement IDs.
`preflight` does not load `thresholds.json`; SLA gates are applied during
`run` only.

---

## 7. Run preflight

Set the backend name and run preflight (host checks + harness preflight in the
container):

```bash
export KUBECONFIG=/path/to/kubeconfig
export BACKEND=partner-storage

make partner-preflight
```

Or invoke the script directly:

```bash
./ci/scripts/run-partner-preflight.sh
```

### What preflight validates

Preflight is layered:

| Layer | What is checked |
| --- | --- |
| **Host (scripts)** | Podman and `oc` available; kubeconfig readable; API reachable; all nodes `Ready`; `storage_class` and `snapshot_class` from `backends.yaml` exist in the cluster; plan node placeholders that are set match real nodes. |
| **`backends.yaml` (harness)** | File parses; `schema_version`; backend name exists. |
| **Harness `preflight` (container)** | Per selected tool: binaries on image `PATH`; plan parameters; resolved `storage_class` from the backend; CRDs (KubeVirt, CDI, snapshots when needed); storage class present via `oc`/`kubectl` inside the container; snapshot class resolution when tests need snapshots. |

There is no separate check named `connectivity.cluster-health` in the harness
output. Connectivity and cluster health are covered by the host script checks
above and by tool preflight messages such as `storage class "…" present` and CRD
presence lines.

Preflight does **not** evaluate SLA numeric values in `thresholds.json`. Resolve
every `[FAIL]` preflight line before starting certification runs.

To use a different plan for preflight tool selection:

```bash
export PLAN=plans/level3-customer-load-80.yaml
make partner-preflight
```

---

## 8. Run the two certification profiles

Level 3 certification uses two runs. Each run uses root `thresholds.json`, the
same `BACKEND` / `backends.yaml` as preflight, and the plan named in section 6:

1. **Load profile** — `plans/level3-customer-load-80.yaml` while the cluster is
   under sustained external load (~80). The partner operates the load mechanism.
2. **No-load profile** — `plans/level3-customer-no-load-80.yaml` after load is
   removed. Pass the first run’s `report.json` with `--continue-from` so the
   second report merges both runs.

Use separate output trees. Helper scripts archive any previous
`certification-results/<profile>/` directory with a UTC timestamp before each
run (disable with `PARTNER_KEEP_WORK=true`).

```bash
export KUBECONFIG=/path/to/kubeconfig
export BACKEND=partner-storage

# Run 1 — maintain external load during this run.
make partner-cert-load-80

# Stop or remove external load, then Run 2.
make partner-cert-no-load-80
```

Default result layout:

```text
certification-results/
├── load-80/
│   ├── work/          # tool workdir (--workdir)
│   └── report/        # report.json, report.md, report.junit.xml, run.log, …
└── no-load-80/
    ├── work/
    └── report/        # merged certification report
```

The no-load script expects the prior report at
`certification-results/load-80/report/report.json` unless you set
`CONTINUE_FROM`.

### Manual container invocation

Equivalent to the Makefile targets (image tag `main` on
[Quay](https://quay.io/repository/virtarraycert/storage-cert-harness?tab=tags)):

```bash
HARNESS_IMAGE=quay.io/virtarraycert/storage-cert-harness:main
LOAD_REPORT=certification-results/load-80/report
NO_LOAD_REPORT=certification-results/no-load-80/report
mkdir -p "$LOAD_REPORT" "$NO_LOAD_REPORT/work" "$NO_LOAD_REPORT/report"

podman pull "$HARNESS_IMAGE"

# Run 1 (load-80) — mount plans and inputs; see run-partner-cert.sh for full flags.
# Run 2 (no-load-80) — add read-only mount of load-80 report dir and:
#   --continue-from /prior-load/report.json
```

Use a fixed image tag instead of `:main` when your certification instructions
pin a specific version.

---

## 9. Collect output (work, report, and logs)

Each profile writes **two** directories under `certification-results/<profile>/`.
Keep **all files** in both trees for submission and troubleshooting.

| Directory | Contents |
| --- | --- |
| `.../report/` | `report.json`, `report.md`, `report.junit.xml`, `run.log`, and any report-side artifacts the harness writes. |
| `.../work/` | Tool workdir output (kube-burner / virtbench result files, variant subdirs, and other files created during the run). |

After **load-80**:

```text
certification-results/load-80/report/   ← reports and run.log
certification-results/load-80/work/     ← all tool artifacts
```

After **no-load-80**:

```text
certification-results/no-load-80/report/   ← merged certification report + run.log
certification-results/no-load-80/work/     ← all tool artifacts
```

Package everything (both profiles, both `work/` and `report/` trees, plus helper
logs under `logs/`):

```bash
make partner-package-results
# creates certification-results-full.tar.gz in the repository root
```

### If a run fails or exits non-zero

1. Send **`certification-results/<profile>/report/run.log`** — full harness and
   tool output for that profile.
2. Send the matching helper log, for example
   **`logs/run-partner-cert-load-80.log`** or
   **`logs/run-partner-cert-no-load-80.log`** (host Podman wrapper and
   validation output).
3. When asked for a complete bundle, run `make partner-package-results` and
   send **`certification-results-full.tar.gz`** (includes all `work/` and
   `report/` files for both profiles when present).

A non-zero exit can mean failed certification criteria; reports and logs are
still written under `report/` and should be included in what you send to Red
Hat.

---

## 10. Provide attestations

Attestations are self-reported claims the harness cannot measure from the
cluster. You may skip them during `run` (`--no-attestations` in the helper
scripts) and add them later:

```bash
podman run --rm \
  -v "$PWD/attestations.json:/inputs/attestations.json:ro,Z" \
  -v "$NO_LOAD_REPORT:/results:Z" \
  "$HARNESS_IMAGE" attest \
    --report /results/report.json \
    --attestations /inputs/attestations.json \
    --output /results
```

See the attestations JSON shape in the previous revision of this guide or run
`harness attest --help` in the image. When `--continue-from` carries
attestations from the first report, they are merged into the final report.

---

## 11. Create the submission archive

**Minimum JUnit + run log** (when Red Hat asks only for graded results):

```bash
LOAD_DIR=certification-results/load-80/report
NO_LOAD_DIR=certification-results/no-load-80/report

test -s "$LOAD_DIR/report.junit.xml"
test -s "$LOAD_DIR/run.log"
test -s "$NO_LOAD_DIR/report.junit.xml"
test -s "$NO_LOAD_DIR/run.log"

tar -czf certification-results-junit.tar.gz \
  -C certification-results \
  load-80/report/report.junit.xml \
  load-80/report/run.log \
  no-load-80/report/report.junit.xml \
  no-load-80/report/run.log
```

**Full output** (all files under every `work/` and `report/` directory):

```bash
make partner-package-results
```

Submit the archive your Red Hat certification contact requests
(`certification-results-junit.tar.gz`, `certification-results-full.tar.gz`, or
both).

---

## 12. Certification result

Review uses the selected plan, catalog joined with `thresholds.json`, JUnit
results, and run logs. A successful harness run does not by itself grant
certification; Red Hat reviews the submission against applicable requirements.

---

## Makefile reference (partner)

| Target | Purpose |
| --- | --- |
| `make partner-preflight` | Host + harness preflight |
| `make partner-cert-load-80` | Load-80 certification run |
| `make partner-cert-no-load-80` | No-load run with `--continue-from` |
| `make partner-package-results` | Tar all `work/` + `report/` trees and helper logs |

Common environment variables: `KUBECONFIG`, `BACKEND`, `HARNESS_IMAGE`,
`CERT_RESULTS_ROOT`, `PARTNER_KEEP_WORK`, `PLAN` (preflight only).

Host execution logs: see **Host logs (`logs/`)** at the top of this guide.
