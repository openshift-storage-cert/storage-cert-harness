# 0002. Repository host and SDLC workflow

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

The harness needs a home now, but the certification program will publish it
publicly. Tracked as ECOPROJECT-5329. Separately, ECOPROJECT-5273 established
that the SLA threshold numbers must **never** appear in a public repository, in
code, or in git history.

We also want SDLC rigor from day one — architecture decision records, an
AI-assisted development workflow, and CI — modeled on
[osac-project/osac-workspace](https://github.com/osac-project/osac-workspace)
(ADRs in `decisions/`, tool-agnostic `AGENTS.md` + thin `CLAUDE.md`, a `skills/`
overlay, and a phased Feature → Design → Implement → E2E workflow), adapted for
a single Go product repo rather than a meta-workspace.

## Decision

1. **Host:** start on internal GitLab (`gitlab.cee.redhat.com/eco-special-projects`)
   for ~week 1, then mirror/migrate to **public GitHub** (~2026-08-30). Build as
   if public from day one — nothing secret is ever committed, even on GitLab,
   because git history travels with the migration.
2. **Module path:** `gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness`
   now; a single `go mod edit`-driven rename sweep at migration time.
3. **SDLC conventions:**
   - ADRs in `decisions/NNNN-title.md` using `decisions/template.md`.
   - `AGENTS.md` (tool-agnostic instructions) + a thin `CLAUDE.md` wrapper.
   - A `skills/` overlay carrying the phased AI-assisted workflow
     (design → implement → e2e), plus contributor-facing docs.
   - PR-based review with branch protection and security scanning enabled;
     AI-assisted commits carry an `Assisted-by:` trailer.
   - CI logic lives in portable `ci/` scripts; GitLab CI and GitHub Actions are
     thin wrappers (see [0010](0010-ci-scripts-and-multi-host.md)). ADR-0002
     originally kept that logic in the Makefile; 0010 supersedes that bullet.
4. **Secret hygiene:** `schemas/` + fake `thresholds.example.json` are committed;
   the real `thresholds.json` (and any KB-derived artifact deemed sensitive) is
   gitignored. Enforced by `.gitignore` plus review. See
   [0003](0003-harness-architecture-ports-and-adapters.md) and 5273.

## Options Considered

- **GitLab week 1 → public GitHub (chosen)** — (+) start immediately behind the
  firewall, deliberate public cutover. (−) one rename sweep; CI defined twice.
- **Public GitHub from day one** — (+) no migration. (−) org/permissions not
  ready; higher risk of a secret slip before hygiene is proven.
- **GitLab only** — (+) simplest. (−) fails the public-program requirement.

## Consequences

- Contributors need GitLab access in week 1; access shifts to GitHub after
  migration.
- The module-path rename is a known, one-time task tied to the migration.
- Defining CI behavior in `Makefile` targets keeps both CI systems thin.
- Secret hygiene is a standing review responsibility, not just a one-time setup.

## Non-Goals

- Does not decide customer delivery of thresholds or the certification portal
  (owned by 5273 / a later ADR).
- Does not fix the branching model's finer points (release branching, tagging) —
  refined once implementation begins.

## References

- [ECOPROJECT-5329](https://redhat.atlassian.net/browse/ECOPROJECT-5329)
- [ECOPROJECT-5273](https://redhat.atlassian.net/browse/ECOPROJECT-5273)
- Jira story 4.1 (repo + SDLC setup) in `csi-certification-jira-plan.md`
- Model: [osac-project/osac-workspace](https://github.com/osac-project/osac-workspace)
