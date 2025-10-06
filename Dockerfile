# syntax=docker/dockerfile:1
FROM golang:1.22-alpine

# Установим зависимости (yt-dlp + ffmpeg)
RUN apk add --no-cache python3 py3-pip ffmpeg \
    && pip install --no-cache-dir yt-dlp --break-system-packages


WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o main .

CMD ["./main"]
