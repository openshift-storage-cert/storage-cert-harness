# 0014. Report: run-environment, attestations, level rollup, exporters

- **Status:** Proposed
- **Date:** 2026-09-07

## Context

Per-item verdicts and KB provenance do not describe the tested environment,
partner-declared Level-3 claims, or tier outcome. Submissions and CI also need
JUnit alongside JSON/Markdown.

## Decision

Extend the shared `core.Report` additively, keeping `internal/core`
dependency-free and preserving KB provenance and existing grading.

- **Environment:** collect hardware (nodes/CPU/RAM) and platform facts
  (OCP/CNV, CSI drivers/capabilities, operators, StorageClasses, active backend).
  Every `Fact` carries its source. `internal/clustercheck` collects best-effort;
  the CLI attaches results. Skip collection without a kube CLI so replay and
  unit tests remain usable.
- **Attestations:** accept `{Claim, Value, SignedBy}` from `--attestations`
  through a schema/loader. Only unobservable claims belong here, initially
  Level-3 reference architecture and published SLAs (requirements B/C).
  Observable facts belong in Environment. No claim appears in both; attestations
  are not cross-checked.
- **Level rollup:** group verdicts by `PartnerLevel`. A level passes with at least
  one pass and no fail/error; skip-only means not evaluated. `CertifiedLevel`
  is the highest tested passing level with no lower tested level failing.
  Untested levels are ignored. This is a harness-suite outcome, not cumulative
  product certification: at proposal time the harness TRs were Level 2 and
  external `openshift-origin-csi` supplied Level 1. Cross-suite aggregation
  belongs to its consumer. The CLI can use the summary for its exit verdict.
- **Exporters:** use one `Exporter` interface/shared model for JSON, Markdown,
  and `report.junit.xml`. JUnit has `environment-*` suites with fact properties,
  `level-N` suites with SLA measured/threshold/operator/unit properties, and an
  attestations suite. Map errors to `<error>` and skips to `<skipped>`; place
  properties under suites, not the root, for parser portability.
- **Measurements and duration:** expose every parser metric in `Measurements`
  even without an SLA; matching verdicts separately carry grading. Record each
  verdict's `DurationS` as its TR tool-run wall time. JSON exposes measurements
  and duration; Markdown adds a measurements table/duration column; JUnit adds a
  measurements suite and testcase `time`. Reporting all parser metrics is the
  initial proposal; filtering to catalog-declared metric descriptors is the
  intended follow-up.
- **Badges/features:** reserve fields and render when present; defer scoring.

## Decision notes

### Options considered

- **Shared report model and exporters (chosen):** all formats gain consistent
  data; requires additive core changes.
- **Separate submission model:** risks drift.
- **JUnit-only extension:** leaves JSON/Markdown without the new data.

### Consequences

Reports may show runtime thresholds alongside measurements. This does not relax
[ADR-0004](0004-threshold-secret-handling.md)'s prohibition on committing real
thresholds or change [ADR-0006](0006-adopt-kb-export-v2-contract.md)'s input split.

Signing/tamper evidence, badge scoring, a strict cumulative level ladder, and
cross-suite certification remain outside scope.
