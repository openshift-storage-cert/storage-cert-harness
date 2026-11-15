# Python runtime is required by the packaged virtbench FIO CLI. The harness
# binary must be built before this image build and is copied from bin/harness.
# Pre-built cert tools live in the digest-pinned BUILD_IMAGE (harness-builder).
# Extend that image instead of re-COPYing the tool tree onto a fresh RUNTIME_IMAGE,
# which duplicates most of the UBI rootfs and fails Dive efficiency gates.
ARG HARNESS_VERSION=0.0.0-dev
ARG IMAGE_VERSION=0.0.0-dev
ARG BUILD_IMAGE
# Same UBI Python stream as harness-builder; passed for pin alignment, not a second FROM.
ARG RUNTIME_IMAGE

FROM ${BUILD_IMAGE}
ARG IMAGE_VERSION
ARG HARNESS_VERSION
USER 0
LABEL org.opencontainers.image.version="${IMAGE_VERSION}"
LABEL io.storage-cert-harness.harness.version="${HARNESS_VERSION}"
COPY bin/harness /usr/bin/
COPY container-patches/virtbench/examples/utilities/ssh-pod.yaml /opt/virtbench-runtime/examples/utilities/ssh-pod.yaml
COPY container-entrypoint.sh /usr/local/bin/
# OS RPM refresh runs in harness-builder (microdnf update); repeating it here
# duplicates large package layers and fails Dive. Keep /var/lib/rpm for Trivy.
RUN rm -rf /opt/virtbench-runtime/build /opt/virtbench-runtime/.github \
  && mkdir -p /home/harness/.config /home/harness/.local/share/containers \
  && chmod 0755 /usr/local/bin/container-entrypoint.sh \
  && chown -R 65532:65532 /opt/virtbench-runtime /home/harness
ENV PATH="/opt/virtbench-venv/bin:${PATH}"
ENV HOME=/home/harness
ENV VIRTBENCH_REPO=/opt/virtbench-runtime
# The image runs as the non-root image user by default. Callers that need a
# different bind-mount mapping can override the container UID at runtime.
USER 65532
ENTRYPOINT ["/usr/local/bin/container-entrypoint.sh"]
