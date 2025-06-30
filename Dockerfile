# Build stage

FROM golang:1.24-alpine AS builder

# Set destination for COPY
WORKDIR /app

# Install git for go modules if private repos
RUN apk add --no-cache git gcc musl-dev

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download


# Copy the entire project
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o authentication-service ./cmd/server

# Final stage
FROM alpine:latest

RUN adduser -D -g '' appuser
USER appuser

WORKDIR /home/appuser

COPY --from=builder /app/authentication-service .

# COPY --from=builder /app/migrations ./migrations

CMD ["./authentication-service"]