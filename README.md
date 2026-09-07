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
