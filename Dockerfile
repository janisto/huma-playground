# syntax=docker/dockerfile:1.19.0

# Rebuild this image for Go standard-library, dependency, and base-image security updates.
ARG GO_IMAGE=golang:1.26-trixie@sha256:116489021a0d8ca3facf79f84ee69052cff88733547150a644d45c5eaa91dc43
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian13:nonroot@sha256:d29e660cc75a5b6b1334e03c5c81ccf9bc0884a002c6000dbf0fb96034814478
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
