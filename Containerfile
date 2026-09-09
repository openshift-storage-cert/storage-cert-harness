# Python runtime is required by the packaged virtbench FIO CLI. The harness
# binary must be built before this image build and is copied from bin/harness.
# ARG defaults must match ci/config/images.env (ci/scripts/sync-images-yml.sh --check).
# Keep the harness build version distinct from the base image version.
ARG HARNESS_VERSION=0.0.0-dev
ARG IMAGE_VERSION=0.0.0-dev
ARG TARGETARCH=amd64
ARG BUILD_IMAGE=registry.access.redhat.com/ubi9/python-311:9.8
ARG RUNTIME_IMAGE=registry.access.redhat.com/ubi9/python-311:9.8
ARG KUBE_BURNER_OCP_VERSION=v1.12.3
ARG OPENSHIFT_CLIENT_VERSION=4.22.11
ARG VIRTBENCH_VERSION=v2.0.0

FROM ${BUILD_IMAGE} AS builder
ARG TARGETARCH
ARG KUBE_BURNER_OCP_VERSION
USER 0
WORKDIR /src

RUN set -eux; \
    ver="${KUBE_BURNER_OCP_VERSION#v}"; \
    case "${TARGETARCH}" in amd64) tool_arch=x86_64 ;; arm64) tool_arch=arm64 ;; *) echo "unsupported TARGETARCH=${TARGETARCH}" >&2; exit 1 ;; esac; \
    curl -sSfL "https://github.com/kube-burner/kube-burner-ocp/releases/download/${KUBE_BURNER_OCP_VERSION}/kube-burner-ocp-V${ver}-linux-${tool_arch}.tar.gz" \
      | tar xz -C /tmp kube-burner-ocp; \
    install -m 0755 /tmp/kube-burner-ocp /kube-burner-ocp

FROM ${RUNTIME_IMAGE} AS virtbench-builder
ARG TARGETARCH
ARG OPENSHIFT_CLIENT_VERSION
ARG VIRTBENCH_VERSION
USER 0
RUN set -eux; \
    mkdir -p /opt/virtbench; \
    curl -sSfL "https://github.com/portworx/kubevirt-benchmark/archive/refs/tags/${VIRTBENCH_VERSION}.tar.gz" \
      | tar xz --strip-components=1 -C /opt/virtbench; \
    python3 -m venv /opt/virtbench-venv; \
    /opt/virtbench-venv/bin/pip install --no-cache-dir /opt/virtbench; \
    /opt/virtbench-venv/bin/virtbench --help >/dev/null; \
    test "$(/opt/virtbench-venv/bin/virtbench --version)" = "virtbench, version ${VIRTBENCH_VERSION#v}"

RUN set -eux; \
    archive="openshift-client-linux-${TARGETARCH}-rhel9-${OPENSHIFT_CLIENT_VERSION}.tar.gz"; \
    mkdir -p /tmp/openshift-client; \
    curl -sSfL "https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/${OPENSHIFT_CLIENT_VERSION}/${archive}" \
      | tar xz -C /tmp/openshift-client; \
    install -m 0755 /tmp/openshift-client/kubectl /kubectl

FROM ${RUNTIME_IMAGE}
ARG IMAGE_VERSION
ARG VIRTBENCH_VERSION
ARG HARNESS_VERSION
USER 0
RUN dnf install -y podman && dnf clean all
LABEL org.opencontainers.image.version="${IMAGE_VERSION}"
LABEL io.storage-cert-harness.virtbench.version="${VIRTBENCH_VERSION}"
LABEL io.storage-cert-harness.harness.version="${HARNESS_VERSION}"
COPY bin/harness /usr/bin/harness
COPY --from=builder /kube-burner-ocp /usr/bin/kube-burner-ocp
COPY --from=virtbench-builder /opt/virtbench /opt/virtbench-runtime
COPY --from=virtbench-builder /opt/virtbench-venv /opt/virtbench-venv
COPY --from=virtbench-builder /opt/virtbench-venv/bin/virtbench /usr/bin/virtbench
COPY --from=virtbench-builder /kubectl /usr/bin/kubectl
COPY container-entrypoint.sh /usr/local/bin/container-entrypoint.sh
RUN mkdir -p /home/harness/.config /home/harness/.local/share/containers \
  && chmod 0755 /usr/local/bin/container-entrypoint.sh \
  && chown -R 65532:65532 /opt/virtbench-runtime /home/harness
ENV PATH="/opt/virtbench-venv/bin:${PATH}"
ENV HOME=/home/harness
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/container-entrypoint.sh"]
