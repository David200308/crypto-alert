# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o bin/crypto-alert cmd/main.go && \
    CGO_ENABLED=0 go build -o bin/log-api cmd/api/main.go && \
    CGO_ENABLED=0 go build -o bin/notification-service cmd/notification-service/main.go

# Runtime stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /app/bin/crypto-alert /app/
COPY --from=builder /app/bin/log-api /app/
COPY --from=builder /app/bin/notification-service /app/
# Default command (override in compose for each service)
CMD ["./crypto-alert"]
