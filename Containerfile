# Python runtime is required by the packaged virtbench FIO CLI. The harness
# binary must be built before this image build and is copied from bin/harness.
# Pre-built cert tools come from ci/config/images.env BUILD_IMAGE (harness-builder digest).
# Keep the harness build version distinct from the base image version.
ARG HARNESS_VERSION=0.0.0-dev
ARG IMAGE_VERSION=0.0.0-dev
ARG BUILD_IMAGE
ARG RUNTIME_IMAGE

FROM ${BUILD_IMAGE} AS tools

FROM ${RUNTIME_IMAGE}
ARG IMAGE_VERSION
ARG HARNESS_VERSION
USER 0
LABEL org.opencontainers.image.version="${IMAGE_VERSION}"
LABEL io.storage-cert-harness.harness.version="${HARNESS_VERSION}"
COPY bin/harness /usr/bin/
COPY --from=tools /usr/bin/kube-burner /usr/bin/kube-burner-ocp /usr/bin/virtctl /usr/bin/
COPY --from=tools /usr/bin/oc /usr/bin/kubectl /usr/bin/virtbench /usr/bin/
COPY --from=tools /opt/virtbench-runtime /opt/virtbench-runtime
COPY --from=tools /opt/virtbench-runtime/examples/vm-templates /opt/virtbench-runtime/examples/vm-templates
COPY container-patches/virtbench/examples/utilities/ssh-pod.yaml /opt/virtbench-runtime/examples/utilities/ssh-pod.yaml
COPY --from=tools /opt/virtbench-venv /opt/virtbench-venv
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
ENV VIRTBENCH_REPO=/opt/virtbench-runtime
# The image runs as the non-root image user by default. Callers that need a
# different bind-mount mapping can override the container UID at runtime.
USER 65532
ENTRYPOINT ["/usr/local/bin/container-entrypoint.sh"]
