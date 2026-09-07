# 0014. Report: run-environment, attestations, level rollup, exporters

- **Status:** Proposed <!-- Proposed | Accepted | Deprecated | Superseded by NNNN -->
- **Date:** 2026-09-07

## Context

The report (ADR-0006) is a flat `[]Verdict` + `Summary`, stamped with KB
`Provenance`, rendered to JSON + Markdown. It says *whether each item passed*, but
not:

1. **What/where it ran** — `Provenance` records only the KB origin, not the
   cluster, hardware, OCP/CNV, CSI driver, or StorageClasses under test.
2. **The tier outcome** — `PartnerLevel` exists but the report never groups by it
   or states a certified level.
3. **The unobservable L3 requirements** — Reference Architecture (req B) and
   published SLAs (req C) are partner-declared; nowhere to put them.

Plus: output is JSON/MD only; CI and the L3 submission (req D) want JUnit.

Carried constraints: keep ADR-0006's "provenance stamped into every report";
ADR-0013's "numeric bars only in `thresholds.json`" is about KB *input* and is
untouched (a report legitimately shows measured-vs-bar). `internal/core` stays
dependency-free (ADR-0003).

## Decision

1. **Add `Environment` next to `Provenance`.** Auto-collected run origin:
   `Hardware` (nodes/cpu/ram, source-tagged `Fact`s) + `Platform` (OCP/CNV,
   `CSIDriver` + capabilities, operators, StorageClasses, active backend). Every
   `Fact` carries a `Source` (`oc get …`, `config:backends.yaml`).
   `internal/clustercheck` grows the collectors; `main` stamps it onto the report.

2. **Add `Attestations` — unobservable only, no cross-check.** `{Claim, Value,
   SignedBy}` from a partner-supplied file. Rule: observable → `Environment`
   (measured); not observable → attestation. Nothing is both, so there is
   deliberately no cross-check field. Scope: L3 reqs B and C.

3. **Level rollup + certified level.** `report.Build` groups verdicts by
   `PartnerLevel` into `LevelRollup{Level, Outcome}` and sets
   `Summary.CertifiedLevel` = the highest *tested* level that passed, with no lower
   tested level failing. A level passes iff ≥1 pass and no fail/error; a skip-only
   level is "not evaluated". **Levels not tested are ignored, not treated as
   failures.** Grader/verdicts unchanged.

   *Scope note:* today every harness TR is **Level 2**, and Level 1 is a separate
   external suite (`openshift-origin-csi`) this harness does not run. So a strict
   cumulative ladder (L3 ⇒ L1+L2) is deliberately **not** enforced here: an all-L2
   pass reports certified level 2, and any cross-suite product certification (which
   must fold in the external L1 result) is decided by whoever aggregates the
   suites, not by this report. The level plumbing exists for when L1/L3 TRs land.

4. **Pluggable exporters + JUnit.** Formalize an `Exporter` interface; keep
   `WriteJSON`/`WriteMarkdown` as its implementations; add `JUnitExporter`. `main`
   also writes `report.junit.xml`. JUnit: `environment-*` suites carry facts as
   `<properties>`; one `level-N` suite per level (SLA verdicts attach
   `measured`/`threshold`/`operator`/`unit`; `error`→`<error>`, `skip`→
   `<skipped>`); an `attestations` suite. Facts sit under a `<testsuite>`, not the
   root (parser portability).

5. **Always report measured values + duration.** Every metric a tool produces
   (`TestResult.Metrics`) is surfaced as a `Report.Measurements` entry — VM boot
   time, p99 latency, etc. — **whether or not an SLA bar grades it**. When an SLA
   exists the matching `Verdict` additionally carries the pass/fail. Each verdict
   also records `DurationS` (wall-clock of the TR's tool run) so the report always
   shows how long the test took. Rendered everywhere: JUnit gets a `measurements`
   suite + testcase `time`; Markdown a Measurements table + Duration column; JSON
   `measurements[]` + `duration_s`.

   *Temporary:* surfacing **all** parser-emitted metrics is a first cut. In the
   future the set will be **filtered to the measurements the catalog declares**
   for the TR (i.e. only KB-defined metric descriptors are reported), so the report
   shows the metrics that matter rather than everything the tool happens to emit.

6. **Badges / supported features** — reserve the struct fields; render when
   present. Scoring rules deferred.

## Options Considered

- **Option A (chosen)** — extend `core.Report` + an `Exporter` interface. (+) one
  source of truth; reuses `Verdict`/`SLA`/`Metric`; additive to 0006; all formats
  gain the data. (−) touches core (additively).
- **Option B** — a separate submission struct built from `Report`. (−) parallel
  model that drifts; rejected.
- **Option C** — JUnit-only, no model change. (−) JSON/MD never gain the data;
  attestations get no honest home; rejected.

## Consequences

- Report answers what/where/which-tier; the L3 submission becomes one artifact
  set. `report.json` only gains fields (backward compatible).
- `clustercheck` gains best-effort environment collectors (skipped when no
  kube CLI, so unit tests and replay are unaffected).
- A new `--attestations` input (schema + loader) parallels plans/backends.
- `Summary.CertifiedLevel` gives the CLI a real exit verdict.

## Non-Goals

- Signing / tamper-evidence of the bundle (future).
- Badge/feature **scoring** rules (shape reserved only).
- A strict cumulative level ladder and cross-suite (external L1) certification —
  deferred until L1/L3 TRs exist; today all TRs are L2.
- Any change to grading or the catalog/thresholds contract (0006/0013 stand).

## References

- [decisions/0006](0006-adopt-kb-export-v2-contract.md), [0013](0013-catalog-descriptors-and-when-scoped-thresholds.md), [0004](0004-threshold-secret-handling.md)
- CSI Partner Leveling Framework — levels, badges, features, L3 reqs A–D
- `storage-bettermentation/harness-output-proposals.md` — format exploration
- ECOPROJECT-XXXX (fill in)
