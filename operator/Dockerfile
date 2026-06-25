# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26.4

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /workspace/operator

COPY operator/go.mod operator/go.sum ./
RUN go mod download

COPY operator/ ./

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /workspace/polykube-operator ./cmd/polykube-operator

FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

LABEL org.opencontainers.image.title="polykube-operator"
LABEL org.opencontainers.image.description="Kubernetes operator for Polykube resources"
LABEL org.opencontainers.image.source="https://github.com/Kismet-Engineering/polykube"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${COMMIT}"
LABEL org.opencontainers.image.created="${DATE}"

COPY --from=build /workspace/polykube-operator /manager

USER 65532:65532
ENTRYPOINT ["/manager"]
