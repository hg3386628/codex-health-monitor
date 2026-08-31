#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION="${VERSION:-0.1.2}"
IMAGE="${GO_IMAGE:-golang:1.24-bookworm}"
OUTPUT="dist/codex-health-monitor.so"

mkdir -p "${ROOT_DIR}/dist"

if [[ "${BUILD_WITH_DOCKER:-1}" == "1" ]]; then
  rm -f "${ROOT_DIR}/${OUTPUT}"
  docker buildx build \
    --platform linux/arm64 \
    --progress plain \
    --build-arg "GO_IMAGE=${IMAGE}" \
    --build-arg "VERSION=${VERSION}" \
    --target artifact \
    --output "type=local,dest=${ROOT_DIR}/dist" \
    "${ROOT_DIR}"
  chmod 0755 "${ROOT_DIR}/${OUTPUT}"
else
  (
    cd "${ROOT_DIR}"
    go test ./...
    CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -buildmode=c-shared -trimpath \
      -ldflags="-s -w -X main.pluginVersion=${VERSION}" -o "${OUTPUT}" .
  )
fi

printf 'Built %s\n' "${ROOT_DIR}/${OUTPUT}"
