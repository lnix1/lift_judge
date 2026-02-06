# Build Stage
FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# 1. Install Goose (The Migration Tool)
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

# Copy source and build app
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -o lift_judge .

# Runtime Stage
FROM debian:bookworm-slim

WORKDIR /app

# 1. Install utilities to manage keys and downloads
RUN apt-get update && apt-get install -y wget gpg

# 2. Add the Raspberry Pi OS GPG key
RUN wget -qO - https://archive.raspberrypi.com/debian/raspberrypi.gpg.key | gpg --dearmor -o /usr/share/keyrings/raspberrypi-archive-keyring.gpg

# 3. Add the Raspberry Pi package repository
RUN echo "deb [signed-by=/usr/share/keyrings/raspberrypi-archive-keyring.gpg] http://archive.raspberrypi.com/debian/ bookworm main" | tee /etc/apt/sources.list.d/raspi.list

# 4. Install the camera tools (rpicam-vid)
# We also update again to see the new repo
RUN apt-get update && apt-get install -y \
    wget \
    gpg \
    rpicam-apps \
    libstdc++6 \
    libatomic1 \
    ffmpeg

# Clean up to keep image small
RUN rm -rf /var/lib/apt/lists/*

# Copy binaries (Same as before)
COPY --from=builder /app/lift_judge .
COPY --from=builder /go/bin/goose ./goose
COPY internal/sql/schema ./schema
COPY --from=builder /app/static ./static

COPY --from=builder /app/internal/annotators/media_pipe ./internal/annotators/media_pipe

# Ensure the binary is executable
RUN chmod +x ./internal/annotators/media_pipe/annotator

ENV LD_LIBRARY_PATH="/app/internal/annotators/media_pipe/libs:$LD_LIBRARY_PATH"

EXPOSE 8080

# (Same CMD as before)
CMD ["sh", "-c", "until ./goose -dir ./schema postgres \"$DB_URL\" up; do echo 'Waiting for DB...'; sleep 2; done && ./lift_judge"]
