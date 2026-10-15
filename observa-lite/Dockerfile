# syntax=docker/dockerfile:1.7
# Multi-stage, multi-platform build: React UI and statically linked Go server/agent.
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund --legacy-peer-deps
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=frontend /src/dist/ ./internal/server/public/app/
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/observa ./cmd/observa
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /out/vetra-agent ./cmd/vetra-agent

FROM builder AS component-packages
COPY scripts/build-components.sh ./scripts/build-components.sh
RUN sh scripts/build-components.sh

FROM gcr.io/distroless/static-debian12:nonroot AS server
LABEL org.opencontainers.image.source="https://github.com/ruanmonteiro19/devops-automation"
COPY --from=builder --chown=65532:65532 /out/observa /usr/local/bin/observa
COPY --from=component-packages --chown=65532:65532 /build/downloads /opt/vetra/downloads
COPY --chown=65532:65532 deploy/docker/data-placeholder /var/lib/observa/.keep
USER 65532:65532
ENV OBSERVA_LISTEN=0.0.0.0:8080 OBSERVA_DB=/var/lib/observa/state.json VETRA_DOWNLOAD_DIR=/opt/vetra/downloads
VOLUME ["/var/lib/observa"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/observa"]
CMD ["server"]
HEALTHCHECK --interval=20s --timeout=5s --start-period=15s --retries=3 \
  CMD ["/usr/local/bin/observa", "health"]

FROM gcr.io/distroless/static-debian12:latest AS agent
LABEL org.opencontainers.image.source="https://github.com/ruanmonteiro19/devops-automation"
COPY --from=builder /out/vetra-agent /usr/local/bin/observa
USER 0:0
ENTRYPOINT ["/usr/local/bin/observa"]
CMD ["agent", "--server", "http://127.0.0.1:8080", "--interval", "30s"]

FROM builder AS gateway-build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /out/observa-gateway ./cmd/observa-gateway
FROM gcr.io/distroless/static-debian12:nonroot AS gateway
COPY --from=gateway-build /out/observa-gateway /usr/local/bin/observa-gateway
USER 65532:65532
EXPOSE 18443
ENTRYPOINT ["/usr/local/bin/observa-gateway"]

FROM builder AS rum-proxy-build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /out/vetra-injector ./cmd/vetra-injector
FROM gcr.io/distroless/static-debian12:nonroot AS rum-proxy
COPY --from=rum-proxy-build /out/vetra-injector /usr/local/bin/vetra-injector
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/vetra-injector"]

FROM nginx:1.28-alpine AS web
COPY --from=frontend /src/dist/ /usr/share/nginx/html/
COPY deploy/web.conf /etc/nginx/nginx.conf
USER 101:101
EXPOSE 8080
ENTRYPOINT ["nginx", "-g", "daemon off;"]

# Preserve the historical default image when docker build has no --target.
FROM server AS default
