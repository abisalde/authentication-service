# Build stage

FROM golang:1.24-alpine AS builder
# WORKDIR /app
# COPY . .
# RUN go mod download
# RUN CGO_ENABLED=0 GOOS=linux go build -o auth-service ./cmd/server

# # Final stage
# FROM alpine:latest
# RUN apk --no-cache add ca-certificates
# WORKDIR /root/
# COPY --from=builder /app/authentication-service .
# COPY --from=builder /app/migrations ./migrations
# CMD ["./authentication-service"]