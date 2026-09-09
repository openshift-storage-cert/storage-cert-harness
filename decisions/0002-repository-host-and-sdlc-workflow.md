# 0002. Repository host and SDLC workflow

- **Status:** Accepted
- **Date:** 2026-08-23

## Context

The harness needs an internal development home before public release. Public
history must never contain readable SLA thresholds or other secrets.

## Decision

- Start on internal GitLab, then migrate/mirror to public GitHub. Treat history
  as public from the first commit. Rename the Go module at migration.
- Use reviewed PRs, branch protection, security scanning, and ADRs in
  `decisions/`. AI-assisted commits carry an `Assisted-by:` trailer.
- Keep tool-agnostic instructions in `AGENTS.md`, a thin `CLAUDE.md` wrapper,
  and a `skills/` workflow for design → implementation → e2e. These stay internal
  under [ADR-0010](0010-ci-scripts-and-multi-host.md).
- Commit schemas and fake `thresholds.example.json`; gitignore real
  `thresholds.json` and sensitive KB artifacts. Review secret hygiene on every
  change. [ADR-0004](0004-threshold-secret-handling.md) governs encryption.
- Keep CI portable. ADR-0010 supersedes the original Makefile-owned logic with
  `ci/scripts/` implementations and thin local/GitLab/GitHub wrappers.

## Decision notes

### Options considered

- **GitLab → GitHub (chosen):** starts before public-org access is ready;
  requires migration work.
- **GitHub immediately:** avoids migration but delays the start.
- **GitLab only:** simpler, but fails the public-program requirement.

### Consequences

Migration requires a module rename and public-export review. The original
one-week migration target was a schedule estimate, not an architectural rule.

Threshold delivery, the certification portal, and detailed release branching
remain separate decisions.

Related: [ECOPROJECT-5329](https://redhat.atlassian.net/browse/ECOPROJECT-5329),
[ECOPROJECT-5273](https://redhat.atlassian.net/browse/ECOPROJECT-5273).
