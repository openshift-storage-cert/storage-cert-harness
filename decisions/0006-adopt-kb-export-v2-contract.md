# 0006. Adopt KB export v2.0 two-file contract

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

The KB export team (ECOPROJECT-5273) locked a concrete **v2.0 two-file
contract** the harness must consume. It replaces the placeholder shapes the
skeleton scaffolded (`TestSpec` + a single `assertion` + `Thresholds.Gates`),
which could not represent a real Test Requirement's multiple SLA gates and
mixed functional checks.

The contract:

- Two files, **`catalog.json`** (safe to publish) and **`thresholds.json`**
  (sensitive, delivered at test time, never committed in the clear — ADR-0004).
- Both carry `schema_version` (`"2.0"`) and `provenance {kb_git_commit,
  kb_commit_date, generated_date}`, and a `test_requirements[]` array sorted by
  `id`. They are **joinable on `id`** and MUST share the same `kb_git_commit`;
  a mismatch means one file is stale.
- The unit of certification is the **Test Requirement (TR)**, not a single
  test. A TR carries `partner_level` (1|2|3 — the primary filter, "tier under
  test"), an `automation_tool`, and in the thresholds file `sla[]` (0..N
  quantitative gates) + `checks[]` (0..N everything-else, dispatched by
  `kind`: capability | suite | scenario | manual).
- New scoring rules: `sla.value == null` → skip (bar not set);
  `status == to-be-validated` → skip (not yet scorable, never pass);
  `check.kind == manual` → human sign-off (not auto-scorable).

## Decision

1. **TR-centric domain model** (`internal/core`). Replace `TestSpec` /
   `AssertionType` / `Threshold` / `Thresholds.Gates` with `TestRequirement`
   (catalog metadata + `SLAs []SLA` + `Checks []Check`), `SLA` (`Value *float64`
   so null is distinct from 0), and `Check`. `Provenance` becomes
   `{KBGitCommit, KBCommitDate, GeneratedDate}`. A `TestResult` is keyed by
   `TRID` and carries `Metrics []Metric` (with `Percentile`) plus
   `Checks map[string]Outcome`.
2. **Two loaders + a join** (`internal/catalog`, `internal/thresholds`,
   `internal/kb`). Each loader validates `schema_version == "2.0"` and unique
   ids; `kb.Join` verifies matching schema_version + `kb_git_commit` and 1:1 id
   parity, then merges into `[]core.TestRequirement`. `kb.FromCatalog` yields
   catalog-only TRs (empty SLAs/Checks) for `list`/`preflight`.
3. **Verdict per scorable item.** The grader emits one `Verdict{TR, Item, ...}`
   per SLA (`sla:<metric>[@<percentile>]`) and per check (`check:<id>`), so a
   multi-gate TR reports each gate independently.
4. **Selection by partner level.** `Filter{Tools, IDs, PartnerLevels}`;
   `partner_level` is the primary tier filter. Plans select via
   `partner_levels` / `tools` / `ids`.
5. **Scheduling hints live in code + plan, not the KB.** The v2.0 contract has
   no `parallel_safe`/`exclusivity_groups`. A `ToolIntegration` declares its
   defaults; a plan's `scheduling:` block overrides them per run.
6. **No-automation TRs skip as not-scorable.** A TR whose `automation_tool` is
   empty, `manual`, or not a registered integration never forms a job — the
   orchestrator emits skip verdicts with a clear reason (never pass/fail).

## Options Considered

- **TR-centric model (chosen)** — (+) matches the real contract 1:1; represents
  multi-SLA/multi-check TRs; join detects stale exports. (−) wholesale rewrite
  of core/ingestion/grading.
- **Keep the per-test `TestSpec` and flatten each sla/check into a synthetic
  test** — (+) smaller diff. (−) loses the TR as the reporting/selection unit;
  re-derives partner_level per synthetic row; fights the export shape forever.
- **Single merged KB file** — (+) no join step. (−) violates the publish/secret
  split (ADR-0004): catalog must be shippable, thresholds must not.

## Consequences

- The old assertion model is gone (no dual maintenance). Tools now emit
  `Metrics` (with percentile) + a `Checks` outcome map; a `ToolIntegration` may
  still supply a custom per-TR `Evaluator` keyed by TR id for TRs whose scoring
  it owns.
- `run` refuses to proceed when a selected TR has scorable criteria
  (`sla_count`/`checks_count` > 0) but no thresholds bundle was supplied.
- Reports gain `schema_version` + provenance and a `TR | Item | ...` verdict
  table.
- CLI flags: `--partner-level` replaces `--tr`/`--tag`; `--catalog` now points
  at `catalog.json`.
- Schemas renamed/rewritten: `catalog.schema.json` (was tests-catalog),
  `thresholds.schema.json`, `report.schema.json`, `plan.schema.json`.

## Non-Goals

- Real tool integrations, `internal/kube` client-go (k8s Secret refs still
  deferred — ADR-0005), and the threshold encryption mechanism (ADR-0004).
- Full JSON Schema enforcement at load time (loaders do structural checks;
  schema files are the reference contract).

## References

- [ECOPROJECT-5273](https://redhat.atlassian.net/browse/ECOPROJECT-5273)
- Refines ADR-0003 (architecture / data model) and ADR-0004 (secret handling);
  builds on ADR-0005 (plans + backend config).
