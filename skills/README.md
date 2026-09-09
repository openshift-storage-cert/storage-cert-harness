# skills/

**Internal only.** Do not copy this overlay onto public GitHub — it is an AI
instruction surface (see [docs/ci.md](../docs/ci.md)).

AI-assisted development workflow overlay, modeled on osac-workspace's phased
skills but scoped to this single repo. These are **scaffolding to be filled in**.

Skills:

- `add-test/` — add a certification test (TR) end to end: KB → harness adapter →
  real cluster run. The comprehensive per-Test-Requirement flow (**filled in**).
- `okf/` — create Google Open Knowledge Format bundles for agent context (TR, tool,
  workload, plans, playbooks). See `skills/okf/SKILL.md`.
- `design/` — turn a problem into an ADR + task breakdown (scaffold).
- `implement/` — pick up a task, plan, TDD, publish a PR (scaffold).

Each skill directory holds a `SKILL.md` with frontmatter (`name`,
`description`). See the top-level `AGENTS.md` for project conventions the skills
should follow.
