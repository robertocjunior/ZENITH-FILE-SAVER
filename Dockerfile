# --- Build Stage ---
FROM golang:1.25-alpine AS builder

# Install build dependencies for CGO (required by mattn/go-sqlite3)
RUN apk add --no-cache gcc musl-dev git

WORKDIR /app

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire codebase
COPY . .

# Compile the binary with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o zenith-file-saver cmd/server/main.go

# --- Runner Stage ---
FROM alpine:latest

# Install runtime dependencies (certificates and timezone data)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/zenith-file-saver .

# Expose Web UI and WebSocket port
EXPOSE 8080

# Expose persistent storage volumes
# - /app/data: Contains configs (config.json), app database (app.db) and whatsmeow session (whatsapp.db)
# - /app/FILES: Contains the structured downloaded files
VOLUME ["/app/data", "/app/FILES"]

# Run the server
CMD ["./zenith-file-saver"]
