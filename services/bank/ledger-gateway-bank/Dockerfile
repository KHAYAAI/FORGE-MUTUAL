# Build context is the repository root, because the module replaces pkg/money.
# Not yet built in CI: the build environment for this work could not run Docker (see docs/QUESTIONS.md, VERIFY-IMG).
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY pkg/money pkg/money
COPY services/bank/ledger-gateway-bank services/bank/ledger-gateway-bank
WORKDIR /src/services/bank/ledger-gateway-bank
# The TigerBeetle client links a prebuilt static library, so cgo is required.
RUN CGO_ENABLED=1 GOWORK=off go build -trimpath -o /out/ledger-gateway-bank ./cmd/ledger-gateway-bank

FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=build /out/ledger-gateway-bank /usr/local/bin/ledger-gateway-bank
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/ledger-gateway-bank"]
