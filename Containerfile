# Python runtime is required by the packaged virtbench FIO CLI; the Go build is
# offline against committed vendor/ (GOPROXY=off).
# ARG defaults must match ci/config/images.env (ci/scripts/sync-images-yml.sh --check).
# The Go toolset base image already defines VERSION. Keep the harness build
# version distinct so linker injection cannot silently use the Go version.
ARG HARNESS_VERSION=0.0.0-dev
ARG IMAGE_VERSION=0.0.0-dev
ARG BUILD_IMAGE=registry.access.redhat.com/ubi10/go-toolset:1.26
ARG RUNTIME_IMAGE=registry.access.redhat.com/ubi9/python-311:9.8
ARG KUBE_BURNER_OCP_VERSION=v1.12.3
ARG OPENSHIFT_CLIENT_VERSION=4.22.11
ARG VIRTBENCH_VERSION=v2.0.0

FROM ${BUILD_IMAGE} AS builder
ARG HARNESS_VERSION
ARG KUBE_BURNER_OCP_VERSION
USER 0
WORKDIR /src
ENV GOPROXY=off
ENV GOFLAGS=-mod=vendor
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY . .
RUN go build -ldflags "-X main.version=${HARNESS_VERSION}" -o /harness ./cmd/harness

RUN set -eux; \
    ver="${KUBE_BURNER_OCP_VERSION#v}"; \
    curl -sSfL "https://github.com/kube-burner/kube-burner-ocp/releases/download/${KUBE_BURNER_OCP_VERSION}/kube-burner-ocp-V${ver}-linux-x86_64.tar.gz" \
      | tar xz -C /tmp kube-burner-ocp; \
    install -m 0755 /tmp/kube-burner-ocp /kube-burner-ocp

FROM ${RUNTIME_IMAGE} AS virtbench-builder
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
    archive="openshift-client-linux-amd64-rhel9-${OPENSHIFT_CLIENT_VERSION}.tar.gz"; \
    mkdir -p /tmp/openshift-client; \
    curl -sSfL "https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/${OPENSHIFT_CLIENT_VERSION}/${archive}" \
      | tar xz -C /tmp/openshift-client; \
    install -m 0755 /tmp/openshift-client/kubectl /kubectl

FROM ${RUNTIME_IMAGE}
ARG IMAGE_VERSION
ARG VIRTBENCH_VERSION
LABEL org.opencontainers.image.version="${IMAGE_VERSION}"
LABEL io.storage-cert-harness.virtbench.version="${VIRTBENCH_VERSION}"
COPY --from=builder /harness /usr/bin/harness
COPY --from=builder /kube-burner-ocp /usr/bin/kube-burner-ocp
COPY --from=virtbench-builder /opt/virtbench /opt/virtbench
COPY --from=virtbench-builder /opt/virtbench-venv /opt/virtbench-venv
COPY --from=virtbench-builder /opt/virtbench-venv/bin/virtbench /usr/bin/virtbench
COPY --from=virtbench-builder /kubectl /usr/bin/kubectl
ENV PATH="/opt/virtbench-venv/bin:${PATH}"
ENV VIRTBENCH_REPO=/opt/virtbench
USER 65532:65532
ENTRYPOINT ["/usr/bin/harness"]
