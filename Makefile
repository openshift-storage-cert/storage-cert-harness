include ci/config/images.env

BINARY := bin/harness
PKG := ./...

.PHONY: all build test unittest vet lint lint-yaml lint-md lint-go tidy cover \
	run-example run-tr-stor-006 clean ci ci-image replay-smoke image-build \
	image-contents-check image-scan-trivy image-scan-dive image-scan image-push \
	secret-scan supply-chain install-tools mirror-ci-tools sync-images set-next-version

all: build

build:
	./ci/scripts/build.sh

# 1:1 with GitLab job unittest (stage: test).
unittest:
	./ci/scripts/unittest.sh

vet:
	go vet $(PKG)

lint-yaml:
	./ci/scripts/lint-yaml.sh

lint-md:
	./ci/scripts/lint-md.sh

lint-go:
	./ci/scripts/lint-go.sh

# GitLab stage: lint
lint: lint-yaml lint-md lint-go

# GitLab stage: test
test: unittest secret-scan supply-chain

tidy:
	go mod tidy

cover:
	go test -coverprofile=coverage.out $(PKG)
	go tool cover -func=coverage.out

run-example: build
	$(BINARY) run --catalog examples/catalog.example.json --thresholds thresholds.example.json --partner-level 3

ci:
	./ci/scripts/ci.sh

ci-image:
	./ci/scripts/ci.sh --image

install-tools:
	./ci/scripts/install-tools.sh

replay-smoke:
	./ci/scripts/replay-smoke.sh

secret-scan:
	./ci/scripts/secret-scan.sh

supply-chain:
	./ci/scripts/supply-chain.sh

image-build:
	./ci/scripts/image-build.sh

image-contents-check:
	./ci/scripts/image-contents-check.sh

image-scan-trivy:
	./ci/scripts/image-scan-trivy.sh

image-scan-dive:
	./ci/scripts/image-scan-dive.sh

image-scan: image-build image-contents-check image-scan-trivy image-scan-dive

image-push:
	PUSH=1 ./ci/scripts/image-push.sh

# One-time (or refresh) Hub → quay.io/virtarraycert/ci_tools retags. Needs
# Docker Hub pull + Quay push credentials. Not run in GitLab.
mirror-ci-tools:
	./ci/scripts/mirror-ci-tools.sh

# After editing ci/config/images.env. lint-yaml.sh --check fails if this is stale.
sync-images:
	./ci/scripts/sync-images-yml.sh

# Set next binary or image version (see docs/ci.md Versioning).
#   make set-next-version BINARY=1.0.0
#   make set-next-version IMAGE=2.0.0
set-next-version:
ifdef BINARY
	./ci/scripts/set-next-version.sh --binary $(BINARY)
else ifdef IMAGE
	./ci/scripts/set-next-version.sh --image $(IMAGE)
else
	$(error specify BINARY=x.y.z or IMAGE=x.y.z)
endif

# TR-STOR-006 as authored: live pvc-density on the cluster.
run-tr-stor-006: build
	$(BINARY) run \
		--catalog examples/catalog.tr-stor-006.json \
		--plan plans/tr-stor-006-pvc-density.yaml \
		--backends backends.example.yaml \
		--workdir ./out/tr-stor-006 \
		--output ./out/tr-stor-006 \
		-v

clean:
	rm -rf bin dist coverage.out logs/*.log logs/*.json
