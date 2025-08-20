# Build stage
FROM golang:1.25.0-alpine3.22 AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary (simple, no version injection)
RUN go build \
    -ldflags "-s -w" \
    -o evacuator \
    ./cmd/evacuator

# Prebuilt stage - for CI optimization using pre-built binaries
FROM alpine:3.22.1 AS prebuilt

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create a non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Create app directory and set ownership
WORKDIR /app
RUN chown appuser:appgroup /app

# Copy the pre-built binary (relative path from build context)
COPY dist/evacuator evacuator
RUN chmod +x evacuator

# Change ownership of the binary
RUN chown appuser:appgroup evacuator

# Switch to non-root user
USER appuser

# Run the binary
CMD ["./evacuator"]

# Final stage - using specific alpine version for security (default target)
FROM alpine:3.22.1

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create a non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Create app directory and set ownership
WORKDIR /app
RUN chown appuser:appgroup /app

# Copy the binary
COPY --from=builder /app/evacuator .

# Change ownership of the binary
RUN chown appuser:appgroup evacuator

# Switch to non-root user
USER appuser

# Run the binary
CMD ["./evacuator"]
