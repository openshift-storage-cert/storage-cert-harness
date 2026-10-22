# Harness image run entry points (local cluster smoke, Quay image smoke, offline replay).
# Repo root Makefile delegates here; CI continues to invoke the root Makefile.
#
# Direct use: make -f ci/scripts/run-harness/Makefile smoke-ci
# From repo root: make smoke-ci

REPO_ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/../../..)
SCRIPTS := $(REPO_ROOT)/ci/scripts/run-harness

KUBECONFIG ?= work/kubeconfig
VERSION ?= latest

.PHONY: help replay-smoke prepare-smoke-env clean-smoke-env smoke-env smoke-ci \
	run-harness-smoke run-quay-image-tests

help:
	@echo "Harness run targets (prefer repo root: make <target>):"
	@echo "  prepare-smoke-env    Stage ci/config/smoke (+ catalog) into work/"
	@echo "  clean-smoke-env      Remove smoke artifacts, logs, staged config copies"
	@echo "  smoke-env            clean-smoke-env then prepare-smoke-env"
	@echo "  smoke-ci             smoke-env then run-harness-smoke (CI / first-time flow)"
	@echo "  run-harness-smoke    Live cluster smoke via local Podman image"
	@echo "  run-quay-image-tests Pull and run published Quay image (VERSION=$(VERSION))"
	@echo "  replay-smoke         Offline validate+run (example catalog, no cluster)"

prepare-smoke-env:
	cd "$(REPO_ROOT)" && "$(SCRIPTS)/prepare-smoke-env.sh"

clean-smoke-env:
	cd "$(REPO_ROOT)" && "$(SCRIPTS)/clean-smoke-env.sh"

smoke-env: clean-smoke-env prepare-smoke-env

smoke-ci: smoke-env run-harness-smoke

replay-smoke:
	cd "$(REPO_ROOT)" && "$(SCRIPTS)/replay-smoke.sh"

run-harness-smoke:
	cd "$(REPO_ROOT)" && "$(SCRIPTS)/run-image-tests.sh" "$(KUBECONFIG)"

run-quay-image-tests:
	cd "$(REPO_ROOT)" && "$(SCRIPTS)/run-quay-image-tests.sh" "$(VERSION)" "$(KUBECONFIG)"
