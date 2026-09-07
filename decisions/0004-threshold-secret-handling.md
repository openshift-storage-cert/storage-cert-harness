# 0004. SLA threshold secret handling: encrypted-in-repo, shared symmetric key

- **Status:** Accepted (mechanism deferred)
- **Date:** 2026-08-23

## Context

ECOPROJECT-5273 requires that the SLA threshold numbers are never *officially
published* by Red Hat — they must not appear in any public repo, in code, or in
readable git history. The harness still needs those numbers at run time to grade
a certification, and Red Hat developers need convenient access to real numbers
while building and testing the harness.

An earlier stance (see `harness-foundational-decisions.md`) ruled out a
committed encrypted file and relied on gitignoring the real `thresholds.json`.
This ADR revisits that: gitignoring alone means every developer must separately
obtain the numbers (e.g. via KB access), and they never live with the code.

## Decision

Real SLA numbers may exist in the repository **only inside a dedicated encrypted
folder**, decryptable with a **shared symmetric key** held by Red Hat developers
and no one else. Anyone who clones the public repo without the key sees only
ciphertext.

- The committed fake `thresholds.example.json` and the JSON Schema stay in the
  clear for contributors without the key.
- The runtime loading contract is unchanged: the harness reads real numbers via
  `--thresholds <path>` / `HARNESS_THRESHOLDS`, and decryption produces that
  file locally. No magic default; provenance is logged into every report.
- **Mechanism is deferred.** Tool selection (git-crypt vs SOPS+age vs
  transcrypt), key custody, and rotation are a follow-up before any real number
  is committed. Until then, no real numbers enter the repo at all.

## Options Considered

- **Encrypted folder + shared symmetric key (chosen)** — (+) numbers live with
  the code; low friction for RH devs; opaque to outside downloaders. (−)
  ciphertext enters permanent git history; a key leak retroactively exposes it;
  needs a key-custody + rotation process.
- **Gitignore real file, KB as sole source (prior stance)** — (+) nothing secret
  ever enters history. (−) numbers never live with the code; every dev fetches
  separately.
- **Runtime fetch from an authenticated RH endpoint** — (+) revocable, always
  current, nothing in the repo. (−) requires building/operating a service and
  run-time credentials.

## Consequences

- The repo gains an encrypted folder + a decrypt step in the dev workflow and
  (optionally) CI; contributors without the key build against the fake example.
- Ciphertext-in-history is permanent — the chosen tool must support per-file
  encryption and a documented key-rotation procedure, and the key must never be
  committed or shipped in the container image.
- **Open follow-up:** reconcile canonical source of truth — the KB (per 0002 /
  5273) versus this in-repo encrypted copy — and whether the KB exporter writes
  into the encrypted folder.

## Non-Goals

- Selecting the specific encryption tool or key-custody process (follow-up).
- Customer/partner delivery of thresholds (owned by 5273).

## References

- [ECOPROJECT-5273](https://redhat.atlassian.net/browse/ECOPROJECT-5273)
- [0002](0002-repository-host-and-sdlc-workflow.md),
  [0003](0003-harness-architecture-ports-and-adapters.md)
- Deliberation log: `harness-foundational-decisions.md`
