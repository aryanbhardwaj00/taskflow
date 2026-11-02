# ==========================================
# STAGE 1: Build the Go Binary
# ==========================================
FROM golang:1.26-alpine AS builder

# Install build dependencies (git for modules, ca-certificates for TLS)
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Leverage Docker layer caching:
# Copy module files first; if dependencies haven't changed, 'go mod download' will be cached
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source tree
COPY . .

# Compile a static, stripped binary:
# - CGO_ENABLED=0 : fully static binary, no libc dependency
# - GOOS=linux     : target Linux OS
# - -ldflags="-s -w": strip debug symbols and DWARF to reduce size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/taskflow-api ./cmd/api

# ==========================================
# STAGE 2: Minimal Production Runtime
# ==========================================
FROM alpine:3.20

# Install runtime certificates for HTTPS calls and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Create a non-root system user and group (appuser, UID 10001)
RUN addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup

WORKDIR /app

# Copy the pre-compiled binary from the builder stage
COPY --from=builder /app/taskflow-api /app/taskflow-api

# Change ownership of binary to non-root user
RUN chown -R appuser:appgroup /app

# Run as non-privileged user for security
USER appuser:appgroup

# Expose the API port
EXPOSE 8080

# Health check to ensure the service responds
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Launch the compiled binary
ENTRYPOINT ["/app/taskflow-api"]
