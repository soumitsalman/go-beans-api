# Use golang bullseye as base image (Debian-based)
FROM golang:1.24-bookworm

# Install necessary dependencies
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    build-essential \
    ca-certificates \
    fuse3 \
    sqlite3 \
    libsqlite3-dev \
    libstdc++-12-dev \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Set CGO flags
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-I/usr/include"
ENV CGO_CXXFLAGS="-I/usr/include"
ENV CGO_LDFLAGS="-L/usr/lib -L/usr/lib/x86_64-linux-gnu -lstdc++"

COPY . .
RUN go mod download
RUN CGO_ENABLED=1 GOOS=linux go build -o gobeansack .

COPY --from=flyio/litefs:0.5 /usr/local/bin/litefs /usr/local/bin/litefs


# Create directory for SQLite database
RUN mkdir -p /data
RUN mkdir -p .logs

ENV VECTOR_DIMENSIONS=384
ENV RELATED_EPS=0.43
ENV PORT=8080
ENV DATA=/data
ENV MAX_CONCURRENT_QUERIES=1
ENV REFRESH_TIME=3
ENV GIN_MODE=release

EXPOSE 8080

# Run the application as entrypoint
ENTRYPOINT ["/app/gobeansack"]