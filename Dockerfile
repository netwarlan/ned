FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
    -o /ned ./cmd/ned

# Runtime — needs bash and docker CLI for shelling out to service scripts
FROM debian:13-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    docker.io \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /ned /usr/local/bin/ned

VOLUME ["/config"]
VOLUME ["/scripts"]

ENTRYPOINT ["ned"]
CMD ["--config", "/config/config.yaml"]
