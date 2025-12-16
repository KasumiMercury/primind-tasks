FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /worker ./cmd/worker

FROM gcr.io/distroless/static-debian12 AS runner

FROM runner AS api

COPY --from=builder /api /api
EXPOSE 8080
ENTRYPOINT ["/api"]

FROM runner AS worker
COPY --from=builder /worker /worker
ENTRYPOINT ["/worker"]
