# 0009. Scoped setup: run / group / individual

- **Status:** Accepted
- **Date:** 2026-08-24

## Context

The first live virtbench run (brnocp, Pure `pure-flasharray-block`) showed that
some setup is not per-test. virtbench's boot-storm/datasource-clone scenarios all
need one shared `ssh-test-pod` (for ping / in-VM SSH), and they register as three
separate `automation_tool`s → three separate jobs. Per-job `Provision`/`Teardown`
would create and destroy that pod three times for a batch of virtbench tests.

More generally, setup lives at three levels and the harness only modeled one:

- **run ("all")** — once per harness run (a cluster-wide precondition).
- **group** — once per named group of tests that share expensive setup (a helper
  pod today; a golden base image / snapshot tomorrow).
- **individual** — per-tool / per-TR.

Only individual existed (the `Provisioner` stage, which runs once per tool-job
with that job's TRs). We needed run and group scopes so shared setup is done once
and never duplicated. Jira: ECOPROJECT-5331.

## Decision

1. **`stages.SetupProvider`** — `SetupInfo() {Scope, Key}`, `Setup(ctx, rc, trs)`,
   `Teardown(ctx, rc, trs)`. Scope is `run` or `group`. Providers register via
   `registry.RegisterSetup` and are deduped by `(scope, key)`, so a group is
   declared once and referenced by many tools.
2. **`ToolIntegration.SetupGroups []string`** — the group keys a tool's tests
   belong to. A group provider runs iff ≥1 runnable TR belongs to a tool listing
   its key.
3. **Orchestrator ordering** — after TR selection: run each active provider's
   `Setup` once, ordered **run → group**; then the per-tool job loop; then
   `defer` each `Teardown` once in reverse (**group → run**). Each provider gets
   its in-scope TRs (run: all runnable; group: the group's TRs) to read
   kubeconfig/backend. A failed `Setup` emits `error` verdicts for its in-scope
   TRs and skips their jobs (no partial runs); a provider whose `Setup` failed is
   not torn down.
4. **Individual scope stays the `Provisioner` stage** — it already runs once per
   job with that job's TRs; no new mechanism. The three scopes are run (new),
   group (new), individual (existing).

## Options Considered

- **Scoped `SetupProvider` in the core (chosen)** — (+) models all three tiers;
  shared setup runs exactly once; generalizes to any tool and any expensive
  resource. (−) a small core addition (stages + registry + orchestrator).
- **Idempotent create-if-absent in each scenario's Provision** — (−) sequential
  jobs each tear down between runs, so the resource is recreated per scenario;
  doesn't actually share, and shared-teardown timing is unsolved.
- **Collapse virtbench scenarios into one integration** — (−) breaks the
  registry-key = `automation_tool` exact match (ADR-0007/0008) and needs
  orchestrator-level dispatch.

## Consequences

- virtbench registers one group provider (`Key: "virtbench"`) that ensures the
  `ssh-test-pod` (create-if-absent; delete only if it created it). A batch of N
  virtbench tests creates the pod once and removes it once. Verified live: a
  boot-storm smoke ran with **zero manual steps** — adapter created the pod, the
  VMs graded, virtbench `--cleanup` removed its namespaces, the pod was removed.
- Replay mode skips shared setup (no cluster).
- New precedent: expensive shared setup (golden images, RBAC, operators) becomes
  a `SetupProvider`, not per-job work.

## Non-Goals

- Making kubeconfig a first-class `RunCtx` field (providers read it from the
  passed TRs for now); in-cluster execution model (ADR-0007).

## References

- ADR-0003 (ports-and-adapters), ADR-0007 (virtbench local-exec), ADR-0008
  (scenario table + unit normalization).
- [ECOPROJECT-5331](https://redhat.atlassian.net/browse/ECOPROJECT-5331)
