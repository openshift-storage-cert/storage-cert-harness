---
name: okf
description: >
  Create Google Open Knowledge Format (OKF) bundles for storage-cert-harness —
  agent-consumable markdown knowledge graphs with YAML frontmatter. Use when the
  user asks for OKF, open knowledge format, agent knowledge bundle, or wants to
  document a tool/TR/workload for AI context. Not for adapter output refs
  (see add-test) or KB TR authoring (csi-certification-kb).
metadata:
  version: "0.1"
  type: skill
  tags: [okf, knowledge-bundle, harness, agent-context]
  platform:
    claude-code:
      user-invocable: true
      argument-hint: "<TR-ID | tool | workload>"
      allowed-tools: [Bash, Read, Write, Edit, Agent]
---

# okf — Google OKF bundles for the harness

Create **agent-consumable knowledge bundles** in [Google Open Knowledge Format
(OKF) v0.1](https://cloud.google.com/blog/products/data-analytics/how-the-open-knowledge-format-can-improve-data-sharing):
a directory of markdown files with YAML frontmatter, linked by relative paths.

## Do not confuse these three artifacts

| Artifact              | Where                                      | Purpose                                                                 |
| --------------------- | ------------------------------------------ | ----------------------------------------------------------------------- |
| **Google OKF bundle** | `okf/` or `concepts/` in harness (future)  | Agent context graph — TR, tool, workload, plans, playbooks              |
| **Output reference doc** | `docs/<tool>-<workload>-output.md`      | Adapter-authoring ground truth — CLI flags, JSON schema, traps          |
| **KB OKF v0.2**       | `csi-certification-kb/concepts/`           | Source of truth for TRs, SLA bars, partner levels                         |

- Adding a certification test end-to-end → use **`add-test`** (output refs + adapter code).
- Authoring or exporting SLA thresholds → use the **KB** repo, not this skill.
- This skill → synthesize a **Google OKF bundle** from harness + KB sources for agent navigation.

## When to produce a bundle

- A new tool adapter or workload ships and agents need structured context beyond code.
- Cross-linking TR → tool → workload → plans → output schema → playbooks helps onboarding.
- Do **not** duplicate `docs/*-output.md` — link to it from `output-schema/` concepts.

Ship the bundle under `okf/` in the same PR as the adapter when using this skill,
or as a follow-up commit on the same branch.

## Bundle layout (harness convention)

```text
okf/
├── index.md                 # Root progressive-disclosure index
├── log.md                   # Chronological bundle history (optional)
├── test-requirements/
│   ├── index.md
│   └── TR-STOR-006.md
├── adapters/                # or tools/ — harness ToolIntegration
│   ├── index.md
│   └── kube-burner-ocp.md
├── workloads/
│   ├── index.md
│   └── pvc-density.md
├── plans/
│   ├── index.md
│   └── tr-stor-006-pvc-density.md
├── output-schema/           # Raw + parsed result formats
│   ├── index.md
│   ├── jobSummary.md
│   └── latency-quantiles.md
└── playbooks/               # Runnable operator steps
    ├── index.md
    └── run-live-pvc-density.md
```

Adapt directory names to the domain; keep **one concept per file**, path = identity.

## Conformance rules (v0.1)

1. **Every concept file** (all `.md` except `log.md` and top-level `README.md`)
   MUST start with YAML frontmatter containing at minimum `type:`.
2. **Recommended fields:** `title`, `description`, `tags`, `timestamp` (ISO 8601).
3. **Optional:** `resource` (URI or repo path), `sources` (provenance list).
4. **Cross-links:** relative markdown links only — build a navigable graph.
5. **Index files:** use `type: index`; list child concepts with one-line descriptions.
6. **No invented SLA numbers** — document `sla: []` / native grading when bars are TBD.
7. **Human-readable:** plain markdown body; no SDK or special runtime required.

### Harness concept types (producer-defined)

| `type`            | Example path                                      |
| ----------------- | ------------------------------------------------- |
| `index`           | `okf/plans/index.md`                              |
| `test-requirement` | `okf/test-requirements/TR-STOR-006.md`         |
| `automation-tool` | `okf/adapters/kube-burner-ocp.md`                |
| `workload`        | `okf/workloads/pvc-density.md`                    |
| `test-plan`       | `okf/plans/tr-stor-006-pvc-density-smoke.md`      |
| `output-schema`   | `okf/output-schema/latency-quantiles.md`          |
| `pipeline`        | `okf/pipeline/kube-burner-ocp-stages.md`          |
| `playbook`        | `okf/playbooks/run-live-pvc-density.md`             |

Types are lowercase kebab-case for consistency with KB exports.

## Workflow

1. **Read sources** (do not guess):
   - Harness: `internal/tools/<tool>/`, `plans/`, `examples/catalog.*.json`, `Makefile`
   - KB: `csi-certification-kb/concepts/test-requirements/<TR>.md` (if available)
   - Upstream tool source for CLI flags and output file names
2. **Draft concepts** — one file per concept; frontmatter + markdown body.
3. **Wire cross-links** — TR links to adapter, workload, plans, output-schema, playbooks.
4. **Add playbooks** for `make run-*` live targets at minimum.
5. **Add output-schema** concepts — quantile names, metric naming contract, `jobSummary.passed`.
6. **Validate conformance:**

   ```bash
   # Every concept except log.md must have type:
   for f in $(find okf -name '*.md' ! -name log.md ! -name README.md); do
     rg -q '^type:' "$f" || echo "MISSING type: $f"
   done
   ```

7. **Write `okf/log.md`** — date + what was added/changed.

## Concept template

```markdown
---
type: workload
title: pvc-density
description: kube-burner-ocp subcommand for high-density PVC provisioning.
tags: [kube-burner-ocp, pvc, TR-STOR-006]
resource: kube-burner-ocp/pkg/workloads/pvc-density.go
timestamp: 2026-08-26T12:00:00Z
---

# pvc-density

Body: CLI flags, behavior, links to [TR-STOR-006](../test-requirements/TR-STOR-006.md),
[output schema](../output-schema/latency-quantiles.md), [live playbook](../playbooks/run-live-pvc-density.md).
```

## What each section should cover

| Section            | Content                                                              |
| ------------------ | -------------------------------------------------------------------- |
| `test-requirements/` | TR identity, partner level, grading mode (native vs SLA), risk/failure modes |
| `adapters/`        | ToolIntegration name, image, stages, env vars, scheduling            |
| `workloads/`       | Upstream subcommand, flags, harness plan param mapping               |
| `plans/`           | YAML plan purpose, backend, overrides, iteration scale               |
| `output-schema/`   | JSON file names, row shapes, harness `core.Metric` names             |
| `pipeline/`        | Preflight → provision → run → collect → parse flow                   |
| `playbooks/`       | Exact `make` / `bin/harness run` commands, prerequisites, expected exit codes |

## Eval lessons (2026-08-26)

Tested with two agents (with vs without Google OKF article context):

- **With spec context:** better conformance (`type:` on all concepts), stronger
  `output-schema/` and `pipeline/` docs.
- **Without context:** still produced useful `playbooks/` and `log.md`, but missed
  `type:` on index files and guessed wrong spec version.
- **Neither replaces** flat `docs/*-output.md` adapter refs — keep both artifacts.

Reference prototypes (not committed): `/tmp/agentA/`, `/tmp/agentB/okf/`.

## References

- [Google OKF blog post](https://cloud.google.com/blog/products/data-analytics/how-the-open-knowledge-format-can-improve-data-sharing)
- `skills/add-test/SKILL.md` — adapter + output reference workflow
- `docs/adapter-authoring.md` — stage interfaces and golden tests
- KB: `csi-certification-kb/AGENT.md` — **different** OKF schema; do not mix frontmatter
