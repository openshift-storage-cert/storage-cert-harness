# Live smoke execution models (A / B / C)

How harness **live cluster smoke** reaches the CI lab (edge-112) given GitHub
Actions cannot call the private OpenShift API.

| Model | Doc | Status |
| --- | --- | --- |
| **A — Contributor laptop** | [smoke-execution-model-a.md](smoke-execution-model-a.md) | **Required** for pre-PR validation |
| **B — GHA self-hosted runner** | [smoke-execution-model-b.md](smoke-execution-model-b.md) | **Not available** (org policy) |
| **C — Lab smoke executor VM** | [smoke-execution-model-c.md](smoke-execution-model-c.md) | **Adopted** for automated lab smoke |

Parent design: [storage-cert-harness-ci-smoke-test.md](storage-cert-harness-ci-smoke-test.md).

Jira umbrella: [ECOPROJECT-5406](https://redhat.atlassian.net/browse/ECOPROJECT-5406). Model C
implementation: [ECOPROJECT-5612](https://redhat.atlassian.net/browse/ECOPROJECT-5612).
