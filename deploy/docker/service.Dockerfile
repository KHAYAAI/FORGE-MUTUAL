# One recipe for every Go service and tool in this repository. The build context is the repository root, because
# modules reach each other through replace directives under pkg/ and tools/. Each module is built on its own
# (GOWORK=off), the way it is tested in CI, so an image contains exactly what its go.mod says.
#
#   docker build -f deploy/docker/service.Dockerfile \
#     --build-arg MODULE=services/platform/workspace-api --build-arg CMD=workspace-api -t forge/workspace-api .
#
# CGO is off except where a module needs it (the TigerBeetle client links a native library): pass CGO_ENABLED=1.
# syntax=docker/dockerfile:1
FROM golang:1.24-bookworm AS build
ARG MODULE
ARG CMD
ARG CGO_ENABLED=0
# Extra CAs for intercepting proxies (see deploy/docker/ca/README.md). Empty means the image's own roots are used.
COPY deploy/docker/ca/ /usr/local/share/ca-certificates/forge/
RUN update-ca-certificates
WORKDIR /src
COPY . .
WORKDIR /src/${MODULE}
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=${CGO_ENABLED} GOWORK=off GOTOOLCHAIN=local go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${CMD}

# The runtime holds the binary and nothing else: no shell, no package manager, a non-root user.
FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=build /out/app /usr/local/bin/app
COPY policy /app/policy
COPY config /app/config
WORKDIR /app
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/app"]
