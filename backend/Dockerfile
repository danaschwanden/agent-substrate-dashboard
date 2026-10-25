# syntax=docker/dockerfile:1.12
FROM golang:1.27.3-alpine3.24 AS build

WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY backend/ ./
ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=development
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/dashboard-backend ./cmd/dashboard-backend

FROM scratch
ARG VERSION=development
LABEL org.opencontainers.image.title="Agent Substrate Dashboard Backend" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.licenses="Apache-2.0"
COPY --from=build /out/dashboard-backend /dashboard-backend
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/dashboard-backend"]
