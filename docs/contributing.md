# Contributing

## Workflow

The project follows an ADR-driven, small-task SDLC (modeled on
osac-project/osac-workspace, adapted for a single Go repo):

1. **Decide** — significant design choices get an ADR in `decisions/` (copy
   `template.md`), reviewed and merged as a PR before large implementation.
2. **Slice** — break work into small tasks; a single stage adapter (e.g. one
   tool's `ResultParser`) is a good unit.
3. **Implement** — TDD where practical; parsers get golden-file tests.
4. **Review** — PR with review; branch protection on the default branch.

## Git

- Feature branches; open a PR into the default branch.
- AI-assisted commits carry an `Assisted-by:` trailer.
- Nothing secret in commits — ever (ADR-0002 / ADR-0004).

## Local checks before pushing

See [ci.md](ci.md) for the full pipeline, versioning, and image contents.

```sh
pre-commit install          # yaml + md + go + secret-scan + version-lock
./ci/scripts/install-tools.sh
./ci/scripts/ci.sh
make set-next-version BINARY=1.0.0   # or IMAGE=1.0.0; see docs/ci.md Versioning
```

`make unittest` / `./ci/scripts/unittest.sh` are **unit tests only** (no cluster).
`make lint` and `make test` match the GitLab `lint` and `test` stages.

## Partner cluster setup

- [cluster-requirements.md](cluster-requirements.md) — hardware / OCP version / storage spec
- [cluster-setup-guide.md](cluster-setup-guide.md) — step-by-step setup guide and prerequisites checklist

## Adding a tool

See [adapter-authoring.md](adapter-authoring.md).

## Decision records

`decisions/NNNN-title.md` from `decisions/template.md`. Status lifecycle:
Proposed → Accepted → (Deprecated | Superseded by NNNN).
