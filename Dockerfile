# Stage 1: Build the Go binaries
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Install required build tools
RUN apk add --no-cache git ca-certificates tzdata

# Leverage module caching
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Build statically-linked binaries for server, migration, and seed CLI
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /bin/sellora ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /bin/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /bin/seed ./cmd/seed

# Stage 2: Minimal runtime image
FROM alpine:3.21

# Install runtime certificates and timezones
RUN apk --no-cache add ca-certificates tzdata

# Create dedicated non-root user and group
RUN addgroup -g 10001 -S sellora && \
    adduser -u 10001 -S sellora -G sellora

WORKDIR /app

# Create digital storage directory with appropriate permissions
RUN mkdir -p /app/storage/products && \
    chown -R sellora:sellora /app

# Copy compiled binaries from builder
COPY --from=builder /bin/sellora /bin/sellora
COPY --from=builder /bin/migrate /bin/migrate
COPY --from=builder /bin/seed /bin/seed

# Run as non-root user
USER sellora:sellora

# Expose web server HTTP port
EXPOSE 8080

# Default execution: run the Sellora web server
ENTRYPOINT ["/bin/sellora"]
