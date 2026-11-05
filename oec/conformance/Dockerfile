# syntax=docker/dockerfile:1
# Image of the compliance tools D7 oec-check and D8 oec-infra (ADR-0025 §10
# and amendment C-107): oi4/oec-conformance. Build context: oec/ (see
# oec/.dockerignore), for linux/amd64 and linux/arm64 with docker buildx
# (oec/conformance/images/build.sh):
#
#   docker buildx build -f conformance/Dockerfile --platform linux/arm64 \
#     -t oi4/oec-conformance:0.1.0 --load oec
#
# The build stage runs on the build platform and cross-compiles (pure Go,
# CGO_ENABLED=0, -trimpath, empty build id as oec/ci/binaries.sh); the
# final stage has no RUN. The images are pinned by tag and digest
# (multi-platform index).
ARG GO_IMAGE=docker.io/library/golang:1.27.3-trixie@sha256:c65ecfe613b539ac74c62a4c3b2c27e8f8a6076384c8f27e8a3c0f3be17cc385
ARG BASE_IMAGE=gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329

# the runtime base of the target platform: its package database feeds the SBOM
FROM ${BASE_IMAGE} AS base

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build
ARG TARGETARCH
ARG VERSION=0.1.0
ARG SOURCE_DATE_EPOCH
ENV GOTOOLCHAIN=local GOFLAGS=-buildvcs=false
COPY --from=base /var/lib/dpkg/status.d /base/var/lib/dpkg/status.d
COPY --from=base /usr/share/doc /base/usr/share/doc
COPY --from=base /usr/lib/os-release /base/usr/lib/os-release
COPY schema /src/schema
COPY spec /src/spec
COPY conformance /src/conformance
# oec-sbom and its license texts (demos/internal/sbom; the demos module
# needs the Go SDK it replaces)
COPY sdk/go /src/sdk/go
COPY demos /src/demos
WORKDIR /src/conformance
RUN mkdir -p /out/bin && \
	for cmd in oec-check oec-infra; do \
		CGO_ENABLED=0 GOOS=linux GOARCH="$TARGETARCH" \
			go build -trimpath -ldflags="-s -w -buildid= -X main.version=$VERSION" -o "/out/bin/$cmd" "./cmd/$cmd" || exit 1; \
	done && \
	go version -m /out/bin/oec-check >/tmp/oec-check.txt && \
	go version -m /out/bin/oec-infra >/tmp/oec-infra.txt && \
	grep -P '^\t(dep|=>)\t' /tmp/oec-check.txt >/tmp/check-deps.txt && \
	grep -P '^\t(dep|=>)\t' /tmp/oec-infra.txt >/tmp/infra-deps.txt && \
	{ cmp -s /tmp/check-deps.txt /tmp/infra-deps.txt || { echo "oec-check and oec-infra link different modules" >&2; diff /tmp/check-deps.txt /tmp/infra-deps.txt >&2; exit 1; }; } && \
	cd ../demos && go run ./internal/sbom/cmd/oec-sbom -name oec-conformance -version "$VERSION" \
		-go /tmp/oec-check.txt -golicenses ../conformance/images/go-licenses.json \
		-base /base -debian images/debian.json -texts images/licenses -out /out/licenses

FROM ${BASE_IMAGE}
ARG VERSION=0.1.0
LABEL org.opencontainers.image.title="oec-conformance" \
	org.opencontainers.image.description="OI4 OEC v2.0 compliance tools: D7 oec-check (messaging) and D8 oec-infra (runtime and infrastructure)" \
	org.opencontainers.image.version="${VERSION}" \
	org.opencontainers.image.licenses="MIT" \
	org.opencontainers.image.vendor="OI4 Alliance" \
	org.opencontainers.image.source="https://github.com/OI4/oec" \
	org.opencontainers.image.base.name="gcr.io/distroless/static-debian13:nonroot"
COPY --from=build /out/bin/ /usr/local/bin/
COPY --from=build /out/licenses/ /opt/oi4/licenses/
USER 65532:65532
# both tools on PATH, no entrypoint: docker run --rm oi4/oec-conformance oec-infra image …
CMD ["oec-check"]
