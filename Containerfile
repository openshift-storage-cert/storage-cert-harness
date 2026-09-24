# Python runtime is required by the packaged virtbench FIO CLI. The harness
# binary must be built before this image build and is copied from bin/harness.
# Image references are supplied by ci/config/images.env through the build scripts.
# Keep the harness build version distinct from the base image version.
ARG HARNESS_VERSION=0.0.0-dev
ARG IMAGE_VERSION=0.0.0-dev
ARG TARGETARCH=amd64
ARG BUILD_IMAGE
ARG SOURCE_BUILD_IMAGE
ARG RUNTIME_IMAGE
ARG KUBE_BURNER_VERSION=v2.8.5
ARG KUBE_BURNER_OCP_VERSION=v1.12.5
ARG KUBE_BURNER_OCP_SOURCE_MODE=release
ARG KUBE_BURNER_OCP_REPOSITORY=https://github.com/kube-burner/kube-burner-ocp.git
ARG KUBE_BURNER_OCP_REF=
ARG KUBE_BURNER_OCP_COMMIT=
ARG OPENSHIFT_CLIENT_VERSION=4.22.15
ARG VIRTBENCH_VERSION=v2.0.0
ARG KUBEVIRT_VERSION=v1.9.0

FROM ${BUILD_IMAGE} AS builder
ARG TARGETARCH
ARG KUBE_BURNER_VERSION
ARG KUBEVIRT_VERSION
USER 0
WORKDIR /src

RUN set -eux; \
  case "${TARGETARCH}" in amd64) kube_arch=x86_64; virtctl_arch=amd64 ;; arm64) kube_arch=arm64; virtctl_arch=arm64 ;; *) echo "unsupported TARGETARCH=${TARGETARCH}" >&2; exit 1 ;; esac; \
    kube_ver="${KUBE_BURNER_VERSION#v}"; \
    curl -sSfL "https://github.com/kube-burner/kube-burner/releases/download/${KUBE_BURNER_VERSION}/kube-burner-V${kube_ver}-linux-${kube_arch}.tar.gz" -o /tmp/kube-burner.tar.gz; \
    tar xz -C /tmp -f /tmp/kube-burner.tar.gz kube-burner; \
    install -m 0755 /tmp/kube-burner /kube-burner; \
    curl -sSfL "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/virtctl-${KUBEVIRT_VERSION}-linux-${virtctl_arch}" \
      -o /tmp/virtctl; \
    install -m 0755 /tmp/virtctl /virtctl; \
    rm -f /tmp/kube-burner.tar.gz /tmp/kube-burner /tmp/virtctl
USER 65532

FROM ${SOURCE_BUILD_IMAGE} AS kube-burner-ocp-builder
ARG TARGETARCH
ARG KUBE_BURNER_OCP_VERSION
ARG KUBE_BURNER_OCP_SOURCE_MODE
ARG KUBE_BURNER_OCP_REPOSITORY
ARG KUBE_BURNER_OCP_REF
ARG KUBE_BURNER_OCP_COMMIT
USER 0
WORKDIR /src

RUN set -eux; \
    case "${TARGETARCH}" in amd64) tool_arch=amd64; archive_arch=x86_64 ;; arm64) tool_arch=arm64; archive_arch=arm64 ;; *) echo "unsupported TARGETARCH=${TARGETARCH}" >&2; exit 1 ;; esac; \
    case "${KUBE_BURNER_OCP_SOURCE_MODE}" in \
      release) \
        ocp_ver="${KUBE_BURNER_OCP_VERSION#v}"; \
        curl -sSfL "https://github.com/kube-burner/kube-burner-ocp/releases/download/${KUBE_BURNER_OCP_VERSION}/kube-burner-ocp-V${ocp_ver}-linux-${archive_arch}.tar.gz" -o /tmp/kube-burner-ocp.tar.gz; \
        tar xz -C /tmp -f /tmp/kube-burner-ocp.tar.gz kube-burner-ocp; \
        install -m 0755 /tmp/kube-burner-ocp /kube-burner-ocp; \
        ;; \
      git) \
        test -n "${KUBE_BURNER_OCP_REPOSITORY}"; \
        test -n "${KUBE_BURNER_OCP_REF}"; \
        git init /src/kube-burner-ocp; \
        git -C /src/kube-burner-ocp remote add origin "${KUBE_BURNER_OCP_REPOSITORY}"; \
        git -C /src/kube-burner-ocp fetch --depth 1 origin "${KUBE_BURNER_OCP_REF}"; \
        git -C /src/kube-burner-ocp checkout --detach FETCH_HEAD; \
        actual_commit="$(git -C /src/kube-burner-ocp rev-parse HEAD)"; \
        if test -n "${KUBE_BURNER_OCP_COMMIT}" && test "${actual_commit}" != "${KUBE_BURNER_OCP_COMMIT}"; then \
          echo "kube-burner-ocp commit mismatch: expected ${KUBE_BURNER_OCP_COMMIT}, got ${actual_commit}" >&2; \
          exit 1; \
        fi; \
        GOPROXY=https://proxy.golang.org,direct GOOS=linux GOARCH="${tool_arch}" CGO_ENABLED=0 \
          make -C /src/kube-burner-ocp ARCH="${tool_arch}" VERSION="custom-${actual_commit}" build; \
        install -m 0755 "/src/kube-burner-ocp/bin/${tool_arch}/kube-burner-ocp" /kube-burner-ocp; \
        ;; \
      *) echo "unsupported KUBE_BURNER_OCP_SOURCE_MODE=${KUBE_BURNER_OCP_SOURCE_MODE} (want release or git)" >&2; exit 1 ;; \
    esac; \
    /kube-burner-ocp --help >/dev/null; \
    rm -rf /src/kube-burner-ocp /tmp/kube-burner-ocp /tmp/kube-burner-ocp.tar.gz
USER 65532

FROM ${BUILD_IMAGE} AS virtbench-builder
ARG TARGETARCH
ARG OPENSHIFT_CLIENT_VERSION
ARG VIRTBENCH_VERSION
USER 0
RUN set -eux; \
    mkdir -p /opt/virtbench; \
    curl -sSfL "https://github.com/portworx/kubevirt-benchmark/archive/refs/tags/${VIRTBENCH_VERSION}.tar.gz" -o /tmp/virtbench.tar.gz; \
    tar xz --strip-components=1 -C /opt/virtbench -f /tmp/virtbench.tar.gz; \
    python3 -m venv /opt/virtbench-venv; \
    /opt/virtbench-venv/bin/python -m pip install --no-cache-dir --upgrade 'pip==26.2.1' 'setuptools>=78.1.1'; \
    /opt/virtbench-venv/bin/pip install --no-cache-dir /opt/virtbench; \
    /opt/virtbench-venv/bin/virtbench --help >/dev/null; \
    test "$(/opt/virtbench-venv/bin/virtbench --version)" = "virtbench, version ${VIRTBENCH_VERSION#v}"; \
    rm -rf /opt/virtbench/docs; \
    archive="openshift-client-linux-${TARGETARCH}-rhel9-${OPENSHIFT_CLIENT_VERSION}.tar.gz"; \
    mkdir -p /tmp/openshift-client; \
    curl -sSfL "https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/${OPENSHIFT_CLIENT_VERSION}/${archive}" -o "/tmp/${archive}"; \
    tar xz -C /tmp/openshift-client -f "/tmp/${archive}"; \
    install -m 0755 /tmp/openshift-client/oc /oc; \
    install -m 0755 /tmp/openshift-client/kubectl /kubectl; \
    rm -rf /tmp/virtbench.tar.gz /tmp/openshift-client
USER 65532

FROM ${RUNTIME_IMAGE}
ARG IMAGE_VERSION
ARG VIRTBENCH_VERSION
ARG HARNESS_VERSION
ARG KUBE_BURNER_OCP_SOURCE_MODE
ARG KUBE_BURNER_OCP_VERSION
ARG KUBE_BURNER_OCP_REF
ARG KUBE_BURNER_OCP_COMMIT
ARG KUBE_BURNER_OCP_REPOSITORY
USER 0
LABEL org.opencontainers.image.version="${IMAGE_VERSION}"
LABEL io.storage-cert-harness.virtbench.version="${VIRTBENCH_VERSION}"
LABEL io.storage-cert-harness.harness.version="${HARNESS_VERSION}"
LABEL io.storage-cert-harness.kube-burner-ocp.source-mode="${KUBE_BURNER_OCP_SOURCE_MODE}"
LABEL io.storage-cert-harness.kube-burner-ocp.version="${KUBE_BURNER_OCP_VERSION}"
LABEL io.storage-cert-harness.kube-burner-ocp.ref="${KUBE_BURNER_OCP_REF}"
LABEL io.storage-cert-harness.kube-burner-ocp.commit="${KUBE_BURNER_OCP_COMMIT}"
LABEL io.storage-cert-harness.kube-burner-ocp.repository="${KUBE_BURNER_OCP_REPOSITORY}"
# Multiple sources are allowed when the destination is a directory (trailing slash).
COPY bin/harness /usr/bin/
COPY --from=builder /kube-burner /virtctl /usr/bin/
COPY --from=kube-burner-ocp-builder /kube-burner-ocp /usr/bin/
COPY --from=virtbench-builder /oc /kubectl /opt/virtbench-venv/bin/virtbench /usr/bin/
COPY --from=virtbench-builder /opt/virtbench /opt/virtbench-runtime
COPY container-patches/virtbench/examples/utilities/ssh-pod.yaml /opt/virtbench-runtime/examples/utilities/ssh-pod.yaml
COPY --from=virtbench-builder /opt/virtbench-venv /opt/virtbench-venv
COPY container-entrypoint.sh /usr/local/bin/
# Refresh UBI RPMs at build time; keep /var/lib/rpm so Trivy can detect OS packages.
RUN python3 -m pip install --no-cache-dir --upgrade 'pip==26.2.1' 'setuptools>=78.1.1' \
  && rm -rf /opt/virtbench-runtime/docs \
  && mkdir -p /home/harness/.config /home/harness/.local/share/containers \
  && chmod 0755 /usr/local/bin/container-entrypoint.sh \
  && chown -R 65532:65532 /opt/virtbench-runtime /home/harness \
  && if command -v microdnf >/dev/null 2>&1; then \
       microdnf update -y --nodocs; \
       microdnf clean all; \
     fi \
  && if command -v dnf >/dev/null 2>&1; then \
       dnf clean all; \
     fi \
  && rm -rf /var/lib/dnf/history* /var/cache/dnf
ENV PATH="/opt/virtbench-venv/bin:${PATH}"
ENV HOME=/home/harness
# The image runs as the non-root image user by default. Callers that need a
# different bind-mount mapping can override the container UID at runtime.
USER 65532
ENTRYPOINT ["/usr/local/bin/container-entrypoint.sh"]
