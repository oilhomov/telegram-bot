# build stage
FROM golang:1.21-bullseye AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /tgloader

# runtime stage: slim image + python (for yt-dlp)
FROM python:3.11-slim
# установим yt-dlp и ffmpeg (ffmpeg нужен для конвертации/merge)
RUN apt-get update && apt-get install -y ffmpeg git && pip install --no-cache-dir yt-dlp && apt-get clean && rm -rf /var/lib/apt/lists/*
COPY --from=builder /tgloader /tgloader
# рабочая директория
WORKDIR /data
# точка входа
ENTRYPOINT ["/tgloader"]
