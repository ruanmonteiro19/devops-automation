#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
destination=${VETRA_BUILD_DIR:-downloads}
mkdir -p "$destination"
for target in linux/amd64 linux/arm64 windows/amd64; do
    platform=${target%/*}
    arch=${target#*/}
    suffix=''
    [ "$platform" != windows ] || suffix=.exe
    CGO_ENABLED=0 GOOS="$platform" GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o "$destination/vetra-agent-$platform-$arch$suffix" ./cmd/vetra-agent
    CGO_ENABLED=0 GOOS="$platform" GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o "$destination/vetra-relay-$platform-$arch$suffix" ./cmd/observa-gateway
    CGO_ENABLED=0 GOOS="$platform" GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o "$destination/vetra-relay-web-$platform-$arch$suffix" ./cmd/vetra-injector
done
