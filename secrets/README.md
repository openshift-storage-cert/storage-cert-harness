# secrets/

Reserved for the **encrypted SLA thresholds folder** decided in
[ADR-0004](../decisions/0004-threshold-secret-handling.md). The mechanism
(git-crypt vs SOPS+age vs transcrypt, key custody, rotation) is **not yet
implemented**.

Rules until then:

- **No real SLA numbers anywhere in this repo**, including here, until the
  encryption mechanism is in place.
- Only ciphertext will ever be committed under this folder; the decrypted output
  goes to `secrets/plain/`, which is gitignored.
- The symmetric key is held by Red Hat developers only and is never committed or
  baked into the container image.

For development now, use the fake `thresholds.example.json` at the repo root.
