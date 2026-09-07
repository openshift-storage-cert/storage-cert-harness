# 0007. virtbench integration: local-exec runner + replay mode

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

virtbench (portworx/kubevirt-benchmark) is the first real tool integration
(TR-VIRT-002, VM DataSource clone at scale). ADR-0003 says we run tools from
their **released container images** and only fall back to a `Containerfile` when
no image exists. virtbench ships **no upstream image**: it is a Python CLI
installed via `git clone` + `install.sh` (pip), driving the cluster client-side
with a kubeconfig, and writes JSON/CSV to local disk with `--save-results`.

We also need to develop and grade TR-VIRT-002 with **no live cluster** — for the
first end-to-end pipeline demo, for parser/grader iteration, and for re-grading a
past run against updated SLAs.

Jira: ECOPROJECT-5273 (KB integration), 5331 (first e2e).

## Decision

1. **Phase-1 runner = local-exec.** The virtbench integration runs the
   `virtbench` CLI on the harness host via `os/exec`
   (`exec.CommandContext`), reading kubeconfig/UUID/scale/storage-class from the
   TR's params and the active backend's storage class. `ToolIntegration.Image`
   is left empty. An in-cluster **Job from a self-built `Containerfile`** (with
   RBAC) is the documented follow-up when we need in-cluster execution.
2. **Replay mode.** A TR param `results_dir` points at a directory already
   containing a virtbench `summary_vm_creation_results.json`. When set, Preflight
   skips the binary/cluster checks, the Runner skips exec, and the LogCollector +
   ResultParser grade the pre-collected results. This makes the whole pipeline
   runnable offline and enables re-grading.
3. **Metric-name contract.** The parser maps virtbench summary keys to the KB SLA
   metric names verbatim: `clone_duration_sec → clone_duration`,
   `running_time_sec → time_to_running`, `ping_time_sec → time_to_ping`. A test
   guards this so the grader's metric lookup keeps matching TR-VIRT-002's `sla[]`.
4. **Registry key = the KB `automation_tool` string.** The integration registers
   as `"virtbench datasource-clone"` so `registry.Get(tr.AutomationTool)`
   resolves it with no orchestrator change. Each virtbench *scenario* (fio,
   migration, vm-ops, …) is a distinct integration keyed by its own
   `automation_tool` string; they may share a common exec/parse helper later.

## Options Considered

- **Local-exec first (chosen)** — (+) unblocks TR-VIRT-002 e2e immediately; no
  image to build/host; matches how virtbench is actually distributed and run
  (client-side). (−) deviates from ADR-0003's image-first rule; needs virtbench
  installed on the host; teardown of cluster leftovers is manual for now.
- **Build a Containerfile + in-cluster Job now** — (+) consistent with ADR-0003;
  reproducible runtime. (−) significant RBAC/manifest/plumbing work before the
  first grade; premature while the parser/grader contract is still settling.
- **One virtbench integration dispatching all scenarios internally** — (+) single
  package. (−) needs orchestrator-level `automation_tool → integration` resolution
  and a fat Provides list; rejected in favor of one integration per scenario,
  which keeps `registry.Get` an exact match.

## Consequences

- Adding the remaining virtbench scenarios is now "another integration keyed by
  its `automation_tool` string, reusing the shared helpers."
- Offline e2e and re-grading are first-class via `results_dir`; every parser is
  still golden-testable with a committed fixture.
- Follow-ups: (a) `internal/kube` client + a Containerfile/in-cluster Job model;
  (b) real teardown of `virtbench-*` namespaces; (c) capture virtbench's CSV
  detail files, not just the summary.
- Secret hygiene unchanged: the runner builds its command line from
  params/backend config only — never from a threshold value — and results
  fixtures carry measured values, never SLA gates.

## Non-Goals

- The in-cluster execution model, live k8s client, and encrypted-thresholds
  mechanism (ADR-0004) are not decided here.

## References

- [ECOPROJECT-5273](https://redhat.atlassian.net/browse/ECOPROJECT-5273)
- [ECOPROJECT-5331](https://redhat.atlassian.net/browse/ECOPROJECT-5331)
- ADR-0003 (ports-and-adapters, run-from-image), ADR-0006 (KB export v2.0)
- [kubevirt-benchmark install](https://portworx.github.io/kubevirt-benchmark/install/)
