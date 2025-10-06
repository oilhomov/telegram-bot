# Используем лёгкий Go-образ
FROM golang:1.22-alpine

WORKDIR /app

# Установим Python и yt-dlp
RUN apk add --no-cache python3 py3-pip ffmpeg \
    && pip install --no-cache-dir --break-system-packages yt-dlp

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o main .

CMD ["./main"]
