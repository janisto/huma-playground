# syntax=docker/dockerfile:1.19.0

# Rebuild this image for Go standard-library, dependency, and base-image security updates.
ARG GO_IMAGE=golang:1.26-trixie@sha256:4ee9ffa999b4583ce281939cdff828763083610292f252279a0cee77473bd9a7
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian13:nonroot@sha256:f7f8f729987ad0fdf6b05eeeae94b26e6a0f613bdf46feea7fc40f7bd72953e6
ARG VERSION=dev

FROM ${GO_IMAGE} AS builder

ARG VERSION=dev
ENV CGO_ENABLED=0 GOOS=linux
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -mod=readonly -buildvcs=false -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /app/server ./cmd/server

FROM ${RUNTIME_IMAGE} AS runtime

ARG RUNTIME_IMAGE
ARG VERSION
LABEL org.opencontainers.image.base.name="${RUNTIME_IMAGE}" \
      org.opencontainers.image.source="https://github.com/janisto/huma-playground" \
      org.opencontainers.image.version="${VERSION}"

COPY --from=builder --chmod=0555 /app/server /server

USER 65532:65532
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/server"]
