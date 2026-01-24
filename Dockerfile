# Build Stage
FROM golang:1.25.4-bookworm AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# CGO is required for jumpboot's shared memory operations
RUN CGO_ENABLED=1 GOOS=linux go build -o jumpboot-mcp .

# Runtime Stage
# Using debian-slim instead of alpine to ensure glibc compatibility 
# for the Python environments and micromamba.
FROM debian:bookworm-slim

# Install CA certificates (required for fetching packages) and git
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/jumpboot-mcp /usr/local/bin/

# Expose the port
EXPOSE 8080

# Create a volume for persistent environment storage
# This ensures that if you restart the container, you don't lose the 
# cached micromamba base or your active environments.
VOLUME ["/root/.jumpboot-mcp"]

# Default entrypoint for HTTP mode
ENTRYPOINT ["jumpboot-mcp"]
CMD ["-transport", "http", "-addr", ":8080"]