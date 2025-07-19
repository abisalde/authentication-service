# Build stage

FROM golang:1.24-alpine AS builder

# Set destination for COPY
WORKDIR /app

# Install git for go modules if private repos
RUN apk add --no-cache git gcc musl-dev

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download


# Copy only what's needed for code generation
COPY internal/database/ent/ ./internal/database/ent/
COPY internal/graph/ ./internal/graph/
COPY gqlgen.yml ./

# Generate Ent code
RUN go generate ./internal/database/ent

# Workaround for local package resolution
RUN go mod edit -replace github.com/abisalde/authentication-service=. && \
    go mod tidy

# Generate GraphQL code
RUN go run github.com/99designs/gqlgen generate --verbose


# Copy the entire project
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /authentication-service ./cmd/server

# Final stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates postgresql-client mysql-client && \
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
COPY --from=builder /app/configs ./configs
COPY --from=builder --chown=appuser /app/migrations ./migrations


# Create directory for certificates
RUN mkdir -p /home/appuser/cockroach/certs


CMD ["./authentication-service"]