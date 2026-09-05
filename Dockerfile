ARG GO_IMAGE=golang:1.24-bookworm
FROM ${GO_IMAGE} AS build

ARG VERSION=0.1.6
ARG TARGET_GOARCH=arm64
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go test ./...
RUN CGO_ENABLED=1 GOOS=linux GOARCH=${TARGET_GOARCH} go build \
    -buildmode=c-shared -trimpath \
    -ldflags="-s -w -X main.pluginVersion=${VERSION}" \
    -o /out/codex-health-monitor.so .

FROM scratch AS artifact
COPY --from=build /out/codex-health-monitor.so /
