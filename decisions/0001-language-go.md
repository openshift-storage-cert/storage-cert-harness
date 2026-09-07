# 0001. Harness implementation language: Go

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

The certification harness orchestrates upstream test frameworks — virtbench,
kube-burner-ocp, kubevirt-storage-checkup, and the openshift/origin CSI suite —
nearly all of which are written in Go. The harness interacts heavily with the
Kubernetes API, must ship as a small container image (and ideally a single
static binary), and is distributed to external partners.

Language choice was tracked as ECOPROJECT-5328 and initially postponed while the
team weighed ecosystem fit against team expertise. See the deliberation log
(`harness-foundational-decisions.md`) for the full pluses/minuses.

## Decision

The harness is written in **Go**.

Module path: `gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness`
(renamed on the GitHub migration — see [0002](0002-repository-host-and-sdlc-workflow.md)).

## Options Considered

- **Go (chosen)** — (+) matches the upstream ecosystem, first-class `client-go`
  for Kubernetes, compiles to a single static binary → tiny container, easy
  partner distribution. (−) more boilerplate than Python; some team ramp-up.
- **Python** — (+) fast to write, aligns with the KB tooling (MkDocs/Python).
  (−) weak Kubernetes client story, must shell out to Go tools anyway, ships an
  interpreter/heavier container.
- **Hybrid (Go core + Python glue)** — (+) best-of-both on paper. (−) two
  toolchains, two CI pipelines, two skill sets — cost outweighs benefit for a
  small team on a tight timeline.

## Consequences

- Single static binary → minimal container image, simple partner distribution.
- In-process Kubernetes access via `client-go`; no shelling out for cluster ops.
- Upstream Go tools can be invoked as released images (see
  [0003](0003-harness-architecture-ports-and-adapters.md)); their source is not
  vendored.
- Establishes Go module layout, `go test` + golden-file conventions, and Go
  linting in CI.

## Non-Goals

- Does not choose the CLI framework, k8s client version, or build tooling —
  those are implementation details settled in scaffolding. Shipping as a
  container image is recorded in
  [0011](0011-harness-ships-as-container-image.md).

## References

- [ECOPROJECT-5328](https://redhat.atlassian.net/browse/ECOPROJECT-5328)
- Deliberation log: `harness-foundational-decisions.md`
- [0011](0011-harness-ships-as-container-image.md)
