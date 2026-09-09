# storage-cert-harness

Storage certification test harness for OpenShift / OpenShift Virtualization. It
orchestrates upstream test tools, grades their results against SLA gates and
functional checks, and emits a certification report.

> **Status:** early scaffold. The architecture and contracts are in place with a
> working reference tool (`example`) that runs offline. Real tool integrations
> and a live-cluster runner are in progress.

## Quickstart

```sh
make build
./bin/harness list --catalog examples/catalog.example.json
./bin/harness run  --catalog examples/catalog.example.json \
                   --thresholds thresholds.example.json \
                   --partner-level 3 --output ./out
```

`thresholds.example.json` contains **fake** numbers. Real SLA thresholds are
never committed in the clear (see below) and are supplied at run time via
`--thresholds <path>` or `HARNESS_THRESHOLDS`.

## How it works

Ports-and-adapters. The KB export **v2.0 two-file contract** drives a run —
`catalog.json` (publishable: per-Test-Requirement metadata) and `thresholds.json`
(sensitive: SLA numbers + check definitions), joined on TR id and required to
share the same `kb_git_commit`. Each tool is an adapter bundle implementing small
stage interfaces (`Preflight → Provision → Run → Collect → Parse`); the core
scores each Test Requirement into one verdict per SLA and per check, then renders
a report. See [`docs/architecture.md`](docs/architecture.md),
[ADR-0003](decisions/0003-harness-architecture-ports-and-adapters.md), and
[ADR-0006](decisions/0006-adopt-kb-export-v2-contract.md).

## Adding a tool

Copy `internal/tools/example/` and follow
[`docs/adapter-authoring.md`](docs/adapter-authoring.md).

## Secret handling

SLA threshold numbers must never be published by Red Hat. This repo commits only
fake examples + JSON Schemas; the real bundle is gitignored and (per
[ADR-0004](decisions/0004-threshold-secret-handling.md)) will live only in an
encrypted folder decryptable by Red Hat developers.

## Layout

See [`AGENTS.md`](AGENTS.md) for the directory map and contributor rules (internal
GitLab only), [`docs/ci.md`](docs/ci.md) for CI, and [`decisions/`](decisions/)
for architecture decision records.

## Release artifact

The harness **ships as a container image** ([ADR-0011](decisions/0011-harness-ships-as-container-image.md)):
`quay.io/eco-special-projects/storage-cert-harness:<image_version>` (`:main` on the
default branch). The image includes `harness`, `kube-burner-ocp`, and `virtbench`
on `PATH`. `harness version` reports the **binary** version track;
the image tag uses the **image** version track. See [`docs/ci.md` Versioning](docs/ci.md#versioning).

`make build` still produces a local `bin/harness` for development. CI
build/scan/push: [`docs/ci.md`](docs/ci.md).

## CI

Lint, unit tests, secret-scan, supply-chain, offline replay-smoke, binary build
(linux/amd64 artifact), and container image build/scan (podman locally):
[`docs/ci.md`](docs/ci.md). One-time: `./ci/scripts/install-tools.sh`.

## License

Apache-2.0 (see `LICENSE`).

## Report metrics

Reports include measurements matching each test's catalog `sla` descriptors by
default, even when no threshold is set. A descriptor with a percentile includes
only that percentile; one without includes all measured percentiles of that name.
Tests without metric descriptors have no default measurements.

Add informational metrics for every selected test with
`--report-metric extra_metric,latency@p99` (the flag can be repeated).
To add metrics for individual tests, set them in the plan:

```yaml
report_metrics:
  TR-STOR-006:
    - podReady@p99
    - extra_metric
```

CLI and plan requests are additive. A bare metric name includes all its measured
percentiles; `name@percentile` selects one. Missing requested metrics produce
warnings in the logs and JSON, Markdown, and JUnit reports. They do not change
verdicts or the exit status. Selection only controls report measurements: grading
still receives every parsed metric, and required SLA failures retain their
existing behavior. Extras must already be produced by the tool; requesting a
metric does not enable additional collection.

## Named plan variants

A plan can run a selected TR with several parameter combinations:

```yaml
schema_version: "1"
name: pvc-density-variants
select:
  ids: [TR-STOR-006]
variants:
  TR-STOR-006:
    - name: small
      params:
        iterations: 10
    - name: large
      params:
        iterations: 100
```

The workload sizes above are illustrative. Parameter precedence is catalog
defaults → plan `overrides` → variant `params`. A variant list replaces that
TR's default execution; TRs without variants still run once. Names must be unique
within a TR and contain letters, digits, underscores or hyphens, starting with a
letter or digit. Empty lists and unknown catalog TR IDs are errors. Selection
filters still apply; variants do not select additional TRs.

Each variant runs the full pipeline with a fresh state bag and a separate artifact
directory under `variants/<hex-encoded-TR-ID>/<hex-encoded-variant-name>/`.
Variants of a tool run sequentially under its existing scheduling constraints;
shared run/group setup still runs once. Ordinary TRs retain their existing batch
execution. The grader matches `when` against each variant's resolved parameters.

JSON verdicts and measurements carry an optional `variant` field alongside the
original `tr` ID. Markdown and JUnit identify executions as `TR-ID [variant]`;
all variants contribute to certification grading. Keep the plan with the report
for parameter provenance: resolved parameter maps are not serialized, since tool
parameters may include sensitive configuration. Existing plans need no changes.

When using `--continue-from`, distinct variants are retained. Repeating the same
TR, scenario and variant replaces its earlier results.
