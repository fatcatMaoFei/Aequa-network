# Build stage
FROM golang:1.25.3-bookworm AS builder
WORKDIR /src
COPY . .
ARG BUILD_TAGS=""
ARG CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=$CGO_ENABLED GOOS=linux GOARCH=amd64 go build -tags "$BUILD_TAGS" -trimpath -ldflags "-s -w" -o /out/dvt-node ./cmd/dvt-node

# Runtime stage
FROM gcr.io/distroless/base-debian12
WORKDIR /opt/aequa
COPY --from=builder /out/dvt-node /usr/local/bin/dvt-node
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/dvt-node"]
CMD ["--validator-api", "0.0.0.0:4600", "--monitoring", "0.0.0.0:4620"]
