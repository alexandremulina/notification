# Build stage
FROM golang:1.20-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/api

# Run stage
FROM alpine:3.14

WORKDIR /app

COPY --from=builder /app/main .
COPY migrations ./migrations

EXPOSE 8080

CMD ["./main"]