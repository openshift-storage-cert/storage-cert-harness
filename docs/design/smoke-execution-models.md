# Live smoke execution models (A / B / C / GitHub)

How harness **live cluster smoke** reaches the CI lab (edge-112). GitHub Actions
**hosted** runners cannot reach the private OpenShift API.

| Model | Doc | Status |
| --- | --- | --- |
| **A — Contributor laptop** | [smoke-execution-model-a.md](smoke-execution-model-a.md) | **Required** for pre-PR validation |
| **B — GitLab smoke on `main`** | [smoke-execution-model-b.md](smoke-execution-model-b.md) | **Adopted** (design) — mirror [GitHub `main`](https://github.com/openshift-storage-cert/storage-cert-harness), run smoke on lab |
| **C — Lab smoke executor VM** | [smoke-execution-model-c.md](smoke-execution-model-c.md) | **Adopted** — cron/systemd without GitLab |
| **GitHub — GHA self-hosted runner** | [smoke-execution-model-github.md](smoke-execution-model-github.md) | **Not applicable** (org policy) |

Parent design: [storage-cert-harness-ci-smoke-test.md](storage-cert-harness-ci-smoke-test.md).

Jira umbrella: [ECOPROJECT-5406](https://redhat.atlassian.net/browse/ECOPROJECT-5406).

- Model **B** (GitLab): smoke pipeline on GitHub `main` (phase 1: create GitLab project).
- Model **C**: [ECOPROJECT-5612](https://redhat.atlassian.net/browse/ECOPROJECT-5612).
