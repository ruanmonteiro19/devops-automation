# syntax=docker/dockerfile:1.7
# Multi-stage, multi-platform build: React UI and statically linked Go server/agent.
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /src
COPY frontend/package.json ./package.json
RUN npm install --no-audit --no-fund --legacy-peer-deps
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /build
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=frontend /src/dist/ ./internal/server/public/app/
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/observa ./cmd/observa

FROM gcr.io/distroless/static-debian12:nonroot AS server
LABEL org.opencontainers.image.source="https://github.com/ruanmonteiro19/devops-automation"
COPY --from=builder --chown=65532:65532 /out/observa /usr/local/bin/observa
COPY --chown=65532:65532 deploy/docker/data-placeholder /var/lib/observa/.keep
USER 65532:65532
ENV OBSERVA_LISTEN=0.0.0.0:8080 OBSERVA_DB=/var/lib/observa/state.json
VOLUME ["/var/lib/observa"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/observa"]
CMD ["server"]
HEALTHCHECK --interval=20s --timeout=5s --start-period=15s --retries=3 \
  CMD ["/usr/local/bin/observa", "health"]

FROM gcr.io/distroless/static-debian12:latest AS agent
LABEL org.opencontainers.image.source="https://github.com/ruanmonteiro19/devops-automation"
COPY --from=builder /out/observa /usr/local/bin/observa
USER 0:0
ENTRYPOINT ["/usr/local/bin/observa"]
CMD ["agent", "--server", "http://127.0.0.1:8080", "--interval", "30s"]
