FROM golang:1.25.5-alpine AS builder

RUN apk update

ARG GRPC_HEALTH_PROBE_VERSION=v0.4.28
RUN wget -qO /grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /grpc_health_probe

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.Version=${VERSION}" -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.Version=${VERSION}" -o /worker ./cmd/worker

FROM gcr.io/distroless/base-debian12 AS runner

FROM runner AS api

COPY --from=builder /grpc_health_probe /grpc_health_probe
COPY --from=builder /api /api
EXPOSE 8080
ENTRYPOINT ["/api"]

FROM runner AS worker
COPY --from=builder /grpc_health_probe /grpc_health_probe
COPY --from=builder /worker /worker
ENTRYPOINT ["/worker"]
