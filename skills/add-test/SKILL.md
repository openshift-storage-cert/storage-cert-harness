---
name: add-test
description: >
  Add a certification test (Test Requirement) to the storage-cert-harness end to
  end — from a KB Test Requirement to a real cluster run. Covers identifying the
  tool/workload, verifying the tool's real CLI + output from upstream source,
  resolving or building the runner image, capturing a golden fixture, implementing
  the adapter, wiring thresholds from the KB, running against a real cluster, and
  proving params are wired. Usage: /add-test <TR-ID | test name | Jira ticket>
metadata:
  version: "1.0"
  type: skill
  tags: [harness, adapter, test-requirement, kube-burner, virtbench, e2e]
  platform:
    claude-code:
      user-invocable: true
      argument-hint: "<TR-ID | test name | Jira ticket>"
      allowed-tools: [Bash, Read, Write, Edit, Agent]
---

# add-test — add a certification test to the harness, end to end

Distilled from the real dv-clone (TR-STOR-005) implementation, **including the
traps that cost us time**. The two repos:

- **KB** (`csi-certification-kb`) — source of truth for Test Requirements (TRs) and
  SLA bars; produces the v2.0 export (`catalog.json` + `thresholds.json`).
- **Harness** (this repo) — execution/grading engine; tool adapters live under
  `internal/tools/<tool>/`.

Read `docs/harness-and-kb-data-flow.md` first. **Do not duplicate adapter mechanics
here** — follow `docs/adapter-authoring.md` and the per-tool output reference (e.g.
`docs/kube-burner-dv-clone-output.md`). This skill is the *orchestration + traps*.

**Input:** a TR id (e.g. `TR-STOR-006`), a test name, or a Jira ticket.

---

## Phase 1 — Identify: TR → tool + workload
- Read the KB TR: `csi-certification-kb/concepts/test-requirements/<TR>.md`. Capture
  what it validates and its **"key metrics"** (these drive what the parser must emit).
- Split `automation_tool` (`"<tool> <workload>"`) into **tool** and **workload**.
- ⚠️ **Trap: wrapper vs base tool.** `kube-burner-ocp` ≠ `kube-burner`; the base
  tool/image will NOT have the wrapper's workloads. Confirm the workload belongs to
  the tool you think it does.
- ⚠️ **Trap: exact-match resolution.** The orchestrator resolves a tool via
  `registry.Get(tr.AutomationTool)` — an **exact** string match. The harness example
  catalog slice must set `automation_tool` to the registered tool **Name** (trimmed,
  no workload suffix); the workload goes in the **plan** overrides.
- (Optional) find the Jira ticket (project ECOPROJECT). Ticket titles often differ
  from the TR (e.g. "Volume capacity test" = `pvc-density`).

## Phase 2 — Verify the tool from upstream source (never trust memory/docs)
- Shallow-clone the tool's upstream repo(s). Confirm from source: the workload
  subcommand exists; its **exact flag names + defaults**; the **output files**
  (names), the **measurement name(s)**, the **quantile names**, and the
  `jobSummary` schema.
- ⚠️ **Traps seen:** a flag that doesn't exist (`--metrics-directory`); the output
  directory **baked into the workload YAML** as a *relative* path; a per-workload
  storage-class flag difference (`--storage-class` vs `--storage-class-name`).
- Record findings in `docs/<tool>-<workload>-output.md` (template:
  `docs/kube-burner-dv-clone-output.md`).

## Phase 3 — Resolve the runner image (ADR-0003)
- Does the tool publish a **runnable image**? Check the repo's release/CI config and
  the registry — **use the correct org** (it was `cloud-bulldozer`, not
  `kube-burner`) and note there may be **no `:latest`** (only version tags).
- If an image exists → set the adapter's `DefaultImage` **const** to it (pin a
  tag/digest).
- If **no image exists** (binary-only distribution) → build a **Containerfile
  fallback** that packages the binary/source, and point `DefaultImage` at that. **We
  own the image const — the user never supplies it.**

## Phase 4 — Manual real run → capture the golden fixture
- Run the tool once against a real cluster (small scale) with local indexing.
  Confirm prereqs on the cluster (CDI / KubeVirt / storage class).
- Copy the resulting output dir into the adapter's `testdata/` — it becomes the
  **golden fixture** and the authoritative schema. (Beats a synthetic fixture.)

## Phase 5 — Implement / extend the adapter (switch-style; see adapter-authoring)
Touch points (mirror the sibling workload): `params.go` (workload params + a
workload-aware `validate`), `runner.go` (`buildCLIArgs` case + global flags + podman
mounts + **`-w` container workdir** for the relative output dir), `preflight.go`
(per-workload prereq via `internal/clustercheck`), `parse.go` (`Parse<Workload>` +
`parser.Parse` switch), registration (`Provides` += TR, evaluator, `DefaultImage`),
wiring (example catalog **trimmed**, plan(s), Makefile targets, blank import in
`cmd/harness/main.go`), fixtures + golden test.
- **Metric-name contract:** the parser's `core.Metric.Name` must equal the KB
  `sla:` `metric:` name, or grading won't line up.
- **Native-or-graded evaluator:** grade SLAs/Checks when present; else emit a native
  verdict from the tool's own success.

## Phase 6 — Parser golden tests

- `go test ./internal/tools/<tool>/...` with committed `testdata/` fixtures covers
  collect→parse logic with **no cluster**. Run it before touching a cluster.

## Phase 7 — Parse-coverage diff

- Diff the **full** real output against what the parser emits. Decide which signals
  to surface (all quantiles, throughput, provenance/params echo) — don't stop at one
  metric. Update the parser + golden test.

## Phase 8 — Thresholds (KB side)

- Inspect the TR's `sla:` block. The KB exporter (`make export`) publishes **only
  authored bars** into `thresholds.json` — it does **not** invent them.
- Bars exist → `make export`; the harness grades them automatically (metric-name
  contract).
- `sla: []` (to-be-validated) → grade **native-only** for now, or author bars in the
  KB from the measured run (`/refine-tr`) then export.
- ⚠️ **Never** put real SLA numbers in the harness repo or its history (ADR-0002/0004).

## Phase 9 — Live full-flow run

- Run the harness against the real cluster (`make run-<tr>` with a `KUBECONFIG`).
  Verify preflight → provision → run → collect → parse → grade → report, exit 0, and
  the expected verdict.

## Phase 10 — Prove params + cleanup + land

- Change params via plan overrides; prove they took effect (the tool echoes them in
  `jobSummary.workloadFlags`) and re-run.
- Ensure GC/teardown ran; remove stray host output dirs.
- Update docs; commit with the `Assisted-by:` trailer; keep thresholds out of harness
  history.

---

## References

- `docs/harness-and-kb-data-flow.md` — the two-repo contract
- `docs/adapter-authoring.md` — the adapter how-to (don't duplicate it here)
- `docs/kube-burner-dv-clone-output.md` — per-tool output reference template
- ADRs: 0003 (architecture), 0006 (KB v2.0 contract), 0004 (secret handling)

## Maintenance

This skill was distilled from the dv-clone (TR-STOR-005) build. When you use it, log
the actual steps taken and reconcile against these phases — add anything missing,
delete anything redundant.
