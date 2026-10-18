# syntax=docker/dockerfile:1.28.0

# Rebuild this image for Go standard-library, dependency, and base-image security updates.
ARG GO_IMAGE=golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
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
