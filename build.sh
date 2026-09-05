#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION="${VERSION:-0.1.6}"
IMAGE="${GO_IMAGE:-golang:1.24-bookworm}"
ARCH="${ARCH:-arm64}"
case "${ARCH}" in
  arm64|amd64) ;;
  *)
    printf 'Unsupported ARCH=%s (expected arm64 or amd64)\n' "${ARCH}" >&2
    exit 1
    ;;
esac

OUTPUT="${ROOT_DIR}/dist/codex-health-monitor-linux-${ARCH}.so"

mkdir -p "${ROOT_DIR}/dist"

if [[ "${BUILD_WITH_DOCKER:-1}" == "1" ]]; then
  rm -f "${OUTPUT}"
  temp_dir="$(mktemp -d "${ROOT_DIR}/.build-output.XXXXXX")"
  docker buildx build \
    --platform "linux/${ARCH}" \
    --progress plain \
    --build-arg "GO_IMAGE=${IMAGE}" \
    --build-arg "VERSION=${VERSION}" \
    --build-arg "TARGET_GOARCH=${ARCH}" \
    --target artifact \
    --output "type=local,dest=${temp_dir}" \
    "${ROOT_DIR}"
  mv "${temp_dir}/codex-health-monitor.so" "${OUTPUT}"
  rmdir "${temp_dir}"
  chmod 0755 "${OUTPUT}"
else
  (
    cd "${ROOT_DIR}"
    go test ./...
    CGO_ENABLED=1 GOOS=linux GOARCH="${ARCH}" go build -buildmode=c-shared -trimpath \
      -ldflags="-s -w -X main.pluginVersion=${VERSION}" -o "${OUTPUT}" .
  )
fi

printf 'Built %s\n' "${OUTPUT}"
