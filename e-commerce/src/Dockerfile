# syntax=docker/dockerfile:1.6

# Build stage - force linux/amd64
FROM --platform=linux/amd64 golang:1.21-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Explicitly set GOOS and GOARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .

# Runtime stage - also force linux/amd64
FROM --platform=linux/amd64 alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]