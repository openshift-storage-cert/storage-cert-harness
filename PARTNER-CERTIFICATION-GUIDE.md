# OpenShift Virtualization CSI Partner Certification Guide

## Level 3: Performance Validated

This guide describes the end-to-end process for running the storage
certification harness and preparing the results for certification review.

The harness runs the Test Requirements selected by a test plan, collects the
tool results, evaluates the results against `thresholds.json`, and writes a
certification report.

## 1. Prerequisites

Prepare the following before starting a certification run:

- An OpenShift cluster that meets the requirements in
  [`docs/cluster-requirements.md`](docs/cluster-requirements.md).
- OpenShift Virtualization and CDI when required by the selected plan.
- The CSI driver and the StorageClass under test installed and healthy.
- A `VolumeSnapshotClass` backed by the CSI driver when required by the plan.
- A filesystem-mode CDI scratch StorageClass for VM image and DataVolume
  workflows.
- A kubeconfig with sufficient permissions to run the selected tests.
- Podman on the machine that runs the harness and access to the required tool
  images.
- A writable directory for the run output and tool artifacts.

Complete the cluster preparation in
[`docs/cluster-setup-guide.md`](docs/cluster-setup-guide.md) before running
certification tests.

## 2. Required inputs

The run uses four input files:

| File | Purpose |
| --- | --- |
| `catalog.json` | Test Requirement metadata and metric descriptors. |
| `thresholds.json` | SLA thresholds and functional check definitions. This file is provided in the repository with the certification inputs. |
| A plan from `plans/` | Selects the Test Requirements and supplies run parameters. |
| `backends.yaml` | Names the storage backend and identifies the StorageClass under test. |

The catalog and thresholds files must come from the same export. The harness
checks their schema version, KB commit, and Test Requirement IDs before running
tests.

Do not put plaintext backend credentials in `backends.yaml`. Backend credentials
are referenced from files or Kubernetes Secrets. See
[`backends.example.yaml`](backends.example.yaml) for the supported structure.

## 3. Configure the storage backend

Copy the example backend file and create an entry for the storage under test:

```bash
cp backends.example.yaml backends.yaml
```

At minimum, set the backend name and StorageClass:

```yaml
schema_version: "1"
backends:
  - name: partner-storage
    vendor: <storage-vendor>
    storage_class: <storage-class-name>
    snapshot_class: <volume-snapshot-class-name>
```

Use the backend name in the plan or pass it with `--backend partner-storage`.
The StorageClass must already exist in the target cluster.

## 4. Select a test plan

Choose the plan supplied for the certification run and review its `select`,
`backend`, and `overrides` sections. The plan determines which Test
Requirements run and how each tool is parameterized.

Plans named `smoke` or `level3-min` are reduced coverage plans for pipeline and
adapter validation. They are not substitutes for the full certification plan.

The certification `run` command validates the repository-provided catalog and
thresholds together, including their schema version, KB commit, and
Test Requirement IDs. No separate validation command is required.

## 5. Run preflight

Set `KUBECONFIG` in the environment used to run the harness:

```bash
export KUBECONFIG=/path/to/kubeconfig
```

Run preflight with the selected plan and backend:

```bash
./bin/harness preflight \
  --catalog ./catalog.json \
  --plan ./plans/<certification-plan>.yaml \
  --backends ./backends.yaml \
  --backend partner-storage
```

Resolve every preflight error before starting the certification run.

## 6. Run the two certification profiles

The Level 3 workflow has two runs:

1. Run the load profile with the `load-80` plan while the cluster is under a
   sustained load of 80. The partner chooses and operates the mechanism that
   creates and maintains that load.
2. Stop or remove that external load, then run the corresponding no-load plan.
   Pass the first run's `report.json` with `--continue-from` so the second
   report contains both runs.

The order is not significant to report merging when the runs have distinct
scenario labels. The sequence below is recommended because it starts with the
load test and then removes the load, but the harness can merge either run first.
Keep the scenario labels distinct: if two runs produce the same Test
Requirement, item, scenario, and variant key, the later run replaces the
earlier result.

Use separate output directories so that both runs' reports, logs, and tool
artifacts are preserved:

```bash
LOAD_DIR="$PWD/certification-results/load-80"
NO_LOAD_DIR="$PWD/certification-results/no-load-80"
mkdir -p "$LOAD_DIR" "$NO_LOAD_DIR"

# Run 1: maintain the required external load while this plan runs.
./bin/harness run \
  --catalog ./catalog.json \
  --thresholds ./thresholds.json \
  --plan ./plans/level3-customer-load-80.yaml \
  --backends ./backends.yaml \
  --backend partner-storage \
  --scenario load-80 \
  --workdir "$LOAD_DIR" \
  --output "$LOAD_DIR" \
  --verbose

# Stop/remove the external load before starting Run 2.

# Run 2: no external load. Merge this run with Run 1's report.
./bin/harness run \
  --catalog ./catalog.json \
  --thresholds ./thresholds.json \
  --plan ./plans/level3-customer-no-load-80.yaml \
  --backends ./backends.yaml \
  --backend partner-storage \
  --scenario no-load-80 \
  --continue-from "$LOAD_DIR/report.json" \
  --workdir "$NO_LOAD_DIR" \
  --output "$NO_LOAD_DIR" \
  --verbose
```

`--continue-from` takes the path to the earlier run's `report.json`; it does
not skip the current run. It reuses the prior report's environment and
attestations, then merges the current run's verdicts and measurements into the
new report. The harness rejects a prior report for a different backend or KB
commit.

The container image can be used instead of a local binary. Run the container
once for each plan with the corresponding output directory mounted. For the
second run, additionally mount the first output directory read-only and pass
the container path to its report:

```bash
# In the second container invocation:
podman run --rm --network host \
  -v "$PWD/catalog.json:/inputs/catalog.json:ro,Z" \
  -v "$PWD/thresholds.json:/inputs/thresholds.json:ro,Z" \
  -v "$PWD/plans/level3-customer-no-load-80.yaml:/inputs/plan.yaml:ro,Z" \
  -v "$PWD/backends.yaml:/inputs/backends.yaml:ro,Z" \
  -v "$KUBECONFIG:/kubeconfig:ro,Z" \
  -v "$NO_LOAD_DIR:/results:Z" \
  -v "$LOAD_DIR:/prior-load:ro,Z" \
  -e KUBECONFIG=/kubeconfig \
  quay.io/eco-special-projects/storage-cert-harness:main \
  run \
    --catalog /inputs/catalog.json \
    --thresholds /inputs/thresholds.json \
    --plan /inputs/plan.yaml \
    --backends /inputs/backends.yaml \
    --backend partner-storage \
    --continue-from /prior-load/report.json \
    --workdir /results \
    --output /results \
    --verbose
```

Use the image version required by the certification instructions instead of
`:main` when a fixed image version is specified.

## 7. Check the run output

After a completed run, the output directory contains:

```text
certification-results/
├── load-80/
│   ├── report.json
│   ├── report.md
│   ├── report.junit.xml
│   ├── run.log
│   └── <tool result artifacts>
└── no-load-80/
    ├── report.json          # merged report from both runs
    ├── report.md
    ├── report.junit.xml    # merged JUnit report
    ├── run.log
    └── <tool result artifacts>
```

Each `report.junit.xml` is a machine-readable result for that run. The
no-load report is the merged certification report. Each `run.log` contains the
harness log and the stdout/stderr emitted by the integrated tools. Use
`--verbose` (or `-v`) to include debug-level harness messages, including the
currently running and still-queued test list. Normal progress messages report
completed and remaining test counts; the final progress message lists passed,
failed, errored, and skipped tests.

Either command may return a failure status when one or more certification
criteria fail. Review the reports and logs in both output directories; reports
are written before the certification failure status is returned.

## 8. Provide attestations

Attestations are self-reported claims that the harness cannot measure from the
cluster. They are optional during the test runs: you may skip them and add
them to the completed report later. The built-in interactive questions require
the URLs for the partner's reference architecture and published SLAs.

For a normal `run` command:

- In an interactive terminal, the harness prompts for attestations.
- Use `--no-attestations` to skip the prompts and add the attestations later.
- Use `--attestations ./attestations.json` to read answers from a file.

The file must have this shape:

```json
{
  "signed_by": "<person or organization>",
  "claims": {
    "reference_architecture_url": "https://example.invalid/reference-architecture",
    "published_slas_url": "https://example.invalid/published-slas"
  }
}
```

When `--continue-from` points to a report that already contains attestations,
the harness carries those attestations into the merged report. Otherwise, it
uses the attestations supplied for the current run.

Attestations can also be added after testing without rerunning any tools:

```bash
./bin/harness attest \
  --report "$NO_LOAD_DIR/report.json" \
  --attestations ./attestations.json \
  --output "$NO_LOAD_DIR"
```

This rewrites `report.json`, `report.md`, and `report.junit.xml` in the output
directory and writes `attestations.json`. It does not change the existing
`run.log` or tool artifacts. If the output directory is omitted, it defaults to
the directory containing the input report.

## 9. Create the submission archive

Create an archive containing the JUnit report and complete run log:

```bash
test -s "$LOAD_DIR/report.junit.xml"
test -s "$LOAD_DIR/run.log"
test -s "$NO_LOAD_DIR/report.junit.xml"
test -s "$NO_LOAD_DIR/run.log"

tar -czf certification-results.tar.gz \
  -C "$PWD/certification-results" \
  load-80/report.junit.xml \
  load-80/run.log \
  no-load-80/report.junit.xml \
  no-load-80/run.log
```

Keep the Markdown and JSON reports and the tool artifacts with the run for
diagnostics. Include them in the archive if the Red Hat certification team
requests the complete run directory.

When you want the results evaluated for certification, submit
`certification-results.tar.gz` to your Red Hat certification contact or team.

## 10. Certification result

Certification review uses the selected plan, the joined catalog and
`thresholds.json`, the JUnit results, and the run log. A successful harness
execution alone does not grant certification; the submitted results must be
reviewed by Red Hat against the applicable certification requirements.
