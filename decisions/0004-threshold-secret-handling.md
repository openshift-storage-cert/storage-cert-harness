# 0004. SLA threshold secret handling: encrypted-in-repo, shared symmetric key

- **Status:** Accepted (mechanism deferred)
- **Date:** 2026-08-23

## Context

The harness needs real SLA thresholds at runtime, but they must never be
published in plaintext, code, or readable Git history. Developers also need a
practical way to obtain matching thresholds.

## Decision

Real thresholds may be committed **only in a dedicated encrypted folder**, using
a shared symmetric key held only by Red Hat developers. Commit fake
`thresholds.example.json` and schemas in plaintext for other contributors.

**Encryption is not yet enabled.** Select a mechanism, key custody, and rotation
process before committing any real thresholds. Until then, keep real files
outside tracked content and gitignore the local `thresholds.json`.

Decryption produces a local file explicitly supplied through `--thresholds` /
`HARNESS_THRESHOLDS`; there is no implicit default. Stamp its provenance into
every report. Never commit the key or ship it in the container.

## Decision notes

### Options considered

- **Encrypted copy (chosen):** keeps data with code and reduces KB retrieval;
  requires key management.
- **Gitignore-only:** keeps secrets out of history but requires separate fetches.
- **Authenticated runtime service:** revocable access, but adds infrastructure
  and credentials.

### Consequences

Ciphertext remains in history permanently: a later key leak can expose old
versions. The selected tool must support per-file encryption and documented
rotation. Tool selection (git-crypt, SOPS+age, or transcrypt), custody, and the
relationship between the canonical KB and encrypted copies/exporter remain open.
Partner threshold delivery is outside this decision.

Related: [ECOPROJECT-5273](https://redhat.atlassian.net/browse/ECOPROJECT-5273),
[ADR-0002](0002-repository-host-and-sdlc-workflow.md).
