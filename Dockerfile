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

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o authentication-service ./cmd/server

# Final stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates postgresql-client mysql-client curl wget && \
    adduser -D -g '' appuser

# RUN adduser -D -g '' appuser
USER appuser

WORKDIR /home/appuser

EXPOSE 8080
EXPOSE 3388
EXPOSE 3306
EXPOSE 3308
EXPOSE 6379

# Copy binary and migrations
COPY --from=builder --chown=appuser /app/authentication-service .
COPY --from=builder --chown=appuser /app/internal/configs ./internal/configs
# RUN touch .env
# COPY --from=builder --chown=1000:1000 /app/.env .


# Create directory for certificates
# RUN mkdir -p /home/appuser/cockroach/certs


CMD ["./authentication-service"]